package nan0

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/yomiji/slog"
	"github.com/yomiji/websocket"
)

/*******************
	Service Params
 *******************/

// The max queued connections for NanoServer handling, used with ListenTCP()
var MaxNanoCache = 50

// The timeout for TCP Writers and server connections
var TCPTimeout = 10 * time.Second

/*******************
	TCP Negotiate
 *******************/

// TCP Preamble, this pre-fixes a unique tcp transmission (protocol buffer message)
var ProtoPreamble = []byte{0x01, 0x02, 0x03, 0xFF, 0x03, 0x02, 0x01}

// The default array width provided with this API, this is the default value of the SizeArrayWidth
const defaultArrayWidth = 4

// The number of bytes that constitute the size of the proto.Message sent/received
// This variable is made visible so that developers can support larger data sizes
var SizeArrayWidth = defaultArrayWidth

// Converts the size header from the byte slice that represents it to an integer type
// NOTE: If you change SizeArrayWidth, you should also change this function
// This function is made visible so that developers can support larger data sizes
var SizeReader = func(bytes []byte) int {
	return int(binary.BigEndian.Uint32(bytes))
}

// Converts the integer representing the size of the following protobuf message to a byte slice
// NOTE: If you change SizeReader, you should also change this function
// This function is made visible so that developers can support variable data sizes
var SizeWriter = func(size int) (bytes []byte) {
	bytes = make([]byte, SizeArrayWidth)
	binary.BigEndian.PutUint32(bytes, uint32(size))
	return bytes
}

/*************************
	Helper Functions
*************************/

// From
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

//	Checks the error passed and panics if issue found
func checkError(err error) {
	if err != nil {
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

// Places the given protocol buffer message in the connection, the connection will receive the following data:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func putMessageInConnection(conn net.Conn, pb proto.Message, inverseMap map[string]int) (err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("Message failed to send: %v due to %v", pb, e)
		err = e
	})()

	// figure out if the type of the message is in our list
	typeString := proto.MessageName(pb)
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
	n, err := conn.Write(bigBytes)
	checkError(err)

	// check the full buffer was written
	totalSize := len(bigBytes)
	if totalSize != n {
		slog.Debug("discrepancy in number of bytes written for message. expected: %v, got: %v", totalSize, n)
		err = errors.New("message size discrepancy while sending")
	} else {
		slog.Debug("wrote message to connection with byte size: %v", len(ProtoPreamble)+SizeArrayWidth+n)
	}
	return err
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
	typeString := proto.MessageName(pb)
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
func getMessageFromConnection(conn net.Conn, identMap map[int]proto.Message) (msg proto.Message, err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("Failed to receive message due to %v", e)
		msg = nil
		err = e
	})()

	// check the preamble
	err = isPreambleValid(conn)
	if err == io.EOF {
		return nil, nil
	}
	checkError(err)
	// get the message type bytes
	messageType := readSize(conn)
	msg = proto.Clone(identMap[messageType])

	// get the size of the next message
	size := readSize(conn)
	// create a byte buffer that will store the whole expected message
	v := make([]byte, size)
	// get the protobuf bytes from the reader
	count, err := conn.Read(v)
	checkError(err)
	slog.Debug("Read data %v", v)

	// check the number of bytes received matches the bytes expected
	if count != size {
		checkError(errors.New("message size discrepancy while sending"))
	}

	err = proto.Unmarshal(v, msg)
	checkError(err)

	return msg, err
}

// Checks the preamble bytes to determine if the expected matches
func isPreambleValid(reader io.Reader) (err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("preamble issue: %v", e)
		err = e
	})()
	b := make([]byte, len(ProtoPreamble))
	_, err = reader.Read(b)
	if err == io.EOF {
		return io.EOF
	}
	checkError(err)
	slog.Debug("checking %v against preamble %v", b, ProtoPreamble)
	for i, v := range b {
		if i < len(ProtoPreamble) && v != ProtoPreamble[i] {
			return errors.New("preamble invalid")
		}
	}
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

func makeSendChannelFromBuilder(sec NanoBuilder) (buf chan interface{}) {
	if sec.sendBuffer == 0 {
		buf = make(chan interface{})
	} else {
		buf = make(chan interface{}, sec.sendBuffer)
	}
	return
}

func makeReceiveChannelFromBuilder(sec NanoBuilder) (buf chan interface{}) {
	if sec.receiveBuffer == 0 {
		buf = make(chan interface{})
	} else {
		buf = make(chan interface{}, sec.receiveBuffer)
	}
	return
}

// Checks the preamble bytes to determine if the expected matches
func isPreambleValidWs(readBytes []byte) (err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("preamble issue: %v", e)
		err = e
	})()

	slog.Debug("checking %v against preamble %v", readBytes, ProtoPreamble)
	for i, v := range readBytes {
		if i < len(ProtoPreamble) && v != ProtoPreamble[i] {
			return errors.New("preamble invalid")
		}
	}
	return nil
}

// Grabs the next two bytes from the reader and figure out the size of the following protobuf message
func readSize(reader io.Reader) int {
	defer recoverPanic(func(e error) {
		slog.Warn("issue reading size: %v", e)
	})()
	bytes := make([]byte, SizeArrayWidth)
	_, err := reader.Read(bytes)
	slog.Debug("read size array %v", bytes)
	checkError(err)

	return SizeReader(bytes)
}
