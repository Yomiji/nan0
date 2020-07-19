package nan0

import (
	"context"
	"net"
	"sync"

	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
)

type NanoBuilder struct {
	secure bool
	*baseBuilder
}

// Flag indicating this builder is secure
// this will set up a secure handshake process on connection (tcp)
func (nsb *NanoBuilder) Secure(_ *baseBuilder)  {
	nsb.secure = true
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (nsb *NanoBuilder) BuildNanoClient(opts...baseBuilderOption) (nan0 NanoServiceWrapper, err error) {
	nsb.build(opts...)
	return buildTcpClient(nsb.baseBuilder)
}

func (nsb *NanoBuilder) BuildNanoDNS(ctx context.Context, strategy clientDNSStrategy, opts...baseBuilderOption) ClientDNSFactory {
	nsb.build(opts...)
	return BuildDNS(ctx, nsb.baseBuilder, buildTcpClient, strategy)
}

// Build a wrapped server instance
func (nsb *NanoBuilder) BuildNanoServer(opts...baseBuilderOption) (*NanoServer, error) {
	nsb.build(opts...)
	return buildTcpServer(nsb.baseBuilder)
}

// Wrap a raw connection which will communicate with the server
func wrapConnectionTcp(connection net.Conn, nsb *baseBuilder) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()

	nan0 = &Nan0{
		ServiceName:    nsb.ns.ServiceName,
		receiver:       makeReceiveChannelFromBuilder(nsb),
		sender:         makeSendChannelFromBuilder(nsb),
		conn:           connection,
		closed:         false,
		writerShutdown: make(chan bool, 1),
		readerShutdown: make(chan bool, 1),
		closeComplete:  make(chan bool, 2),
	}

	go nan0.startServiceReceiver(nsb.messageIdentMap)
	go nan0.startServiceSender(nsb.inverseIdentMap, nsb.writeDeadlineActive)

	return nan0, err
}

func buildTcpClient(sec *baseBuilder) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()

	// otherwise, handle the connection like this using tcp
	conn, err := net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
	checkError(err)
	slog.Info("Connection to %v established!", sec.ns.ServiceName)
	return wrapConnectionTcp(conn, sec)
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

	// handle shutdown separate from checking for clients
	go func(listener net.Listener) {
		for ; ; {
			// every time we get a new client
			conn, err := listener.Accept()
			if err != nil {
				slog.Warn("Listener for %v no longer accepting connections.", server.service.ServiceName)
				return
			}

			// create a new nan0 connection to the client
			newNano, err := wrapConnectionTcp(conn, nsb)
			if err != nil {
				slog.Warn("Connection dropped due to %v", err)
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