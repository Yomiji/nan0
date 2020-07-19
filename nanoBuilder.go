package nan0

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/yomiji/genrsa"
	"github.com/yomiji/goprocrypt/v2"
	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
)

type NanoBuilder struct {
	*baseBuilder
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (nsb *NanoBuilder) BuildNanoClient(opts ...baseBuilderOption) (nan0 NanoServiceWrapper, err error) {
	nsb.build(opts...)
	return buildTcpClient(nsb.baseBuilder)
}

func (nsb *NanoBuilder) BuildNanoDNS(ctx context.Context, strategy clientDNSStrategy, opts ...baseBuilderOption) ClientDNSFactory {
	nsb.build(opts...)
	return BuildDNS(ctx, nsb.baseBuilder, buildTcpClient, strategy)
}

// Build a wrapped server instance
func (nsb *NanoBuilder) BuildNanoServer(opts ...baseBuilderOption) (*NanoServer, error) {
	nsb.build(opts...)
	return buildTcpServer(nsb.baseBuilder)
}

// Wrap a raw connection which will communicate with the server
func wrapConnectionTcp(connection net.Conn, bb *baseBuilder, encKey *[32]byte, hmac *[32]byte) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()

	nan0 = &Nan0{
		ServiceName:    bb.ns.ServiceName,
		receiver:       makeReceiveChannelFromBuilder(bb),
		sender:         makeSendChannelFromBuilder(bb),
		conn:           connection,
		closed:         false,
		writerShutdown: make(chan bool, 1),
		readerShutdown: make(chan bool, 1),
		closeComplete:  make(chan bool, 2),
	}

	go nan0.startServiceReceiver(bb.messageIdentMap, encKey, hmac)
	go nan0.startServiceSender(bb.inverseIdentMap, bb.writeDeadlineActive, encKey, hmac)

	return nan0, err
}

func buildTcpClient(sec *baseBuilder) (nan0 NanoServiceWrapper, err error) {
	conn, err := net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
	defer recoverPanic(func(e error) {
		if conn != nil {
			conn.Close()
		}
		nan0 = nil
		err = e.(error)
	})()
	checkError(err)
	slog.Info("Connection to %v established!", sec.ns.ServiceName)

	if sec.secure {
		privKey, _ := genrsa.MakeKeys(2048)
		encKey := NewEncryptionKey()
		hmac := NewHMACKey()
		/** Handshake: Client side
		// 0. create temp insecure connection
		// 1. generate all api keys (enckey and hmac)
		// 2. wait for server public key over insecure conn
		// 3. encrypt api keys and place in encrypted message
		// 4. send encrypted message over insecure conn
		// 5. wrap secure connection and resume secure comms
		*/
		slog.Debug("Begin handshake with %v!", sec.ns.ServiceName)
		tempNano, err := wrapConnectionTcp(conn, sec, nil, nil)
		checkError(err)
		select {
		case serverKey := <-tempNano.GetReceiver():
			encMsg, err := goprocrypt.Encrypt(nil,
				&ApiKey{
					EncKey:  encKey[:],
					HmacKey: hmac[:],
				}, serverKey.(*rsa.PublicKey), privKey)
			checkError(err)
			tempNano.GetSender() <- encMsg
			tempNano.softClose()
		case <-time.After(TCPTimeout):
			tempNano.Close()
			checkError(fmt.Errorf("client timeout while security handshaking"))
		}
		return wrapConnectionTcp(conn, sec, encKey, hmac)
	} else {
		return wrapConnectionTcp(conn, sec, nil, nil)
	}
}

// Builds a wrapped server instance that will provide a channel of wrapped connections
func buildTcpServer(nsb *baseBuilder) (server *NanoServer, err error) {
	defer recoverPanic(func(e error) {
		slog.Fail("while building server %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()

	var mdnsServer *mdns.Server = nil
	if nsb.serviceDiscovery {
		mdnsServer, err = makeMdnsServerTcp(nsb)
		checkError(err)
	}

	server = &NanoServer{
		newConnections: make(chan NanoServiceWrapper, MaxNanoCache),
		connections:    make([]NanoServiceWrapper, MaxNanoCache),
		closed:         false,
		service:        nsb.ns,
		rxTxWaitGroup:  new(sync.WaitGroup),
		mdnsServer:     mdnsServer,
	}
	// start a listener
	server.listener, err = nsb.ns.start()

	checkError(err)
	//make rsa keys
	var rsaPriv *rsa.PrivateKey
	var rsaPub *rsa.PublicKey
	if nsb.secure {
		rsaPriv, rsaPub = genrsa.MakeKeys(2048)
	}

	// handle shutdown separate from checking for clients
	go func(listener net.Listener) {
		for ; ; {
			// every time we get a new client
			conn, err := listener.Accept()
			if err != nil {
				slog.Warn("Listener for %v no longer accepting connections.", server.service.ServiceName)
				return
			}

			var newNano NanoServiceWrapper
			if nsb.secure {
				/** Handshake: Server side
				// 0. generate all keys (rsa)
				// 1. begin insecure connection
				// 2. send public key in clear
				// 3. receive encrypted message with client public key and secrets
				// 4. decrypt message set secureConfigs
				// 5. wrap secure connection
				*/
				// create a temporary insecure nan0 connection to the client
				tempNano, err := wrapConnectionTcp(conn, nsb, nil, nil)
				if err != nil {
					slog.Warn("Connection dropped due to %v", err)
					continue
				}
				var encKey *[32]byte
				var hmac *[32]byte
				tempNano.GetSender() <- goprocrypt.RsaKeyToPbKey(*rsaPub)
				select {
				case encMsg := <-tempNano.GetReceiver():
					apiKey := new(ApiKey)
					err := goprocrypt.Decrypt(nil, encMsg.(*goprocrypt.EncryptedMessage), rsaPriv, apiKey)
					if err != nil {
						slog.Fail("Security handshake failure: %v", err)
					}
					copy(encKey[:], apiKey.EncKey)
					copy(hmac[:], apiKey.HmacKey)
				case <-time.After(TCPTimeout):
					slog.Fail("Security handshake timeout")
					continue
				}
				tempNano.softClose()
				// begin all communications with encryption
				newNano, err = wrapConnectionTcp(conn, nsb, encKey, hmac)
				if err != nil {
					slog.Warn("Connection dropped due to %v", err)
					continue
				}
			} else {
				// create a new nan0 connection to the client
				newNano, err = wrapConnectionTcp(conn, nsb, nil, nil)
				if err != nil {
					slog.Warn("Connection dropped due to %v", err)
					continue
				}
			}
			// place the new connection on the channel and in the connections cache
			server.AddConnection(newNano)
		}
	}(server.listener)
	slog.Info("Server %v started!", server.GetServiceName())
	return
}

func makeMdnsServerTcp(nsb *baseBuilder) (s *mdns.Server, err error) {
	return makeMdnsServer(nsb, false)
}
