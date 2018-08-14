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

// Start the active receiver for this Nan0 connection. This enables the 'receiver' channel,
// constantly reads from the open connection and places the received message on receiver channel
func (n Nan0) startServiceReceiver(identMap map[int] proto.Message, decryptKey *[32]byte, hmacKey *[32]byte) {
	if n.conn != nil && !n.closed {
		for ; ; {
			n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))


			newMsg, err := getMessageFromConnection(n.conn, identMap, decryptKey, hmacKey)
			if newMsg != nil && err == nil {
				debug("sending %v on receiver", newMsg)
				// Send the message received to the awaiting receive buffer
				n.receiver <- newMsg
			}
			select {
			case <-n.readerShutdown:
				n.writerShutdown <- true
				debug("Shutting down service receiver for %v", n.ServiceName)
				return
			default:
			}
		}
	}
}

// Start the active sender for this Nan0 connection. This enables the 'sender' channel and allows the user to send
// protocol buffer messages to the server
func (n Nan0) startServiceSender(inverseMap map[string]int, writeDeadlineIsActive bool, encryptKey *[32]byte, hmacKey *[32]byte) {
	if n.conn != nil && !n.closed {
		for ; ; {
			if writeDeadlineIsActive {
				n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
			}
			select {
			case pb := <- n.sender:
				debug("Sending message %v", pb)
				putMessageInConnection(n.conn, pb.(proto.Message), inverseMap, encryptKey, hmacKey)
			default:
			}
			select {
			case <-n.writerShutdown:
				n.readerShutdown <- true
				debug("Shutting down service sender for %v", n.ServiceName)
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
	debug("Reader stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.writerShutdown <- true
	debug("Writer stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.conn.Close()
	debug("Dialed connection for server %v closed after shutdown signal received", n.ServiceName)
	// wait until both goroutines are closed
	<-n.readerShutdown
	<-n.writerShutdown
	warn("Connection to %v is shut down!", n.ServiceName)
}

// Determine if this connection is closed
func (n Nan0) IsClosed() bool {
	return n.closed
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n Nan0) GetSender() chan<- interface{} {
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n Nan0) GetReceiver() <-chan interface{} {
	return n.receiver
}

// Get the service name identifier
func (n Nan0) GetServiceName() string {
	return n.ServiceName
}

// Determine if two instances are equal
func (n Nan0) Equals(other NanoServiceWrapper) bool {
	return n.GetServiceName() == other.GetServiceName()
}