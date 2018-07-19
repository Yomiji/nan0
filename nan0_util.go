package nan0

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"net"
	"time"
	"os"
	"log"
	"errors"
	"io"
)

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
	Warn  = log.New(os.Stderr, "Nan0 [WARN]: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(os.Stderr, "Nan0 [ERROR]: ", log.Ldate|log.Ltime|log.Lshortfile)
	Debug = log.New(os.Stdout, "Nan0 [DEBUG]: ", log.Ldate|log.Ltime|log.Lshortfile)
)

// Wrapper around the Info global log that allows for this api to log to that level correctly
func info(msg string, vars ...interface{}) {
	if Info != nil {
		Info.Printf(msg, vars...)
	}
}

// Wrapper around the Warn global log that allows for this api to log to that level correctly
func warn(msg string, vars ...interface{}) {
	if Warn != nil {
		Warn.Printf(msg, vars...)
	}
}

// Wrapper around the Error global log that allows for this api to log to that level correctly
func fail(msg string, vars ...interface{}) {
	if Error != nil {
		Error.Printf(msg, vars...)
	}
}

// Wrapper around the Debug global log that allows for this api to log to that level correctly
func debug(msg string, vars ...interface{}) {
	if Debug != nil {
		Debug.Printf(msg, vars...)
	}
}

// Conveniently disable all logging for this api
func NoLogging() {
	Info = nil
	Warn = nil
	Error = nil
	Debug = nil
}

func SetLogWriter( w io.Writer ) {
	if Info != nil {
		Info = log.New(w, "Nan0 [INFO]: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	if Warn != nil {
		Warn  = log.New(w, "Nan0 [WARN]: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	if Error != nil {
		Error = log.New(w, "Nan0 [ERROR]: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	if Debug != nil {
		Debug = log.New(w, "Nan0 [DEBUG]: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
}

// The timeout for TCP Writers and server connections
var TCPTimeout = 10 * time.Second


/*******************
	TCP Negotiate
 *******************/

// TCP Preamble, this pre-fixes a unique tcp transmission (protocol buffer message)
var ProtoPreamble = []byte{0x01, 0x02, 0x03, 0xFF, 0x03, 0x02, 0x01}

// The default array width provided with this API, this is the default value of the SizeArrayWidth
const defaultArrayWidth = 2

// The number of bytes that constitute the size of the proto.Message sent/received
// This variable is made visible so that developers can support larger data sizes
var SizeArrayWidth = defaultArrayWidth

// Converts the size header from the byte slice that represents it to an integer type
// NOTE: If you change SizeArrayWidth, you should also change this function
// This function is made visible so that developers can support larger data sizes
var SizeReader = func(bytes []byte) int {
	return int(binary.BigEndian.Uint16(bytes))
}

// Converts the integer representing the size of the following protobuf message to a byte slice
// NOTE: If you change SizeReader, you should also change this function
// This function is made visible so that developers can support larger data sizes
var SizeWriter = func(size int) (bytes []byte) {
	bytes = make([]byte, SizeArrayWidth)
	binary.BigEndian.PutUint16(bytes, uint16(size))
	return bytes
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

// Places the given protocol buffer message in the connection, the connection will receive the following data:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//	2. The size of the following protocol buffer message (defaults to 2 bytes)
// 	3. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func putMessageInConnection(conn net.Conn, pb proto.Message) (err error) {
	defer recoverPanic(func(e error) {
		fail("Message failed to send: %v due to %v", pb, e)
	})()
	//buffer := bufio.NewWriter(conn)
	// write the preamble, size and message
	// marshal the protobuf message
	v, err := proto.Marshal(pb)
	checkError(err)

	protoSize := len(v)
	//prepare all items
	bigBytes := append(ProtoPreamble, SizeWriter(protoSize)...)
	bigBytes = append(bigBytes, v...)

	n, err := conn.Write(bigBytes)
	checkError(err)
	totalSize := len(ProtoPreamble) + SizeArrayWidth + protoSize
	if totalSize != n {
		debug("discrepancy in number of bytes written for message. expected: %v, got: %v", totalSize, n)
		err = errors.New("message size discrepancy while sending")
	} else {
		debug("wrote message to connection with byte size: %v", len(ProtoPreamble) + SizeArrayWidth + n)
	}
	return err
}

// Retrieves the given protocol buffer message from the connection, the connection is expected to send the following:
// 	1. The preamble bytes stored in ProtoPreamble (defaults to 7 bytes)
//	2. The size of the following protocol buffer message (defaults to 2 bytes)
// 	3. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func getMessageFromConnection(conn net.Conn, pb *proto.Message) (err error) {
	defer recoverPanic(func(e error) {
		fail("Failed to receive message due to %v", e)
		err = e
	})()

	// check the preamble
	_,err = isPreambleValid(conn)
	checkError(err)
	// if the preamble is invalid, return the error and do not continue
	//if !p {
	//	return errors.New("preamble was not correctly received for response")
	//}

	// get the size of the next message
	size := readSize(conn)
	// create a byte buffer that will store the whole expected message
	v := make([]byte, size)
	// get the protobuf bytes from the reader
	count, err := conn.Read(v)
	checkError(err)
	debug("Read data %v", v)

	// check the number of bytes received matches the bytes expected
	if count == size {
		debug("Unmarshaling")
		// convert bytes to a protobuf message
		err = proto.Unmarshal(v, *pb)
		checkError(err)
	} else {
		err = errors.New("message size discrepancy while sending")
	}
	return err
}

// Checks the preamble bytes to determine if the expected matches
func isPreambleValid(reader io.Reader) (result bool, err error) {
	defer recoverPanic(func(e error) {
		debug("preamble issue: %v", e)
		result = false
		err = e
	})()
	b := make([]byte, len(ProtoPreamble))
	_, err = reader.Read(b)
	checkError(err)
	debug("checking %v against preamble %v", b, ProtoPreamble)
	for i,v := range b {
		if i < len(ProtoPreamble) && v != ProtoPreamble[i] {
			return false, errors.New("preamble invalid")
		}
	}
	return result, err
}

// Grabs the next two bytes from the reader and figure out the size of the following protobuf message
func readSize(reader io.Reader) int {
	defer recoverPanic(func(e error) {
		warn("issue reading size: %v", e)
	})()
	bytes := make([]byte, SizeArrayWidth)
	_,err := reader.Read(bytes)
	debug("read size array %v", bytes)
	checkError(err)

	return SizeReader(bytes)
}
