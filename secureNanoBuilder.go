package nan0

import (
	"net"
	"github.com/golang/protobuf/proto"
	"reflect"
)

type SecureNanoBuilder struct {
	ns                      *Service
	writeDeadlineActive     bool
	messageIdentMap			map[int]proto.Message
	inverseIdentMap			map[string]int
	sendBuffer              int
	receiveBuffer           int
	encdecKey               *[32]byte
	hmacKey                 *[32]byte
}

func (ns *Service) NewNanoBuilder() *SecureNanoBuilder {
	var builder = new(SecureNanoBuilder)
	builder.messageIdentMap = make(map[int]proto.Message)
	builder.inverseIdentMap = make(map[string]int)
	builder.ns = ns
	return builder
}

// Part of the SecureNanoBuilder chain, sets write deadline to the TCPTimeout global value
func (sec *SecureNanoBuilder) ToggleWriteDeadline(writeDeadline bool) *SecureNanoBuilder {
	sec.writeDeadlineActive = writeDeadline
	return sec
}

// Adds multiple identity-type objects that will be cloned to either send or receive messages.
// All protocol buffers you intend to send or receive should be registered with this method
// or the transmissions will fail
func (sec *SecureNanoBuilder) AddMessageIdentities(messageIdents ...proto.Message) *SecureNanoBuilder {
	for _,ident := range messageIdents {
		sec.AddMessageIdentity(ident)
	}
	return sec
}

// Adds a single identity-type object that will be cloned to either send or receive messages.
// All protocol buffers you intend to send or receive should be registered with this method
// or the transmissions will fail
func (sec *SecureNanoBuilder) AddMessageIdentity(messageIdent proto.Message) *SecureNanoBuilder {
	i := len(sec.messageIdentMap)
	sec.messageIdentMap[i] = messageIdent
	t := reflect.TypeOf(messageIdent).String()
	sec.inverseIdentMap[t] = i
	return sec
}

// Part of the SecureNanoBuilder chain, sets the number of messages that can be simultaneously placed on the send buffer
func (sec *SecureNanoBuilder) SendBuffer(sendBuffer int) *SecureNanoBuilder {
	sec.sendBuffer = sendBuffer
	return sec
}

// Part of the SecureNanoBuilder chain, sets the number of messages that can be simultaneously placed on the
// receive buffer
func (sec *SecureNanoBuilder) ReceiveBuffer(receiveBuffer int) *SecureNanoBuilder {
	sec.receiveBuffer = receiveBuffer
	return sec
}

// Part of the SecureNanoBuilder chain, used internally
func (sec *SecureNanoBuilder) EnableEncryption(secretKey *[32]byte, authKey *[32]byte) *SecureNanoBuilder {
	sec.encdecKey = secretKey
	sec.hmacKey = authKey
	return sec
}

// Establish a connection creating a first-class Nan0 connection which will communicate with the server
func (sec SecureNanoBuilder) Build() (nan0 NanoServiceWrapper, err error) {
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
func (sec *SecureNanoBuilder) BuildServer(handler func(net.Listener, *SecureNanoBuilder, <-chan bool)) (*Nan0Server, error) {
	return buildServer(sec, handler)
}

// Wrap a raw connection which will communicate with the server
func (sec SecureNanoBuilder) WrapConnection(conn net.Conn) (nan0 NanoServiceWrapper, err error) {
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

	go nan0.startServiceReceiver(sec.messageIdentMap, sec.encdecKey, sec.hmacKey)
	go nan0.startServiceSender(sec.inverseIdentMap, sec.writeDeadlineActive,  sec.encdecKey, sec.hmacKey)
	return nan0, err
}
