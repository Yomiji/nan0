package nan0

import (
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/websocket"
	"net"
	"net/http"
	"reflect"
	"sync"
)

type NanoBuilder struct {
	ns                  *Service
	writeDeadlineActive bool
	messageIdentMap     map[int]proto.Message
	inverseIdentMap     map[string]int
	sendBuffer          int
	receiveBuffer       int
	origin              string
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

func (sec *NanoBuilder) SetOrigin(origin string) *NanoBuilder {
	sec.origin = origin
	return sec
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (sec NanoBuilder) Build() (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = &Nan0{
			ServiceName:    sec.ns.ServiceName,
			receiver:       nil,
			sender:         nil,
			conn:           nil,
			closed:         true,
			writerShutdown: nil,
			readerShutdown: nil,
		}
		err = e.(error)
	})()
	conn, err := net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
	checkError(err)
	info("Connection to %v established!", sec.ns.ServiceName)

	return sec.WrapConnection(conn)
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (sec NanoBuilder) Send(obj interface{}) (ws net.Conn, err error) {
	defer recoverPanic(func(e error) {
		err = e.(error)
	})()
	//paths := strings.Split(sec.ns.Uri, "/")
	if sec.websocketFlag {
		ws, err = dialWebsocket(sec)
	} else {
		ws, err = net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
	}

	checkError(err)
	info("Connection to %v established!", sec.ns.ServiceName)

	err = putMessageInConnection(ws, obj.(proto.Message), sec.inverseIdentMap)
	return ws, err
}

func (sec NanoBuilder) SendAndAwait(obj interface{}) (await chan interface{}, err error) {
	defer recoverPanic(func(e error) {
		err = e.(error)
	})()
	ws,err := sec.Send(obj)
	checkError(err)
	msg, err := getMessageFromConnection(ws, sec.messageIdentMap)
	checkError(err)
	await = makeReceiveChannelFromBuilder(sec)
	await <- msg
	return
}

func (sec NanoBuilder) WsAwait() (await chan interface{}, err error) {
	defer recoverPanic(func(e error) {
		err = e.(error)
	})()
	var ws net.Conn
	if sec.websocketFlag {
		ws, err = dialWebsocket(sec)
	} else {
		ws, err = net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))
	}
	checkError(err)
	msg, err := getMessageFromConnection(ws, sec.messageIdentMap)
	checkError(err)

	await = makeReceiveChannelFromBuilder(sec)
	await <- msg
	return
}

func (sec *NanoBuilder) buildWebsocketServer() (server *Nan0Server, err error) {
	defer recoverPanic(func(e error) {
		fail("Error occurred while serving %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()
	server = &Nan0Server{
		newConnections:   make(chan NanoServiceWrapper, MaxNanoCache),
		connections:      make([]NanoServiceWrapper, MaxNanoCache),
		listenerShutdown: make(chan bool, 1),
		confirmShutdown:  make(chan bool, 1),
		closed:           false,
		service:          sec.ns,
	}

	wsHandler := func(ws *websocket.Conn) {
		ws.PayloadType = websocket.BinaryFrame
		newNano, err := sec.WrapConnection(ws)
		if err != nil {
			warn("Connection dropped due to %v", err)
		} else {
			info("Adding: %s", ws.RemoteAddr())
		}
		server.AddConnection(newNano)
	}

	srv := &http.Server{Addr: composeTcpAddress("", sec.ns.Port)}
	http.Handle(sec.ns.GetUri(), websocket.Handler(wsHandler))
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			fail("Websocket server: %s", err)
		} else {
			info("Websocket server %s closed.", server.GetServiceName())
		}
	}()
	//listener,err := sec.ns.Start()
	//checkError(err)
	//go func() {
	//	info("Server %v started!", server.GetServiceName())
	//	err = http.Serve(listener, websocket.Handler(wsHandler))
	//	checkError(err)
	//}()

	// handle shutdown separate from checking for clients
	go func() {
		// check to see if we need to shut everything down
		<-server.listenerShutdown // close all current connections
		srv.Close()
		server.confirmShutdown <- true
		return
	}()

	return
}

// Build a wrapped server instance
func (sec *NanoBuilder) BuildServer(handler func(net.Listener, *NanoBuilder, <-chan bool)) (*Nan0Server, error) {
	if sec.websocketFlag {
		return sec.buildWebsocketServer()
	}
	return buildServer(sec, handler)
}

// Wrap a raw connection which will communicate with the server
func (sec NanoBuilder) WrapConnection(conn net.Conn) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = &Nan0{
			ServiceName:    sec.ns.ServiceName,
			receiver:       nil,
			sender:         nil,
			conn:           nil,
			closed:         true,
			writerShutdown: nil,
			readerShutdown: nil,
		}
		err = e.(error)
	})()
	checkError(err)

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

	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	go nan0.startServiceReceiver(sec.messageIdentMap, waitGroup)
	waitGroup.Add(1)
	go nan0.startServiceSender(sec.inverseIdentMap, sec.writeDeadlineActive, waitGroup)

	waitGroup.Wait()
	debug("[****] Finished waiting for wrapconn")
	return nan0, err
}

