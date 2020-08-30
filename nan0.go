package nan0

/**
Service Discovery API
Some features that are implemented here:
DiscoveryService implements Stringer, io.Reader, io.Writer
This API accepts and manages nanoservices
*/
import (
	"bufio"
	"net"
	"time"

	"github.com/yomiji/slog"
	"google.golang.org/protobuf/proto"
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
	closed chan struct{}
	// Channel governing the reader service
	readerShutdown chan bool
	// Channel governing the writer service
	writerShutdown chan bool
	// Channel governing the shutdown completion
	closeComplete chan bool
}

// Start the active receiver for this Nan0 connection. This enables the 'receiver' channel,
// constantly reads from the open connection and places the received message on receiver channel
func (n Nan0) startServiceReceiver(identMap map[int]proto.Message, decryptKey *[32]byte, hmacKey *[32]byte) {
	defer recoverPanic(func(e error) {
		slog.Fail("Connection to %v receiver service error occurred: %v", n.GetServiceName(), e)
	})()
	defer func() {
		n.closeComplete <- true
		slog.Debug("Shutting down service receiver for %v", n.ServiceName)
	}()
	if n.conn != nil && !n.IsClosed() {
		for ; ; {
			select {
			case <-n.readerShutdown:
				return
			default:
				conn := bufio.NewReader(n.conn)
				err := n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))
				checkError(err)

				var newMsg proto.Message

				newMsg, err = getMessageFromConnection(conn, identMap, decryptKey, hmacKey)
				if err != nil && newMsg == nil {
					panic(err)
				}

				if newMsg != nil {
					//Send the message received to the awaiting receive buffer
					n.receiver <- newMsg
				}
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// Start the active sender for this Nan0 connection. This enables the 'sender' channel and allows the user to send
// protocol buffer messages to the server
func (n *Nan0) startServiceSender(inverseMap map[string]int, writeDeadlineIsActive bool, encryptKey *[32]byte, hmacKey *[32]byte) {
	defer recoverPanic(func(e error) {
		slog.Fail("Connection to %v sender service error occurred: %v", n.GetServiceName(), e)
	})()
	defer func() {
		n.closeComplete <- true
		slog.Debug("Shutting down service sender for %v", n.ServiceName)
	}()
	if n.conn != nil && !n.IsClosed() {
		for ; ; {
			select {
			case <-n.writerShutdown:
				return
			case pb := <-n.sender:
				slog.Debug("N.Conn: %v Remote:%v", n.conn.LocalAddr(), n.conn.RemoteAddr())
				if writeDeadlineIsActive {
					err := n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
					checkError(err)
				}
				err := putMessageInConnection(n.conn, pb.(proto.Message), inverseMap, encryptKey, hmacKey)
				if err != nil {
					checkError(err)
				}
			default:
				time.Sleep(1 * time.Millisecond)
			}
		}
	}
}

// performs a close on everything except closing the connection
func (n *Nan0) softClose() {
	defer recoverPanic(func(e error) {
		slog.Fail("Failed to close %s due to %v", n.ServiceName, e)
	})
	if n.IsClosed() {
		return
	}
	n.readerShutdown <- true
	slog.Debug("Reader stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.writerShutdown <- true
	_ = n.conn.SetReadDeadline(time.Now())
	slog.Debug("Writer stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	<-n.closeComplete
	<-n.closeComplete
	slog.Debug("Dialed connection for server %v closed after shutdown signal received", n.ServiceName)
	// after goroutines are closed, close the read/write channels
	close(n.receiver)
	close(n.sender)
	close(n.closed)
	slog.Debug("Connection %v is soft closed", n.ServiceName)
}

// Closes the open connection and terminates the goroutines associated with reading them
func (n *Nan0) Close() {
	defer recoverPanic(func(e error) {
		slog.Fail("Failed to close %s due to %v", n.ServiceName, e)
	})
	if n.IsClosed() {
		slog.Warn("%s already closed.", n.ServiceName)
		return
	}
	n.softClose()
	_ = n.conn.SetReadDeadline(time.Now())
	_ = n.conn.Close()
	slog.Warn("Connection to %v is shut down!", n.ServiceName)
}

// Determine if this connection is closed
func (n Nan0) IsClosed() bool {
	select {
	case <-n.closed:
		return true
	default:
		return false
	}
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n Nan0) GetSender() chan<- interface{} {
	// wait on the close to complete before returning the sender
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n Nan0) GetReceiver() <-chan interface{} {
	// wait on the close to complete before returning the receiver
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
