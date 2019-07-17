package nan0

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/websocket"
	"hash/fnv"
	"io"
	"log"
	"net"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"time"
)

/*************
	Logging
************/

//Loggers provided:
//  Level | Output | Format
// 	Info: Standard Output - 'Nan0 [INFO] %date% %time%'
// 	Warn: Standard Error - 'Nan0 [DEBUG] %date% %time%'
// 	Error: Standard Error - 'Nan0 [ERROR] %date% %time%'
// 	Debug: Disabled by default
var (
	Info              = log.New(os.Stdout, "Nan0 [INFO]: ", log.Ldate|log.Ltime)
	Warn              = log.New(os.Stderr, "Nan0 [WARN]: ", log.Ldate|log.Ltime)
	Error             = log.New(os.Stderr, "Nan0 [ERROR]: ", log.Ldate|log.Ltime)
	Debug *log.Logger = nil
)
//Toggle line numbers for output messages
var infoLine = false
var warnLine = false
var failLine = false
var debugLine = false

func ToggleLineNumberPrinting(info, warn, fail, debug bool) {
	infoLine = info
	warnLine = warn
	failLine = fail
	debugLine = debug
}

// Wrapper around the Info global log that allows for this api to log to that level correctly
func info(msg string, vars ...interface{}) {
	if Info != nil {
		var formattedMsg = msg
		if infoLine {
			_, fn, line, _ := runtime.Caller(2)
			formattedMsg = fmt.Sprintf("%s:%d %s", fn, line, msg)
		}
		Info.Printf(formattedMsg, vars...)
	}
}

// Wrapper around the Warn global log that allows for this api to log to that level correctly
func warn(msg string, vars ...interface{}) {
	if Warn != nil {
		var formattedMsg = msg
		if warnLine {
			_, fn, line, _ := runtime.Caller(2)
			formattedMsg = fmt.Sprintf("%s:%d %s", fn, line, msg)
		}
		Warn.Printf(formattedMsg, vars...)
	}
}

// Wrapper around the Error global log that allows for this api to log to that level correctly
func fail(msg string, vars ...interface{}) {
	if Error != nil {
		var formattedMsg = msg
		if failLine {
			_, fn, line, _ := runtime.Caller(2)
			formattedMsg = fmt.Sprintf("%s:%d %s", fn, line, msg)
		}
		Error.Printf(formattedMsg, vars...)
	}
}

// Wrapper around the Debug global log that allows for this api to log to that level correctly
func debug(msg string, vars ...interface{}) {
	if Debug != nil {
		var formattedMsg = msg
		if debugLine {
			_, fn, line, _ := runtime.Caller(2)
			formattedMsg = fmt.Sprintf("%s:%d %s", fn, line, msg)
		}
		Debug.Printf(formattedMsg, vars...)
	}
}

// Conveniently disable all logging for this api
func NoLogging() {
	Info = nil
	Warn = nil
	Error = nil
	Debug = nil
}

func SetLogWriter(w io.Writer) {
	if Info != nil {
		Info = log.New(w, "Nan0 [INFO]: ", log.Ldate|log.Ltime)
	}
	if Warn != nil {
		Warn = log.New(w, "Nan0 [WARN]: ", log.Ldate|log.Ltime)
	}
	if Error != nil {
		Error = log.New(w, "Nan0 [ERROR]: ", log.Ldate|log.Ltime)
	}
	if Debug != nil {
		Debug = log.New(w, "Nan0 [DEBUG]: ", log.Ldate|log.Ltime)
	}
}

/*******************
	Service Params
 *******************/

// The max queued connections for Nan0Server handling, used with ListenTCP()
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
		debug("Message failed to send: %v due to %v", pb, e)
		err = e
	})()

	// figure out if the type of the message is in our list
	typeString := reflect.TypeOf(pb).String()
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
	debug("Writing to connection")
	n, err := conn.Write(bigBytes)
	checkError(err)

	// check the full buffer was written
	totalSize := len(bigBytes)
	if totalSize != n {
		debug("discrepancy in number of bytes written for message. expected: %v, got: %v", totalSize, n)
		err = errors.New("message size discrepancy while sending")
	} else {
		debug("wrote message to connection with byte size: %v", len(ProtoPreamble)+SizeArrayWidth+n)
	}
	return err
}

func pollUntilReady(conn net.Conn, totalWaitTimeSec float64) (tempBuf []byte, err error) {
	tempBuf = make([]byte, 1)
	startTime := time.Now()
	var accTime float64 = 0
	var wait = totalWaitTimeSec / float64(time.Second)
	debug("Polling connection until ready, source: %v dest: %v", conn.LocalAddr(), conn.RemoteAddr())
	debug ("Polling timeout in %v seconds", wait)
	var n int
	for n, _ = conn.Read(tempBuf); n <= 0 && accTime < wait; n, err = conn.Read(tempBuf) {
		accTime = time.Now().Sub(startTime).Seconds()
		time.Sleep(100 * time.Microsecond)
	}
	length := n
	if length > 0 {
		debug("Ready len %v of %v sec at %v", n, totalWaitTimeSec, accTime)
		return tempBuf, nil
	} else {
		debug("Returning w/o anything")
		return nil, err
	}
}
// Retrieves the given protocol buffer message from the connection, the connection is expected to send the following:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func getMessageFromConnection(conn net.Conn, identMap map[int]proto.Message) (msg proto.Message, err error) {
	defer recoverPanic(func(e error) {
		debug("Failed to receive message due to %v", e)
		msg = nil
		err = e
	})()
	debug("call get message")
	// get total message all at once
	var buffer = make([]byte,0)
	tempBuf := make([]byte, 1024)
	buffer, err = pollUntilReady(conn, float64(TCPTimeout))
	if err != nil {
		return nil, err
	}
	for n, err := conn.Read(tempBuf); err == nil && n > 0; n, err = conn.Read(tempBuf) {
		buffer = append(buffer, tempBuf[0:n]...)
	}
	if err == io.EOF || len(buffer) == 0 {
		return nil, nil
	}
	// get the preamble
	preamble := buffer[0 : len(ProtoPreamble)]
	// check the preamble
	err = isPreambleValid(preamble)
	checkError(err)
	// get the message type
	messageTypeIdex := len(ProtoPreamble)
	messageTypeBuf := buffer[messageTypeIdex : (messageTypeIdex + SizeArrayWidth)]
	messageType := SizeReader(messageTypeBuf)

	// clone message using the retrieved message type
	msg = proto.Clone(identMap[messageType])

	// get the size of the next message
	sizeIdex := messageTypeIdex + SizeArrayWidth
	sizeBuf := buffer[sizeIdex : (sizeIdex + SizeArrayWidth)]
	size := SizeReader(sizeBuf)


	valueIdex := sizeIdex + SizeArrayWidth
	valueBuf := buffer[valueIdex : ]

	count := len(valueBuf)

	// check the number of bytes received matches the bytes expected
	if count != size {
		checkError(errors.New("message size discrepancy while sending"))
	}

	err = proto.Unmarshal(valueBuf, msg)
	checkError(err)

	return msg, err
}

func dialWebsocket(sec NanoBuilder) (ws *websocket.Conn, err error) {
	defer recoverPanic(func(e error) {
		err = e.(error)
	})()
	//paths := strings.Split(sec.ns.Uri, "/")
	ws, err = websocket.Dial("ws://"+sec.ns.HostName+":"+strconv.Itoa(int(sec.ns.Port))+sec.ns.Uri, "", sec.origin)
	return ws, err
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
func isPreambleValid(readBytes []byte) (err error) {
	defer recoverPanic(func(e error) {
		debug("preamble issue: %v", e)
		err = e
	})()


	debug("checking %v against preamble %v", readBytes, ProtoPreamble)
	for i,v := range readBytes {
		if i < len(ProtoPreamble) && v != ProtoPreamble[i] {
			return errors.New("preamble invalid")
		}
	}
	return nil
}

// Grabs the next two bytes from the reader and figure out the size of the following protobuf message
func readSize(reader io.Reader) int {
	defer recoverPanic(func(e error) {
		warn("issue reading size: %v", e)
	})()
	bytes := make([]byte, SizeArrayWidth)
	_, err := reader.Read(bytes)
	debug("read size array %v", bytes)
	checkError(err)

	return SizeReader(bytes)
}

// Builds a wrapped server instance that will provide a channel of wrapped connections
func buildServer(nsb *NanoBuilder, customHandler func(net.Listener, *NanoBuilder, <-chan bool)) (server *Nan0Server, err error) {
	defer recoverPanic(func(e error) {
		fail("Error occurred while serving %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()
	server = &Nan0Server{
		newConnections:   make(chan NanoServiceWrapper, MaxNanoCache),
		connections:      make([]NanoServiceWrapper, MaxNanoCache),
		listenerShutdown: make(chan bool, 1),
		confirmShutdown:  make(chan bool, 1),
		closed:           false,
		service:          nsb.ns,
	}
	// start a listener
	listener, err := nsb.ns.Start()
	checkError(err)
	if customHandler != nil {
		go customHandler(listener, nsb, server.listenerShutdown)
		return
	}
	// handle shutdown separate from checking for clients
	go func() {
		// check to see if we need to shut everything down
		<-server.listenerShutdown // close all current connections
		server.ResetConnections()
		listener.Close()
		server.confirmShutdown <- true
		return
	}()
	go func() {
		for !server.IsShutdown() {
			// every time we get a new client
			conn, err := listener.Accept()
			if err != nil {
				fail("Listener for %v no longer accepting connections.", server.service.ServiceName)
				server.Shutdown()
			}

			// create a new nan0 connection to the client
			newNano, err := nsb.WrapConnection(conn)
			if err != nil {
				warn("Connection dropped due to %v", err)
			}

			// place the new connection on the channel and in the connections cache
			server.AddConnection(newNano)
		}
		warn("***** Server %v is shutdown *****", server.GetServiceName())
	}()
	info("Server %v started!", server.GetServiceName())
	return
}
