package nan0

import (
	"net"
	"github.com/golang/protobuf/proto"
	"reflect"
)

type NanoBuilder struct {
	ns                      *Service
	writeDeadlineActive     bool
	messageIdentMap			map[int]proto.Message
	inverseIdentMap			map[string]int
	sendBuffer              int
	receiveBuffer           int
}

func (ns *Service) NewNanoBuilder() *NanoBuilder {
	var builder = new(NanoBuilder)
	builder.messageIdentMap = make(map[int]proto.Message)
	builder.inverseIdentMap = make(map[string]int)
	builder.ns = ns
	return builder
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
	for _,ident := range messageIdents {
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
		nan0 = &Nan0{
			ServiceName:    sec.ns.ServiceName,
			receiver:       nil,
			sender:         nil,
			conn:           nil,
			closed:         true,
			writerShutdown: nil,
			readerShutdown: nil,
			closeComplete:  nil,
		}
		err = e.(error)
	})()
	conn, err := net.Dial("tcp", composeTcpAddress(sec.ns.HostName, sec.ns.Port))

	info("Connection to %v established!", sec.ns.ServiceName)

	return sec.WrapConnection(conn)
}

// Build a wrapped server instance
func (sec *NanoBuilder) BuildServer(handler func(net.Listener, *NanoBuilder, <-chan bool)) (*Nan0Server, error) {
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
			closeComplete:  nil,
		}
		err = e.(error)
	})()
	checkError(err)
	nan0 = &Nan0{
		ServiceName:    sec.ns.ServiceName,
		receiver:       make(chan interface{}, sec.receiveBuffer),
		sender:         make(chan interface{}, sec.sendBuffer),
		conn:           conn,
		closed:         false,
		writerShutdown: make(chan bool,1),
		readerShutdown: make(chan bool,1),
		closeComplete:  make(chan bool, 2),
	}

	go nan0.startServiceReceiver(sec.messageIdentMap)
	go nan0.startServiceSender(sec.inverseIdentMap, sec.writeDeadlineActive)
	return nan0, err
}
