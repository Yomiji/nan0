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
	"log"
	"os"
	"fmt"
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
	receiver chan proto.Message
	// Messages placed on this channel will be sent
	sender chan proto.Message
	// A connection maintained by this object
	conn net.Conn
	// The closed status
	closed bool
	// Channel governing the reader service
	readerShutdown chan bool
	// Channel governing the writer service
	writerShutdown chan bool
}

/*************
	Logging
************/

//Loggers provided:
//  Level | Output | Format
// 	Info: Standard Output - 'Nan0 [INFO] %date% %time% %filename:line%'
// 	Warn: Standard Error - 'Nan0 [DEBUG] %date% %time% %filename:line%'
// 	Error: Standard Error - 'Nan0 [ERROR] %date% %time% %filename:line%'
// 	Debug: Standard Output - 'Nan0 [DEBUG] %date% %time% %filename:line%'
var (
	Info  = log.New(os.Stdout, "Nan0 [INFO]: ", log.Ldate|log.Ltime|log.Lshortfile)
	Warn  = log.New(os.Stderr, "Nan0 [DEBUG]: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(os.Stderr, "Nan0 [ERROR]: ", log.Ldate|log.Ltime|log.Lshortfile)
	Debug = log.New(os.Stdout, "Nan0 [DEBUG]: ", log.Ldate|log.Ltime|log.Lshortfile)
)

// Wrapper around the Info global log that allows for this api to log to that level correctly
func info(msg string, vars...interface{}) {
	if Info != nil {
		Info.Printf(msg, vars...)
	}
}

// Wrapper around the Warn global log that allows for this api to log to that level correctly
func warn(msg string, vars...interface{}) {
	if Warn != nil {
		Warn.Printf(msg, vars...)
	}
}

// Wrapper around the Error global log that allows for this api to log to that level correctly
func fail(msg string, vars...interface{}) {
	if Error != nil {
		Error.Printf(msg, vars...)
	}
}

// Wrapper around the Debug global log that allows for this api to log to that level correctly
func debug(msg string, vars...interface{}) {
	if Debug != nil {
		Debug.Printf(msg, vars...)
	}
}

// Conveniently disable all logging for this api
func noLogging() {
	Info = nil
	Warn = nil
	Error = nil
	Debug = nil
}

// The timeout for TCP Writers and server connections
var TCPTimeout = 10 * time.Second

/*******************
 	Service API
 *******************/

// Starts the listener for the given service
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
	defer recoverPanic(func(e error) { err = e.(error) })
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
	conn, err := net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))

	return conn, err
}

// Create a connection to this nanoservice using the Nan0 wrapper around a protocol buffer service layer
func (ns *Service) DialNan0(writeDeadlineActive bool, receiverMessageIdentity proto.Message) (nan0 *Nan0, err error) {
	defer recoverPanic(func(e error) {
		nan0 = &Nan0{
			ServiceName:    ns.ServiceName,
			receiver:       nil,
			sender:         nil,
			conn:           nil,
			closed:         true,
			writerShutdown: nil,
			readerShutdown: nil,
		}
		err = e.(error)
	})
	conn, err := net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))
	nan0 = &Nan0{
		ServiceName:    ns.ServiceName,
		receiver:       make(chan proto.Message),
		sender:         make(chan proto.Message),
		conn:           conn,
		closed:         false,
		writerShutdown: make(chan bool, 1),
		readerShutdown: make(chan bool, 1),
	}
	checkError(err)

	go nan0.startServiceReceiver(receiverMessageIdentity)
	go nan0.startServiceSender(writeDeadlineActive)
	return nan0, err
}

// Start the active receiver for this Nan0 connection. This enables the 'receiver' channel,
// constantly reads from the open connection and places the received message on receiver channel
func (n *Nan0) startServiceReceiver(msg proto.Message) {
	if n.conn != nil && !n.closed {
		for ; ; {
			n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))
			b := make([]byte, 1024)
			_, err := n.conn.Read(b)
			if err != nil {
				info("Reader error on connection for Server '%v'", n.ServiceName)
				continue
			}
			err = proto.Unmarshal(b, msg)
			if err != nil {
				info("Unmarshaler error on connection for Server '%v'", n.ServiceName)
				continue
			}
			// Send the message received to the awaiting receive buffer
			n.receiver <- msg
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
func (n *Nan0) startServiceSender(writeDeadlineIsActive bool) {
	if n.conn != nil && !n.closed {
		for ; ; {
			if writeDeadlineIsActive {
				n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
			}
			pb := <-n.sender
			b, err := proto.Marshal(pb)
			if err != nil {
				info("Marshaller error on connection for Server '%v'", n.ServiceName)
				continue
			}
			_, err = n.conn.Write(b)
			if err != nil {
				info("Writer error on connection for Server '%v'", n.ServiceName)
				continue
			}

			select {
			case <-n.writerShutdown:
				n.readerShutdown <- true
				info("Shutting down service writer for %v", n.ServiceName)
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
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n *Nan0) GetSender() chan<- proto.Message {
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n *Nan0) GetReceiver() <-chan proto.Message {
	return n.receiver
}

/*************************
	Helper Functions
*************************/

//	Checks the error passed and panics if issue found
func checkError(err error) {
	if err != nil {
		info("Error occurred: %s", err.Error())
		panic(err)
	}
}

// Combines a host name string with a port number to create a valid tcp address
func composeTcpAddress(hostName string, port int32) string {
	return fmt.Sprintf("%s:%d", hostName, port)
}

// Recovers from a panic using the recovery method and applies the given behavior when a recovery
// has occurred.
func recoverPanic(errfunc func(error)) func() {
	if errfunc != nil {
		return func() {
			if e := recover(); e != nil {
				// execute the abstract behavior
				errfunc(e.(error))
			}
		}
	} else {
		return func() {
			recover()
		}
	}
}
