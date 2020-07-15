package nan0

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
	"github.com/yomiji/websocket"
)

type TLSConfig struct {
	CertFile string
	KeyFile  string
}

type NanoBuilder struct {
	ns                  *Service
	writeDeadlineActive bool
	messageIdentMap     map[int]proto.Message
	inverseIdentMap     map[string]int
	sendBuffer          int
	receiveBuffer       int
	origin              string
	origins             []*url.URL
	websocketFlag       bool
	tlsConfig           *TLSConfig
	serviceDiscovery    bool
}

func (ns *Service) NewNanoBuilder() *NanoBuilder {
	var builder = new(NanoBuilder)
	builder.messageIdentMap = make(map[int]proto.Message)
	builder.inverseIdentMap = make(map[string]int)
	builder.ns = ns
	return builder
}

// Flag indicating if service discovery is enabled (client/server)
func (sec *NanoBuilder) ServiceDiscovery(port int32) *NanoBuilder {
	sec.serviceDiscovery = true
	sec.ns.MdnsPort = port
	return sec
}

// Enables websocket handling for this connection
func (sec *NanoBuilder) Websocket() *NanoBuilder {
	sec.websocketFlag = true
	return sec
}

func (sec *NanoBuilder) Secure(config TLSConfig) *NanoBuilder {
	sec.tlsConfig = &config
	return sec
}

func (sec *NanoBuilder) Insecure() *NanoBuilder {
	sec.tlsConfig = nil
	return sec
}

// Adds origin checks to websocket handler (no use for clients)
func (sec *NanoBuilder) AddOrigins(origin ...string) *NanoBuilder {
	for _, v := range origin {
		url, err := url.Parse(v)
		if err == nil {
			sec.origins = append(sec.origins, url)
		}
	}
	return sec
}

// Adds origin checks to websocket handler (no use for clients)
func (sec *NanoBuilder) AppendOrigins(origin ...*url.URL) {
	sec.origins = append(sec.origins, origin...)
}

// Part of the NanoBuilder chain, sets write deadline to the TCPTimeout global value
func (sec *NanoBuilder) ToggleWriteDeadline(writeDeadline bool) *NanoBuilder {
	sec.writeDeadlineActive = writeDeadline
	return sec
}

// Adds multiple identity-type objects that will be cloned to either send or receive messages.
// All protocol buffers you intend to send or receive should be registered with this method
// or the transmissions will fail
func (sec *NanoBuilder) AddMessageIdentities(messageIdents ...proto.Message) *NanoBuilder {
	for _, ident := range messageIdents {
		sec.AddMessageIdentity(ident)
	}
	return sec
}

// Adds a single identity-type object that will be cloned to either send or receive messages.
// All protocol buffers you intend to send or receive should be registered with this method
// or the transmissions will fail
func (sec *NanoBuilder) AddMessageIdentity(messageIdent proto.Message) *NanoBuilder {
	t := proto.MessageName(messageIdent)
	i := int(hashString(t))
	slog.Debug("Identity: %s, Hash: %d", t, i)
	slog.Debug("Ident bytes: %v", SizeWriter(i))
	sec.messageIdentMap[i] = messageIdent
	sec.inverseIdentMap[t] = i
	return sec
}

// Part of the NanoBuilder chain, sets the number of messages that can be simultaneously placed on the send buffer
func (sec *NanoBuilder) SendBuffer(sendBuffer int) *NanoBuilder {
	sec.sendBuffer = sendBuffer
	return sec
}

// Part of the NanoBuilder chain, sets the number of messages that can be simultaneously placed on the
// receive buffer
func (sec *NanoBuilder) ReceiveBuffer(receiveBuffer int) *NanoBuilder {
	sec.receiveBuffer = receiveBuffer
	return sec
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (sec *NanoBuilder) Build() (nan0 NanoServiceWrapper, err error) {
	return buildClient(sec)
}

// Build a wrapped server instance
func (sec *NanoBuilder) BuildServer(handler func(net.Listener, *NanoBuilder)) (*NanoServer, error) {
	if sec.websocketFlag {
		return buildWebsocketServer(sec)
	}
	return buildServer(sec, handler)
}

func (sec *NanoBuilder) BuildNan0DNS(ctx context.Context) func(timeoutAfter time.Duration, strictProtocols bool) (nan0 NanoServiceWrapper, err error) {
	definitionChannel := startClientServiceDiscovery(ctx, sec.ns)
	return func(timeoutAfter time.Duration, strictProtocols bool) (NanoServiceWrapper, error) {
		if timeoutAfter <= 0 {
			if mdef, ok := <-definitionChannel; ok {
				if err := processMdef(sec, mdef, strictProtocols); err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("DNS is closed")
			}
		} else {
			select {
			case mdef,ok := <-definitionChannel:
				if ok {
					populateServiceFromMDef(sec.ns, mdef)
					if err := processMdef(sec, mdef, strictProtocols); err != nil {
						return nil, err
					}
				} else {
					return nil, fmt.Errorf("DNS is closed")
				}
			case <-time.After(timeoutAfter):
				return nil, fmt.Errorf("client service discovery timeout after %v", timeoutAfter.Truncate(time.Millisecond))
			}
		}
		return buildClient(sec)
	}
}

func processMdef(sec *NanoBuilder, mdef *MDefinition, strict bool) error  {
	if strict && !allProtocolsMatch(sec, mdef) {
		return fmt.Errorf("builder fails to satisfy all protocols for service %s", sec.ns.ServiceName)
	}
	populateServiceFromMDef(sec.ns, mdef)
	return nil
}
func allProtocolsMatch(sec *NanoBuilder, definition *MDefinition) bool {
	builderNames := getIdentityMessageNames(sec)
	mdefNames := definition.SupportedMessageTypes
	if len(builderNames) != len(mdefNames) {
		return false
	}
	var checkerMap = make(map[string]bool)
	for _,name := range builderNames {
		checkerMap[name] = true
	}
	for _,name := range mdefNames {
		if _,ok := checkerMap[name]; !ok {
			return false
		}
	}
	return true
}

// Wrap a raw connection which will communicate with the server
func (sec NanoBuilder) WrapConnection(connection interface{}) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()
	checkError(err)

	switch conn := connection.(type) {
	case net.Conn:
		nan0 = &Nan0{
			ServiceName:    sec.ns.ServiceName,
			receiver:       makeReceiveChannelFromBuilder(sec),
			sender:         makeSendChannelFromBuilder(sec),
			conn:           conn,
			closed:         false,
			writerShutdown: make(chan bool, 1),
			readerShutdown: make(chan bool, 1),
			closeComplete:  make(chan bool, 2),
		}
	case *websocket.Conn:
		nan0 = &WsNan0{
			ServiceName:    sec.ns.ServiceName,
			receiver:       makeReceiveChannelFromBuilder(sec),
			sender:         makeSendChannelFromBuilder(sec),
			conn:           conn,
			closed:         false,
			writerShutdown: make(chan bool, 1),
			readerShutdown: make(chan bool, 1),
			closeComplete:  make(chan bool, 2),
			rxTxWaitGroup:  new(sync.WaitGroup),
		}
	}

	go nan0.startServiceReceiver(sec.messageIdentMap)
	go nan0.startServiceSender(sec.inverseIdentMap, sec.writeDeadlineActive)

	return nan0, err
}

func buildClient(sec *NanoBuilder) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()

	var conn interface{}

	// if the connection is a websocket, handle it this way
	if sec.websocketFlag {
		// setup a url to dial the websocket, hostname shouldn't include protocol
		u := url.URL{Scheme: "ws", Host: composeTcpAddress(sec.ns.HostName, sec.ns.Port), Path: sec.ns.Uri}
		// call the websocket server
		conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		checkError(err)
	} else {
		// otherwise, handle the connection like this using tcp
		conn, err = net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
		checkError(err)
		slog.Info("Connection to %v established!", sec.ns.ServiceName)
	}
	return sec.WrapConnection(conn)
}

// Builds a wrapped server instance that will provide a channel of wrapped connections
func buildServer(nsb *NanoBuilder, customHandler func(net.Listener, *NanoBuilder)) (server *NanoServer, err error) {
	defer recoverPanic(func(e error) {
		slog.Fail("while building server %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()

	var mdnsServer *mdns.Server = nil
	if nsb.serviceDiscovery {
		mdnsServer,err = makeMdnsServer(nsb)
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
	listener, err := nsb.ns.Start()
	// start secure listener instead
	if nsb.tlsConfig != nil {
		cer, err := tls.X509KeyPair([]byte(nsb.tlsConfig.CertFile), []byte(nsb.tlsConfig.KeyFile))
		if err != nil {
			slog.Fail("configuration for tls failed due to %v", err)
		}
		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		l := tls.NewListener(listener, config)
		server.listener = l
	} else {
		// start insecure listener
		server.listener = listener
	}
	checkError(err)
	if customHandler != nil {
		go customHandler(listener, nsb)
		return
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

			// create a new nan0 connection to the client
			newNano, err := nsb.WrapConnection(conn)
			if err != nil {
				slog.Warn("Connection dropped due to %v", err)
			}

			// place the new connection on the channel and in the connections cache
			server.AddConnection(newNano)
		}
	}(listener)
	slog.Info("Server %v started!", server.GetServiceName())
	return
}

func buildWebsocketServer(sec *NanoBuilder) (server *NanoServer, err error) {
	defer recoverPanic(func(e error) {
		slog.Fail("Error occurred while serving %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()
	var mdnsServer *mdns.Server = nil
	if sec.serviceDiscovery {
		mdnsServer,err = makeMdnsServer(sec)
		checkError(err)
	}
	server = &NanoServer{
		newConnections: make(chan NanoServiceWrapper),
		connections:    make([]NanoServiceWrapper, MaxNanoCache),
		closed:         false,
		service:        sec.ns,
		rxTxWaitGroup:  new(sync.WaitGroup),
		mdnsServer: mdnsServer,
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// create a CheckOrigin function suitable for the upgrader
	upgrader.CheckOrigin = func(r *http.Request) bool {
		for _, v := range sec.origins {
			if v.Host == r.Header.Get("Host") {
				return true
			}
		}
		slog.Debug("Failed origin check for host %s", r.Header.Get("Host"))
		return false
	}

	// construct and sanitize origins
	if sec.origin != "" {
		// dissect and interpret a manually set origin (for backward compat)
		rawOrigin, err := url.Parse(sec.origin)
		if err != nil {
			sec.origins = append(sec.origins, rawOrigin)
		}
	}

	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		//check origin
		if !upgrader.CheckOrigin(r) {
			slog.Fail("Connection failed due to origin not accepted: %s", r.Host)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("Connection dropped due to %v", err)
			return
		} else {
			slog.Info("%s has connected to the server.", conn.RemoteAddr())
		}
		newNano, err := sec.WrapConnection(conn)
		server.AddConnection(newNano)
	}

	srv := &http.Server{Addr: composeTcpAddress("", sec.ns.Port)}
	server.wsServer = srv
	http.Handle(sec.ns.GetUri(), handler)
	go func(serviceName string) {
		if sec.tlsConfig != nil {
			if err := srv.ListenAndServeTLS(sec.tlsConfig.CertFile, sec.tlsConfig.KeyFile); err != http.ErrServerClosed {
				slog.Fail("Websocket server: %s", err)
			} else {
				slog.Info("Websocket server %s closed.", serviceName)
			}
		} else {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				slog.Fail("Websocket server: %s", err)
			} else {
				slog.Info("Websocket server %s closed.", serviceName)
			}
		}

	}(server.GetServiceName())

	return
}