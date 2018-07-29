package nan0

/**
Service Discovery API
Some features that are implemented here:
DiscoveryService implements Stringer, io.Reader, io.Writer
This API accepts and manages nanoservices
 */
import (
	"time"
	"github.com/golang/protobuf/proto"
	"net"
)

// The Nan0 structure is a wrapper around a net/TCP connection which sends
// and receives protocol buffers across it. The protocol buffers are not
// descriptive and one must send or receive using the methods provided.
// If one needs more control over the conduit, one should create a
// connection to the server using the Server.DialTCP method.
type Nan0 struct {
	// The name of the service
	ServiceName string
	// Receive messages from this channel
	receiver chan interface{}
	// Messages placed on this channel will be sent
	sender chan interface{}
	// A connection maintained by this object
	conn net.Conn
	// The closed status
	closed bool
	// Channel governing the reader service
	readerShutdown chan bool
	// Channel governing the writer service
	writerShutdown chan bool
}

/*******************
 	Service API
 *******************/


// Starts the listener for this service
func (ns *Service) Start() (net.Listener, error) {
	return net.Listen("tcp", composeTcpAddress(ns.HostName, ns.Port))
}

// Checks if a particular nanoservice is expired based on its start time and time to live
func (ns Service) IsExpired() bool {
	return ns.Expired
}

// Checks if this nanoservice is responding to tcp on its port
func (ns Service) IsAlive() bool {
	address := composeTcpAddress(ns.HostName, ns.Port)
	conn, err := net.Dial("tcp", address)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	if err != nil {
		ns.Expired = true
		return false
	}
	return true
}

// Refreshes the start time so that this service does not expire
func (ns *Service) Refresh() {
	ns.StartTime = time.Now().Unix()
}

// Registers this nanoservice with the service discovery host at the given address
func (ns *Service) Register(host string, port int32) (err error) {
	address := composeTcpAddress(host, port)
	info("Registering '%v' service with discovery at '%v'", ns.ServiceName, address)
	defer recoverPanic(func(e error) { err = e.(error) })()
	conn, err := net.Dial("tcp", address)
	checkError(err)
	serviceList := &ServiceList{
		ServiceType:       ns.ServiceType,
		ServicesAvailable: []*Service{ns},
	}
	serviceListBytes, err := proto.Marshal(serviceList)
	checkError(err)
	_, err = conn.Write(serviceListBytes)
	checkError(err)
	return err
}

// Compare two service instances for equality
func (ns Service) Equals(other Service) bool {
	return ns.Port == other.Port &&
		ns.HostName == other.HostName &&
		ns.Expired == other.Expired &&
		ns.StartTime == other.StartTime &&
		ns.ServiceName == other.ServiceName &&
		ns.ServiceType == other.ServiceType
}

// Create a connection to this nanoservice using a traditional TCP connection
func (ns *Service) DialTCP() (nan0 net.Conn, err error) {
	nan0, err = net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))

	return nan0, err
}

type SecureNanoBuilder struct {
	ns                      *Service
	writeDeadlineActive     bool
	receiverMessageIdentity proto.Message
	sendBuffer              int
	receiveBuffer           int
	encdecKey               *[32]byte
	hmacKey                 *[32]byte
}

func (sec *SecureNanoBuilder) setService(ns *Service) *SecureNanoBuilder {
	sec.ns = ns
	return sec
}

func (sec *SecureNanoBuilder) ToggleWriteDeadline(writeDeadline bool) *SecureNanoBuilder {
	sec.writeDeadlineActive = writeDeadline
	return sec
}

func (sec *SecureNanoBuilder) MessageIdentity(messageIdent proto.Message) *SecureNanoBuilder {
	sec.receiverMessageIdentity = messageIdent
	return sec
}

func (sec *SecureNanoBuilder) SendBuffer(sendBuffer int) *SecureNanoBuilder {
	sec.sendBuffer = sendBuffer
	return sec
}

func (sec *SecureNanoBuilder) ReceiveBuffer(receiveBuffer int) *SecureNanoBuilder {
	sec.receiveBuffer = receiveBuffer
	return sec
}

func (sec *SecureNanoBuilder) enableEncryption(secretKey *[32]byte, authKey *[32]byte) *SecureNanoBuilder {
	sec.encdecKey = secretKey
	sec.hmacKey = authKey
	return sec
}

func (sec *SecureNanoBuilder) BuildNan0() (nan0 *Nan0, err error) {
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
	nan0 = &Nan0{
		ServiceName:    sec.ns.ServiceName,
		receiver:       make(chan interface{}, sec.receiveBuffer),
		sender:         make(chan interface{}, sec.sendBuffer),
		conn:           conn,
		closed:         false,
		writerShutdown: make(chan bool, 1),
		readerShutdown: make(chan bool, 1),
	}

	go nan0.startServiceReceiver(sec.receiverMessageIdentity, sec.encdecKey, sec.hmacKey)
	go nan0.startServiceSender(sec.writeDeadlineActive, sec.encdecKey, sec.hmacKey)
	return nan0, err
}

// Create a connection to this nanoservice using the Nan0 wrapper around a protocol buffer service layer
func (ns *Service) DialNan0(writeDeadlineActive bool, receiverMessageIdentity proto.Message, sendBuffer, receiveBuffer int) (nan0 *Nan0, err error) {
	return ns.DialNan0Secure(nil, nil).
		ToggleWriteDeadline(writeDeadlineActive).
		MessageIdentity(receiverMessageIdentity).
		SendBuffer(sendBuffer).
		ReceiveBuffer(receiveBuffer).
		BuildNan0()
}

// Start the process of creating a secure nanoservice using a builder for the parameters
func (ns *Service) DialNan0Secure(secretKey *[32]byte, authKey *[32]byte) *SecureNanoBuilder {
	return new(SecureNanoBuilder).enableEncryption(secretKey, authKey).setService(ns)
}

// Start the active receiver for this Nan0 connection. This enables the 'receiver' channel,
// constantly reads from the open connection and places the received message on receiver channel
func (n *Nan0) startServiceReceiver(msg proto.Message, decryptKey *[32]byte, hmacKey *[32]byte) {
	if n.conn != nil && !n.closed {
		for ; ; {
			n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))

			//b := make([]byte, 128)
			//n.conn.Read(b)
			//debug("New Message Received\n\t\tBytes: %v\n\t\tSize: %v", b, len(b))
			newMsg := proto.Clone(msg)
			err := getMessageFromConnection(n.conn, &newMsg, decryptKey, hmacKey)
			if err == nil {
				debug("sending %v on receiver", newMsg)
				// Send the message received to the awaiting receive buffer
				n.receiver <- newMsg
			}
			select {
			case <-n.readerShutdown:
				n.writerShutdown <- true
				info("Shutting down service receiver for %v", n.ServiceName)
				return
			default:
			}
		}
	}
}

// Start the active sender for this Nan0 connection. This enables the 'sender' channel and allows the user to send
// protocol buffer messages to the server
func (n *Nan0) startServiceSender(writeDeadlineIsActive bool, encryptKey *[32]byte, hmacKey *[32]byte) {
	if n.conn != nil && !n.closed {
		for ; ; {
			if writeDeadlineIsActive {
				n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
			}
			select {
			case pb := <-n.sender:
				debug("Sending message %v", pb)
				putMessageInConnection(n.conn, pb.(proto.Message), encryptKey, hmacKey)
			case <-n.writerShutdown:
				n.readerShutdown <- true
				info("Shutting down service sender for %v", n.ServiceName)
				return
			default:
			}
		}
	}
}

// Closes the open connection and terminates the goroutines associated with reading them
func (n *Nan0) Close() {
	if n.closed {
		return
	}
	n.closed = true
	n.readerShutdown <- true
	info("Reader stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.writerShutdown <- true
	info("Writer stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.conn.Close()
	info("Dialed connection for server %v closed after shutdown signal received", n.ServiceName)
	// wait until both goroutines are closed
	<-n.readerShutdown
	<-n.writerShutdown
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n *Nan0) GetSender() chan<- interface{} {
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n *Nan0) GetReceiver() <-chan interface{} {
	return n.receiver
}
