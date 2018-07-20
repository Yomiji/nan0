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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/hmac"
	"crypto/sha512"
)

/*************
	Logging
************/

//Loggers provided:
//  Level | Output | Format
// 	Info: Standard Output - 'Nan0 [INFO] %date% %time% %filename:line%'
// 	Warn: Standard Error - 'Nan0 [DEBUG] %date% %time% %filename:line%'
// 	Error: Standard Error - 'Nan0 [ERROR] %date% %time% %filename:line%'
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
func putMessageInConnection(conn net.Conn, pb proto.Message, encryptKey *[32]byte, hmacKey *[32]byte) (err error) {
	defer recoverPanic(func(e error) {
		fail("Message failed to send: %v due to %v", pb, e)
	})()

	encrypted := encryptKey != nil && hmacKey != nil

	var bigBytes []byte
	if encrypted {
		bigBytes = EncryptProtobuf(pb, encryptKey, hmacKey)
	} else {
		// marshal the protobuf message
		v, err := proto.Marshal(pb)
		checkError(err)
		protoSize := len(v)
		//prepare all items
		bigBytes = append(ProtoPreamble, SizeWriter(protoSize)...)
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
//	2. The size of the following protocol buffer message (defaults to 2 bytes)
// 	3. The protocol buffer message (slice of bytes the size of the result of #2 as integer)
func getMessageFromConnection(conn net.Conn, pb *proto.Message, decryptKey *[32]byte, hmacKey *[32]byte) (err error) {
	defer recoverPanic(func(e error) {
		fail("Failed to receive message due to %v", e)
		err = e
	})()

	// determine if this is an encrypted msg
	encrypted := decryptKey != nil && hmacKey != nil

	// check the preamble
	err = isPreambleValid(conn)
	checkError(err)

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
		err := DecryptProtobuf(v, pb, hmacSize, decryptKey, hmacKey)
		checkError(err)
	} else {
		err = proto.Unmarshal(v, *pb)
		checkError(err)
	}
	return err
}

func DecryptProtobuf(rawData []byte, msg *proto.Message, hmacSize int, decryptKey *[32]byte, hmacKey *[32]byte) error {
	decryptedBytes, err := Decrypt(rawData, decryptKey)
	checkError(err)
	mac := decryptedBytes[:hmacSize]
	realData := decryptedBytes[hmacSize:]
	if CheckHMAC(realData,mac,hmacKey) {
		err = proto.Unmarshal(realData, *msg)
		checkError(err)
	} else {
		fail("Couldn't decrypt message, authentication failed")
		checkError(errors.New("authentication failed"))
	}
	return nil
}

func EncryptProtobuf(pb proto.Message, encryptKey *[32]byte, hmacKey *[32]byte) []byte {
	debug("Encrypting, Signing and Marshaling")
	// marshall the message
	rawData, err := proto.Marshal(pb)
	checkError(err)

	mac := GenerateHMAC(rawData, hmacKey)
	macSize := len(mac)
	data := append(mac, rawData...)

	encryptedMsg, err := Encrypt(data, encryptKey)
	encryptedMsgSize := len(encryptedMsg)
	checkError(err)
	result := append(ProtoPreamble, SizeWriter(macSize)...)
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



// GenerateHMAC produces a symmetric signature using a shared secret key.
func GenerateHMAC(data []byte, key *[32]byte) []byte {

	h := hmac.New(sha512.New512_256, key[:])

	h.Write(data)

	return h.Sum(nil)



}



// CheckHMAC securely checks the supplied MAC against a message using the shared secret key.
func CheckHMAC(data, suppliedMAC []byte, key *[32]byte) bool {

	expectedMAC := GenerateHMAC(data, key)

	return hmac.Equal(expectedMAC, suppliedMAC)

}