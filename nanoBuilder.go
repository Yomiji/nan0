package nan0

import (
	"context"
	"crypto/rsa"
	"net"
	"sync"
	"time"

	"github.com/yomiji/genrsa"
	"github.com/yomiji/goprocrypt/v2"
	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
)

const defaultPurgeDuration = 1 * time.Minute
const defaultMaxConnections = 50

type nanoBuilderOption func(*NanoBuilder)

type NanoBuilder struct {
	*baseBuilder
}

func (nsb *NanoBuilder) buildOpts(opts []interface{}) {
	for _, opt := range opts {
		switch ot := opt.(type) {
		case baseBuilderOption:
			ot(nsb.baseBuilder)
		case nanoBuilderOption:
			ot(nsb)
		case func(builder *baseBuilder):
			ot(nsb.baseBuilder)
		}
	}
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (nsb *NanoBuilder) BuildNanoClient(opts ...interface{}) (nan0 NanoServiceWrapper, err error) {
	nsb.buildOpts(opts)
	return buildTcpClient(nsb.baseBuilder)
}

func (nsb *NanoBuilder) BuildNanoDNS(ctx context.Context, strategy clientDNSStrategy, opts ...interface{}) ClientDNSFactory {
	nsb.buildOpts(opts)
	return buildDNS(ctx, nsb.baseBuilder, buildTcpClient, strategy)
}

// Build a wrapped server instance
func (nsb *NanoBuilder) BuildNanoServer(opts ...interface{}) (*NanoServer, error) {
	nsb.buildOpts(opts)
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
		closed:         make(chan struct{}),
		readerShutdown: make(chan struct{}, 1),
		writerShutdown: make(chan struct{}, 1),
		lastTxRx:       time.Now(),
		closeMux:       new(sync.Mutex),
	}

	go func() {
		nan0.startServiceReceiver(bb.routes, bb.messageIdentMap, encKey, hmac)
		nan0.Close()
	}()
	go func() {
		nan0.startServiceSender(bb.inverseIdentMap, bb.writeDeadlineActive, encKey, hmac)
		nan0.Close()
	}()
	go CheckAndDoWithDuration(func() bool {
		if bb.txRxIdleDuration <= 0 {
			return nan0.LastComm().Add(defaultTxRxIdleDuration).After(time.Now())
		}
		return nan0.LastComm().Add(bb.txRxIdleDuration).After(time.Now())
	}, func() {
		slog.Debug("[%s] Auto Close Triggered (idle: %v) (lastcom: %v)",
			nan0.GetServiceName(), bb.txRxIdleDuration, nan0.LastComm())
		nan0.Close()
	}, bb.txRxIdleDuration)

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
		slog.Debug("Begin security handshake with %v!", sec.ns.ServiceName)
		tempNano, err := wrapConnectionTcp(conn, sec, nil, nil)
		checkError(err)
		select {
		case serverKey := <-tempNano.GetReceiver():
			rsaPublicKey := goprocrypt.PbKeyToRsaKey(serverKey.(*goprocrypt.PublicKey))
			encMsg, err := goprocrypt.Encrypt(nil,
				&ApiKey{
					EncKey:  encKey[:],
					HmacKey: hmac[:],
				}, rsaPublicKey, privKey)
			checkError(err)
			tempNano.GetSender() <- encMsg
			tempNano.softClose()
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
		connectionsList:    new(connList),
		allConnectionsList: new(connList),
		closed:             make(chan struct{}),
		service:            nsb.ns,
		mdnsServer:         mdnsServer,
	}

	server.connectionsList.init()
	server.allConnectionsList.init()

	if nsb.purgeConnections <= 0 {
		go func() {
			purgeLogic(defaultPurgeDuration, server, nsb)
		}()
	} else {
		go func() {
			purgeLogic(nsb.purgeConnections, server, nsb)
		}()
	}

	if nsb.maxServerConnections <= 0 {
		nsb.maxServerConnections = defaultMaxConnections
	}

	// start a listener
	server.listener, err = nsb.ns.start()

	checkError(err)
	slog.Info("Server %v started!", server.GetServiceName())

	// handle shutdown separate from checking for clients
	go func(listener net.Listener) {
		startServerListening(listener, server, nsb)
	}(server.listener)
	return
}

func startServerListening(listener net.Listener, server *NanoServer, nsb *baseBuilder) {
	//make rsa keys
	var rsaPriv *rsa.PrivateKey
	var rsaPub *rsa.PublicKey
	if nsb.secure {
		rsaPriv, rsaPub = genrsa.MakeKeys(2048)
	}
	for ; ; {
		// every time we get a new client
		conn, err := listener.Accept()
		if err != nil {
			slog.Warn("Listener for %v no longer accepting connections.", server.service.ServiceName)
			return
		}

		if server.ActiveConnectionsCount() == nsb.maxServerConnections {
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
			slog.Debug("Accepting secure connection...")
			tempNano, err := wrapConnectionTcp(conn, nsb, nil, nil)
			if err != nil {
				slog.Warn("Connection dropped due to %v", err)
				continue
			}
			var encKey = &[32]byte{}
			var hmac = &[32]byte{}
			tempNano.GetSender() <- goprocrypt.RsaKeyToPbKey(*rsaPub)
			select {
			case encMsg := <-tempNano.GetReceiver():
				tempNano.softClose()
				if _, ok := encMsg.(*goprocrypt.EncryptedMessage); !ok {
					slog.Fail("Security handshake intercepted wrong message format")
					tempNano.Close()
					continue
				}
				apiKey := new(ApiKey)
				err := goprocrypt.Decrypt(nil, encMsg.(*goprocrypt.EncryptedMessage), rsaPriv, apiKey)
				if err != nil {
					slog.Fail("Security handshake failure: %v", err)
					tempNano.Close()
					continue
				}
				copy(encKey[:], apiKey.EncKey)
				copy(hmac[:], apiKey.HmacKey)
				slog.Debug("End security handshake successfully")
			}
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
}

func purgeLogic(duration time.Duration, server *NanoServer, bb *baseBuilder) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for !server.IsShutdown() {
		<-ticker.C
		server.allConnectionsList.purgeClosedConnections()
		if server.ActiveConnectionsCount() == bb.maxServerConnections {
			slog.Warn("Max Connections Reached")
		}
	}
}

func makeMdnsServerTcp(nsb *baseBuilder) (s *mdns.Server, err error) {
	return makeMdnsServer(nsb, false)
}
