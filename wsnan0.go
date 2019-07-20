package nan0

import (
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)
// The WsNan0 structure is a wrapper around a websocket connection which sends
// and receives protocol buffers across it. Use the provided Builder object to
// correctly initialize this structure.
type WsNan0 struct {
	// The name of the service
	ServiceName string
	// Receive messages from this channel
	receiver chan interface{}
	// Messages placed on this channel will be sent
	sender chan interface{}
	// The closed status
	closed bool
	// Channel governing the reader service
	readerShutdown chan bool
	// Channel governing the writer service
	writerShutdown chan bool
	// Channel governing the shutdown completion
	closeComplete chan bool

	rxHeartbeat chan bool
	txHeartbeat chan bool

	// A connection maintained by this object
	conn *websocket.Conn

	mutex sync.Mutex
}

// Start the active sender for this Nan0 connection. This enables the 'sender' channel and allows the user to send
// protocol buffer messages to the server
func (n WsNan0) startServiceSender(inverseMap map[string]int, writeDeadlineIsActive bool) {
	defer recoverPanic(func(e error) {
		fail("Connection to %v sender service error occurred: %v", n.GetServiceName(), e)
	})()
	defer func() {
		n.closeComplete <- true
		debug("Shutting down service sender for %v", n.ServiceName)
	}()
	if n.conn != nil && !n.IsClosed() {
		for ; ; {
			select {
			case <-n.writerShutdown:
				return
			case pb := <-n.sender:
				debug("N.Conn: %v Remote:%v", n.conn.LocalAddr(), n.conn.RemoteAddr())
				if writeDeadlineIsActive {
					err := n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
					checkError(err)
				}
				debug("Sending message %v", pb)
				err := putMessageInConnectionWs(n.conn, pb.(proto.Message), inverseMap)
				if err != nil {
					checkError(err)
				}
			default:
				time.Sleep(10 * time.Microsecond)
			}
		}
	}
}

// Closes the open connection and terminates the goroutines associated with reading them
func (n WsNan0) Close() {
	if n.IsClosed() {
		return
	}
	defer recoverPanic(func(e error) {
		fail("Failed to close %s due to %v", n.ServiceName, e)
	})

	n.readerShutdown <- true
	debug("Reader stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	n.writerShutdown <- true
	_ = n.conn.SetReadDeadline(time.Now())
	debug("Writer stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
	<-n.closeComplete
	<-n.closeComplete
	n.mutex.Lock()
	_ = n.conn.Close()
	debug("Dialed connection for server %v closed after shutdown signal received", n.ServiceName)
	// after goroutines are closed, close the read/write channels
	defer n.mutex.Unlock()
	n.closed = true
	warn("Connection to %v is shut down!", n.ServiceName)
}

// Determine if this connection is closed
func (n WsNan0) IsClosed() bool {
	n.mutex.Lock()
	defer	n.mutex.Unlock()
	c := n.closed
	return c
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n WsNan0) GetSender() chan<- interface{} {
	if n.IsClosed() {
		return nil
	}
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n WsNan0) GetReceiver() <-chan interface{} {
	if n.IsClosed() {
		return nil
	}
	return n.receiver
}

// Get the service name identifier
func (n WsNan0) GetServiceName() string {
	return n.ServiceName
}

// Determine if two instances are equal
func (n WsNan0) Equals(other NanoServiceWrapper) bool {
	return n.GetServiceName() == other.GetServiceName()
}

// Start the active receiver for this Nan0 connection. This enables the 'receiver' channel,
// constantly reads from the open connection and places the received message on receiver channel
func (n WsNan0) startServiceReceiver(identMap map[int]proto.Message) {
	defer recoverPanic(func(e error) {
		fail("Connection to %v receiver service error occurred: %v", n.GetServiceName(), e)
	})()
	defer func() {
		n.closeComplete <- true
		debug("Shutting down service receiver for %v", n.ServiceName)
	}()

	if n.conn != nil && !n.IsClosed() {
		for ; !n.IsClosed() ; {
			select {
			case <-n.readerShutdown:
				return
			default:
				err := n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))
				checkError(err)

				var newMsg proto.Message

				newMsg, err = getMessageFromConnectionWs(n.conn, identMap)
				if err != nil && newMsg == nil {
					panic(err)
				}

				if newMsg != nil {
					debug("sending %v on receiver", newMsg)
					//Send the message received to the awaiting receive buffer
					n.receiver <- newMsg
				}
			}
			time.Sleep(10 * time.Microsecond)
		}
	}
}