package nan0

import (
	"errors"
	"sync"
	"time"

	"github.com/yomiji/slog"
	"github.com/yomiji/websocket"
	"google.golang.org/protobuf/proto"
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
	closed chan struct{}
	// Channel governing the reader service
	readerShutdown chan struct{}
	// Channel governing the writer service
	writerShutdown chan struct{}
	// Last communication time
	lastTxRx time.Time
	// A connection maintained by this object
	conn *websocket.Conn
	// close mutex
	closeMux *sync.Mutex
}

func (n WsNan0) LastComm() time.Time {
	return n.lastTxRx
}

// Start the active sender for this Nan0 connection. This enables the 'sender' channel and allows the user to send
// protocol buffer messages to the server
func (n WsNan0) startServiceSender(inverseMap map[string]int, writeDeadlineIsActive bool, _, _ *[32]byte) {
	defer recoverPanic(func(e error) {
		slog.Fail("Connection to %v sender service error occurred: %v", n.GetServiceName(), e)
	})()
	if n.conn != nil && !n.IsClosed() {
		for ; ; {
			select {
			case <-n.writerShutdown:
				panic(errors.New("writer shutdown"))
			case pb := <-n.sender:
				slog.Debug("N.Conn: %v Remote:%v", n.conn.LocalAddr(), n.conn.RemoteAddr())
				if writeDeadlineIsActive {
					err := n.conn.SetWriteDeadline(time.Now().Add(TCPTimeout))
					checkError(err)
				}
				err := putMessageInConnectionWs(n.conn, pb.(proto.Message), inverseMap)
				if err != nil {
					checkError(err)
				}
				n.lastTxRx = time.Now()
			default:
				time.Sleep(10 * time.Microsecond)
			}
		}
	}
}

func (n WsNan0) softClose() {
	panic("this is a no-op")
}

// Closes the open connection and terminates the goroutines associated with reading them
func (n *WsNan0) Close() {
	defer recoverPanic(func(e error) {
		slog.Fail("Failed to close %s due to %v", n.ServiceName, e)
	})

	n.closeMux.Lock()
	defer n.closeMux.Unlock()
	if n.IsClosed() {
		return
	}
	go func() {
		n.readerShutdown <- struct{}{}
		slog.Debug("Reader stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
		n.writerShutdown <- struct{}{}
		slog.Debug("Writer stream for Nan0 server '%v' shutdown signal sent", n.ServiceName)
		_ = n.conn.SetReadDeadline(time.Now())
	}()
	slog.Debug("Dialed connection for server %v closed after shutdown signal received", n.ServiceName)
	_ = n.conn.Close()
	close(n.closed)
	slog.Debug("Connection %v is closed", n.ServiceName)
}

// Determine if this connection is closed
func (n *WsNan0) IsClosed() bool {
	select {
	case <-n.closed:
		return true
	default:
		return false
	}
}

// Return a write-only channel that is used to send a protocol buffer message through this connection
func (n WsNan0) GetSender() chan<- interface{} {
	return n.sender
}

// Returns a read-only channel that is used to receive a protocol buffer message returned through this connection
func (n WsNan0) GetReceiver() <-chan interface{} {
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
func (n WsNan0) startServiceReceiver(rmap RouteMap, identMap map[int]proto.Message, _, _ *[32]byte) {
	defer recoverPanic(func(e error) {
		slog.Fail("Connection to %v receiver service error occurred: %v", n.GetServiceName(), e)
	})()

	if n.conn != nil && !n.IsClosed() {
		for ; ; {
			select {
			case <-n.readerShutdown:
				panic(errors.New("reader shutdown"))
			default:
				err := n.conn.SetReadDeadline(time.Now().Add(TCPTimeout))
				checkError(err)

				var newMsg proto.Message

				newMsg, err = getMessageFromConnectionWs(n.conn, identMap)
				if err != nil {
					panic(err)
				}

				if newMsg != nil {
					if exec, ok := rmap[getProtobufMessageName(newMsg)]; ok {
						//Execute any mapped routes
						exec.Execute(newMsg, n.sender)
					} else if exec, ok := rmap[DefaultRoute]; len(rmap) > 0 && ok {
						//If no mapped routes, execute default route if defined
						exec.Execute(newMsg, n.sender)
					} else {
						//If no routes defined, send the message received to the receive buffer
						// for manual processing
						n.receiver <- newMsg
					}
					n.lastTxRx = time.Now()
				}
			}
			time.Sleep(10 * time.Microsecond)
		}
	}
}

// Places the given protocol buffer message in the connection, the connection will receive the following data:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func putMessageInConnectionWs(conn *websocket.Conn, pb proto.Message, inverseMap map[string]int) (err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("Message failed to send: %v due to %v", pb, e)
		err = e
	})()

	// figure out if the type of the message is in our list
	typeString := getProtobufMessageName(pb)
	typeVal, ok := inverseMap[typeString]
	if !ok {
		checkError(errors.New("type value for message not present"))
	}

	var bigBytes []byte
	// marshal the protobuf message
	v, err := proto.Marshal(pb)
	checkError(err)
	protoSize := len(v)
	//prepare all items
	bigBytes = append(ProtoPreamble, SizeWriter(typeVal)...)
	bigBytes = append(bigBytes, SizeWriter(protoSize)...)
	bigBytes = append(bigBytes, v...)

	// write the preamble, sizes and message
	slog.Debug("Writing to connection")
	err = conn.WriteMessage(websocket.BinaryMessage, bigBytes)
	checkError(err)

	return err
}

// Retrieves the given protocol buffer message from the connection, the connection is expected to send the following:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func getMessageFromConnectionWs(conn *websocket.Conn, identMap map[int]proto.Message) (msg proto.Message, err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("Failed to receive message due to %v", e)
		msg = nil
		err = e
	})()
	// get total message all at once
	var buffer []byte
	t, buffer, err := conn.ReadMessage()
	if t != websocket.BinaryMessage {
		if t == websocket.CloseMessage {
			return nil, errors.New("client connection closed")
		}
		return nil, nil
	}
	checkError(err)

	// get the preamble
	preamble := buffer[0:len(ProtoPreamble)]
	// check the preamble
	err = isPreambleValidWs(preamble)
	checkError(err)
	// get the message type
	messageTypeIdex := len(ProtoPreamble)
	messageTypeBuf := buffer[messageTypeIdex:(messageTypeIdex + SizeArrayWidth)]
	messageType := SizeReader(messageTypeBuf)

	// clone message using the retrieved message type
	msg = proto.Clone(identMap[messageType])

	// get the size of the next message
	sizeIdex := messageTypeIdex + SizeArrayWidth
	sizeBuf := buffer[sizeIdex:(sizeIdex + SizeArrayWidth)]
	size := SizeReader(sizeBuf)

	valueIdex := sizeIdex + SizeArrayWidth
	valueBuf := buffer[valueIdex:]

	count := len(valueBuf)

	// check the number of bytes received matches the bytes expected
	if count != size {
		checkError(errors.New("message size discrepancy while sending"))
	}

	err = proto.Unmarshal(valueBuf, msg)
	checkError(err)

	return msg, err
}
