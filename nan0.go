package nan0

/**
Service Discovery API
Some features that are implemented here:
DiscoveryService implements Stringer, io.Reader, io.Writer
This API accepts and manages nanoservices
 */
import (
	"fmt"
	"time"
	"github.com/golang/protobuf/proto"
	"net"
	"os"
	"log"
)

type Nan0 struct {
	// The name of the service
	ServiceName    string
	// Receive messages from this channel
	receiver chan *proto.Message
	// Messages placed on this channel will be sent
	sender chan *proto.Message
	// A connection maintained by this object
	conn           net.Conn
	// The closed status
	closed         bool
	// Channel governing the reader service
	readerShutdown chan bool
	// Channel governing the writer service
	writerShutdown chan bool
}

// A logger with a 'Service API' prefix used to log  s from this api. Call log.SetOutput to change output
// writer
var Logger = log.New(os.Stderr, "Service API: ", log.Ldate|log.Ltime|log.Lshortfile)

// The timeout for TCP Writers and server connections
var TCPTimeout = 10 * time.Second



/*
 *	Service API
 */

// Checks if a particular nanoservice is expired based on its start time and time to live
func (ns Service) IsExpired() bool {
	nowInMS := time.Now().Unix()
	return (nowInMS - ns.StartTime) >= ns.TimeToLiveInMS
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
		return false
	}
	return true
}

// Refreshes the start time so that this service does not expire
func (ns *Service) Refresh() {
	ns.StartTime = time.Now().Unix()
}

/*
 *	Helper Functions
 */

// Create a connection to this nanoservice using a traditional TCP connection
func (ns *Service) DialTCP() (nan0 net.Conn, err error) {
	conn, err := net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))

	return conn, err
}

// Registers this nanoservice with the service discovery host at the given address
func (ns *Service) Register(host string, port int32) (err error) {
	address := composeTcpAddress(host, port)
	Logger.Printf("Registering '%v' service with discovery at '%v'", ns.ServiceName, address)
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
		ns.TimeToLiveInMS == other.TimeToLiveInMS &&
		ns.StartTime == other.StartTime &&
		ns.ServiceName == other.ServiceName &&
		ns.ServiceType == other.ServiceType
}

// Create a connection to this nanoservice using the Nan0 wrapper around a protocol buffer service layer
func (ns *Service) DialNan0(writeDeadlineActive bool, receiverMessageIdentity *proto.Message) (nan0 *Nan0, err error) {
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
		receiver:       make(chan *proto.Message),
		sender:         make(chan *proto.Message),
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

// constantly reads from the open connection and places the received message on receiver channel
func (n *Nan0) startServiceReceiver(msg *proto.Message) {
	if n.conn != nil && !n.closed {
		for ; ; {
			n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))
			b := make([]byte, 1024)
			_, err := n.conn.Read(b)
			if err != nil {
				Logger.Printf("Reader error on connection for Server '%v'", n.ServiceName)
				continue
			}
			err = proto.Unmarshal(b, msg)
			if err != nil {
				Logger.Printf("Unmarshaler error on connection for Server '%v'", n.ServiceName)
				continue
			}
			// Send the message received to the awaiting receive buffer
			n.receiver <- msg
			select {
			case <-n.readerShutdown:
				n.writerShutdown <- true
				Logger.Printf("Shutting down service receiver for %v", n.ServiceName)
				return
			default:
			}
		}
	}
}

func (n *Nan0) startServiceSender(writeDeadlineIsActive bool) {
	if n.conn != nil && !n.closed {
		for ; ; {
			if writeDeadlineIsActive {
				n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
			}
			pb := <-n.sender
			b, err := proto.Marshal(pb)
			if err != nil {
				Logger.Printf("Marshaller error on connection for Server '%v'", n.ServiceName)
				continue
			}
			_, err = n.conn.Write(b)
			if err != nil {
				Logger.Printf("Writer error on connection for Server '%v'", n.ServiceName)
				continue
			}

			select {
			case <-n.writerShutdown:
				n.readerShutdown <- true
				Logger.Printf("Shutting down service writer for %v", n.ServiceName)
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
	Logger.Printf("Reader stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.writerShutdown <- true
	Logger.Printf("Writer stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n *Nan0) GetSender() chan<- *proto.Message {
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n *Nan0) GetReceiver() <-chan *proto.Message {
	return n.receiver
}

// Checks the error passed and panics if issue found
func checkError(err error) {
	if err != nil {
		Logger.Printf("Error occurred: %s", err.Error())
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
