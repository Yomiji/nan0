package nan0

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"time"

	"github.com/yomiji/slog"
	"google.golang.org/protobuf/proto"
)

/*******************
	Service Params
 *******************/

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

func AwaitNSignals(c chan struct{}, n int) {
	defer close(c)
	for i := n; i > 0; i-- {
		<-c
	}
	drainSignalChan(c)
}

func DoAfterNSignals(c chan struct{}, n int, action func()) {
	go func() {
		defer action()
		AwaitNSignals(c, n)
	}()
}

func DoAfterFirstSignal(c chan struct{}, action func()) {
	go func() {
		defer close(c)
		defer action()
		<-c
		drainSignalChan(c)
	}()
}

func CheckAndDo(awaitedCondition func() bool, do func()) {
	CheckAndDoWithDuration(awaitedCondition, do, 0)
}

func CheckAndDoWithDuration(awaitedCondition func() bool, do func(), after time.Duration) {
	if after == 0 {
		after = 333 * time.Microsecond
	}
	ticker := time.NewTicker(after)
	defer ticker.Stop()
	defer do()
	for awaitedCondition() {
		<-ticker.C
	}
}

func drainSignalChan(c <-chan struct{}) {
	go func() {for _ = range c{}}()
}

func drainTxRxChan(c <-chan interface{}) {
	go func() {for _ = range c{}}()
}

func getProtobufMessageName(message proto.Message) string {
	if message == nil {
		return ""
	}
	return string(message.ProtoReflect().Descriptor().FullName())
}

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
				if err,ok := e.(error); ok {
					errfunc(err)
				} else if err,ok := e.(string); ok {
					errfunc(errors.New(err))
				} else {
					slog.Fail("err: %v", e)
				}
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
func putMessageInConnection(conn io.Writer, pb proto.Message, inverseMap map[string]int, encryptKey *[32]byte, hmacKey *[32]byte) (err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("Message failed to send due to %v", e)
		err = e
	})()
	encrypted := encryptKey != nil && hmacKey != nil

	// figure out if the type of the message is in our list
	typeString := getProtobufMessageName(pb)
	typeVal, ok := inverseMap[typeString]
	if !ok {
		checkError(errors.New("type value for message not present"))
	}

	var bigBytes []byte
	if encrypted {
		bigBytes = EncryptProtobuf(pb, typeVal, encryptKey, hmacKey)
	} else {
		// marshal the protobuf message
		v, err := proto.Marshal(pb)
		checkError(err)
		protoSize := len(v)
		//prepare all items
		bigBytes = append(ProtoPreamble, SizeWriter(typeVal)...)
		bigBytes = append(bigBytes, SizeWriter(protoSize)...)
		bigBytes = append(bigBytes, v...)
	}
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

// Retrieves the given protocol buffer message from the connection, the connection is expected to send the following:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func getMessageFromConnection(conn io.Reader, identMap map[int]proto.Message, decryptKey *[32]byte, hmacKey *[32]byte) (msg proto.Message, err error) {
	defer recoverPanic(func(e error) {
		slog.Debug("Failed to receive message due to %v", e)
		msg = nil
		err = e
	})()

	// determine if this is an encrypted msg
	encrypted := decryptKey != nil && hmacKey != nil

	// check the preamble
	err = isPreambleValid(conn)
	if err == io.EOF {
		return nil, nil
	}
	checkError(err)
	// get the message type bytes
	messageType := readSize(conn)
	msg = proto.Clone(identMap[messageType])
	if msg == nil {
		checkError(fmt.Errorf("message ident %v not found", messageType))
	}
	// get the size of the hmac if it is encrypted
	var hmacSize int
	if encrypted {
		hmacSize = readSize(conn)
	}
	// get the size of the next message
	size := readSize(conn)
	// create a byte buffer that will store the whole expected message
	v := make([]byte, size)
	// get the protobuf bytes from the reader
	count, err := conn.Read(v)
	checkError(err)

	// check the number of bytes received matches the bytes expected
	if count != size {
		checkError(errors.New("message size discrepancy while sending"))
	}

	if encrypted {
		err := DecryptProtobuf(v, msg, hmacSize, decryptKey, hmacKey)
		checkError(err)
	} else {
		err = proto.Unmarshal(v, msg)
		checkError(err)
	}

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

func makeSendChannelFromBuilder(bb *baseBuilder) (buf chan interface{}) {
	if bb.sendBuffer == 0 {
		buf = make(chan interface{})
	} else {
		buf = make(chan interface{}, bb.sendBuffer)
	}
	return
}

func makeReceiveChannelFromBuilder(bb *baseBuilder) (buf chan interface{}) {
	if bb.receiveBuffer == 0 {
		buf = make(chan interface{})
	} else {
		buf = make(chan interface{}, bb.receiveBuffer)
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
	checkError(err)

	return SizeReader(bytes)
}
