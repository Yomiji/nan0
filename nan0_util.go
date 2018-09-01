package nan0

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"net"
		"os"
	"log"
	"errors"
	"io"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/hmac"
	"crypto/sha512"
	"reflect"
	"time"
	"encoding/base64"
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
	Info  = log.New(os.Stdout, "Nan0 [INFO]: ", log.Ldate|log.Ltime)
	Warn  = log.New(os.Stderr, "Nan0 [WARN]: ", log.Ldate|log.Ltime)
	Error = log.New(os.Stderr, "Nan0 [ERROR]: ", log.Ldate|log.Ltime)
	Debug *log.Logger = nil
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
		Info = log.New(w, "Nan0 [INFO]: ", log.Ldate|log.Ltime)
	}
	if Warn != nil {
		Warn  = log.New(w, "Nan0 [WARN]: ", log.Ldate|log.Ltime)
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
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func putMessageInConnection(conn net.Conn, pb proto.Message, inverseMap map[string]int, encryptKey *[32]byte, hmacKey *[32]byte) (err error) {
	defer recoverPanic(func(e error) {
		fail("Message failed to send: %v due to %v", pb, e)
		err = e
	})()

	// figure out if the type of the message is in our list
	typeString := reflect.TypeOf(pb).String()
	typeVal, ok := inverseMap[typeString]
	if !ok {
		checkError(errors.New("type value for message not present"))
	}

	// if type cannot be found, fail and keep going
	encrypted := encryptKey != nil && hmacKey != nil

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
	debug("Writing to connection")
	n, err := conn.Write(bigBytes)
	checkError(err)

	// check the full buffer was written
	totalSize := len(bigBytes)
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
//  2. The protobuf type identifier (4 bytes)
//	3. The size of the following protocol buffer message (defaults to 4 bytes)
// 	4. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func getMessageFromConnection(conn net.Conn, identMap map[int]proto.Message, decryptKey *[32]byte, hmacKey *[32]byte) (msg proto.Message, err error) {
	defer recoverPanic(func(e error) {
		fail("Failed to receive message due to %v", e)
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
	messageType:= readSize(conn)
	msg = proto.Clone(identMap[messageType])
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
	debug("Read data %v", v)

	// check the number of bytes received matches the bytes expected
	if count != size {
		checkError(errors.New("message size discrepancy while sending"))
	}
	if encrypted {
		err := DecryptProtobuf(v, &msg, hmacSize, decryptKey, hmacKey)
		checkError(err)
	} else {
		err = proto.Unmarshal(v, msg)
		checkError(err)
	}
	return msg, err
}

// Decrypts, authenticates and unmarshals a protobuf message using the given encrypt/decrypt key and hmac key
func DecryptProtobuf(rawData []byte, msg *proto.Message, hmacSize int, decryptKey *[32]byte, hmacKey *[32]byte) (err error) {
	debug("Decrypting a byte slice of size %v", len(rawData))
	defer recoverPanic(func(e error) {
		fail("decryption issue: %v", e)
		err = e
	})()

	// decrypt message
	decryptedBytes, err := Decrypt(rawData, decryptKey)
	checkError(err)

	// split the hmac signature from the real data based hmacSize
	mac := decryptedBytes[:hmacSize]
	realData := decryptedBytes[hmacSize:]

	// check the hmac signature to ensure authenticity
	if CheckHMAC(realData,mac,hmacKey) {
		// unmarshal the bytes, placing result into msg
		err = proto.Unmarshal(realData, *msg)
		checkError(err)
		debug("Decrypt completed successfully, result: %v", msg)
	} else {
		// fail out if the message authenticity cannot be verified
		fail("Couldn't decrypt message, authentication failed")
		checkError(errors.New("authentication failed"))
	}
	return err
}

// Signs and encrypts a marshalled protobuf message using the given encrypt/decrypt key and hmac key
func EncryptProtobuf(pb proto.Message, typeVal int,  encryptKey *[32]byte, hmacKey *[32]byte) []byte {
	debug("Encrypting %v", pb)
	defer recoverPanic(func(e error) {
		fail("decryption issue: %v", e)
	})()
	// marshall the message, turning it into bytes
	rawData, err := proto.Marshal(pb)
	checkError(err)

	// sign the data
	mac := GenerateHMAC(rawData, hmacKey)
	macSize := len(mac)
	data := append(mac, rawData...)

	// encrypt the data
	encryptedMsg, err := Encrypt(data, encryptKey)
	encryptedMsgSize := len(encryptedMsg)
	checkError(err)

	// build the byte slice that will be the TCP packet sent on the tcp stream
	result := append(ProtoPreamble, SizeWriter(typeVal)...)
	result = append(result, SizeWriter(macSize)...)
	result = append(result, SizeWriter(encryptedMsgSize)...)
	result = append(result, encryptedMsg...)
	debug("Encrypt complete, result: %v", result)
	return result
}


// Checks the preamble bytes to determine if the expected matches
func isPreambleValid(reader io.Reader) (err error) {
	defer recoverPanic(func(e error) {
		debug("preamble issue: %v", e)
		err = e
	})()
	b := make([]byte, len(ProtoPreamble))
	_, err = reader.Read(b)
	if err == io.EOF {
		return io.EOF
	}
	checkError(err)
	debug("checking %v against preamble %v", b, ProtoPreamble)
	for i,v := range b {
		if i < len(ProtoPreamble) && v != ProtoPreamble[i] {
			return errors.New("preamble invalid")
		}
	}
	return  err
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

// From cryptopasta (https://github.com/gtank/cryptopasta)
//
// NewEncryptionKey generates a random 256-bit key for Encrypt() and
// Decrypt(). It panics if the source of randomness fails.
func NewEncryptionKey() *[32]byte {
	key := [32]byte{}
	_, err := io.ReadFull(rand.Reader, key[:])
	if err != nil {
		panic(err)
	}
	return &key
}

// From cryptopasta (https://github.com/gtank/cryptopasta)
//
// Encrypt encrypts data using 256-bit AES-GCM.  This both hides the content of
// the data and provides a check that it hasn't been altered. Output takes the
// form nonce|ciphertext|tag where '|' indicates concatenation.
func Encrypt(plaintext []byte, key *[32]byte) (ciphertext []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// From cryptopasta (https://github.com/gtank/cryptopasta)
//
// Decrypt decrypts data using 256-bit AES-GCM.  This both hides the content of
// the data and provides a check that it hasn't been altered. Expects input
// form nonce|ciphertext|tag where '|' indicates concatenation.
func Decrypt(ciphertext []byte, key *[32]byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("malformed ciphertext")
	}

	return gcm.Open(nil,
		ciphertext[:gcm.NonceSize()],
		ciphertext[gcm.NonceSize():],
		nil,
	)
}

// From cryptopasta (https://github.com/gtank/cryptopasta)
//
// NewHMACKey generates a random 256-bit secret key for HMAC use.
// Because key generation is critical, it panics if the source of randomness fails.
func NewHMACKey() *[32]byte {

	key := &[32]byte{}

	_, err := io.ReadFull(rand.Reader, key[:])

	if err != nil {

		panic(err)

	}

	return key
}

// From cryptopasta (https://github.com/gtank/cryptopasta)
//
// GenerateHMAC produces a symmetric signature using a shared secret key.
func GenerateHMAC(data []byte, key *[32]byte) []byte {

	h := hmac.New(sha512.New512_256, key[:])

	h.Write(data)

	return h.Sum(nil)
}

// From cryptopasta (https://github.com/gtank/cryptopasta)
//
// CheckHMAC securely checks the supplied MAC against a message using the shared secret key.
func CheckHMAC(data, suppliedMAC []byte, key *[32]byte) bool {

	expectedMAC := GenerateHMAC(data, key)

	return hmac.Equal(expectedMAC, suppliedMAC)
}

// A simple function to make the keys generated a sharable string
func ShareKeys() (encKeyShare, authKeyShare string) {
	encKeyBytes := NewEncryptionKey()
	authKeyBytes := NewHMACKey()

	encKeyShare = base64.StdEncoding.EncodeToString(encKeyBytes[:])
	authKeyShare = base64.StdEncoding.EncodeToString(authKeyBytes[:])

	return
}

// The nan0 functions require a specific key type and width, this is a way to make
// that conversion from strings to the required type.
func KeysToNan0Bytes(encKeyShare, authKeyShare string) (encKey, authKey *[32]byte) {
	encKeyBytes, _ := base64.StdEncoding.DecodeString(encKeyShare)
	authkeyBytes, _ := base64.StdEncoding.DecodeString(authKeyShare)

	encKey = &[32]byte{}
	authKey = &[32]byte{}
	copy(encKey[:], encKeyBytes)
	copy(authKey[:], authkeyBytes)

	return
}


// Builds a wrapped server instance that will provide a channel of wrapped connections
func buildServer(nsb *SecureNanoBuilder, customHandler func(net.Listener, *SecureNanoBuilder, <-chan bool)) (server *Nan0Server, err error) {
	defer recoverPanic(func(e error) {
		fail("Error occurred while serving %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()
	server = &Nan0Server{
		newConnections:make(chan NanoServiceWrapper, MaxNanoCache),
		connections: make([]NanoServiceWrapper, MaxNanoCache),
		listenerShutdown: make(chan bool, 1),
		confirmShutdown: make(chan bool, 1),
		closed: false,
		service: nsb.ns,
	}
	// start a listener
	listener,err := nsb.ns.Start()
	checkError(err)
	if customHandler != nil {
		go customHandler(listener, nsb, server.listenerShutdown)
		return
	}
	// handle shutdown separate from checking for clients
	go func() {
		for ; ; {
			// check to see if we need to shut everything down
			select {
			case <-server.listenerShutdown:// close all current connections
				server.ResetConnections()
				listener.Close()
				server.confirmShutdown <- true
				return
			default:
			}
		}
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
