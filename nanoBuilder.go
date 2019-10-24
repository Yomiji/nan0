package nan0

import (
	"github.com/golang/protobuf/proto"
	"github.com/Yomiji/websocket"
	"net"
	"net/http"
	"net/url"
	"reflect"
)

type NanoBuilder struct {
	ns                  *Service
	writeDeadlineActive bool
	messageIdentMap     map[int]proto.Message
	inverseIdentMap     map[string]int
	sendBuffer          int
	receiveBuffer       int
	origin				string
	origins				[]*url.URL
	websocketFlag       bool
}

func (ns *Service) NewNanoBuilder() *NanoBuilder {
	var builder = new(NanoBuilder)
	builder.messageIdentMap = make(map[int]proto.Message)
	builder.inverseIdentMap = make(map[string]int)
	builder.ns = ns
	return builder
}

// Enables websocket handling for this connection
func (sec *NanoBuilder) Websocket() *NanoBuilder {
	sec.websocketFlag = true
	return sec
}

// Adds origin checks to websocket handler (no use for clients)
func(sec *NanoBuilder) AddOrigins(origin ...string) *NanoBuilder {
	for _,v := range origin {
		url, err := url.Parse(v)
		if err == nil {
			sec.origins = append(sec.origins, url)
		}
	}
	return sec
}

// Adds origin checks to websocket handler (no use for clients)
func(sec *NanoBuilder) AppendOrigins(origin ...*url.URL) {
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
	t := reflect.TypeOf(messageIdent).String()
	i := int(hashString(t))
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
func (sec NanoBuilder) Build() (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()

	var conn interface{}
	// if the connection is a websocket, handle it this way
	if sec.websocketFlag {
		// setup a url to dial the websocket, hostname shouldn't include protocol
		u := url.URL{Scheme:"ws",Host:composeTcpAddress(sec.ns.HostName, sec.ns.Port), Path:sec.ns.Uri}
		// call the websocket server
		conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		checkError(err)
	} else {
		// otherwise, handle the connection like this using tcp
		conn, err = net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
		checkError(err)
		info("Connection to %v established!", sec.ns.ServiceName)
	}
	return sec.WrapConnection(conn)
}

type handler struct {
	handleFunc func(w http.ResponseWriter, r *http.Request)
}
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleFunc(w, r)
}

func (sec *NanoBuilder) buildWebsocketServer() (server *NanoServer, err error) {
	defer recoverPanic(func(e error) {
		fail("Error occurred while serving %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()
	server = &NanoServer{
		newConnections:   make(chan NanoServiceWrapper),
		connections:      make([]NanoServiceWrapper, MaxNanoCache),
		closed:           false,
		service:          sec.ns,
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// create a CheckOrigin function suitable for the upgrader
	upgrader.CheckOrigin = func(r *http.Request) bool {
		for _,v := range sec.origins {
			if v.Host == r.Header.Get("Host") {
				return true
			}
		}
		debug("Failed origin check for host %s", r.Header.Get("Host"))
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

	handler := &handler{
		handleFunc: func(w http.ResponseWriter, r *http.Request) {
			// if any origins, check them, else add a localhost only check
			if len(sec.origins) > 0 {

			} else {

			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				warn("Connection dropped due to %v", err)
				return
			} else {
				info("%s has connected to the server.", conn.RemoteAddr())
			}
			newNano, err := sec.WrapConnection(conn)
			server.AddConnection(newNano)
		},
	}

	srv := &http.Server{Addr: composeTcpAddress("", sec.ns.Port)}
	server.wsServer = srv
	http.Handle(sec.ns.GetUri(), handler)
	go func(serviceName string) {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			fail("Websocket server: %s", err)
		} else {
			info("Websocket server %s closed.", serviceName)
		}
	}(server.GetServiceName())

	return
}

// Build a wrapped server instance
func (sec *NanoBuilder) BuildServer(handler func(net.Listener, *NanoBuilder)) (*NanoServer, error) {
	if sec.websocketFlag {
		return sec.buildWebsocketServer()
	}
	return buildServer(sec, handler)
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
		}
	}

	go nan0.startServiceReceiver(sec.messageIdentMap)
	go nan0.startServiceSender(sec.inverseIdentMap, sec.writeDeadlineActive)

	return nan0, err
}

