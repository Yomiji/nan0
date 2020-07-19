package nan0

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/yomiji/slog"
)

// Decrypts, authenticates and unmarshals a protobuf message using the given encrypt/decrypt key and hmac key
func DecryptProtobuf(rawData []byte, msg proto.Message, hmacSize int, decryptKey *[32]byte, hmacKey *[32]byte) (err error) {
	slog.Debug("Decrypting a byte slice of size %v", len(rawData))
	defer recoverPanic(func(e error) {
		slog.Fail("decryption issue: %v", e)
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
		err = proto.Unmarshal(realData, msg)
		checkError(err)
		slog.Debug("Decrypt completed successfully, result: %v", msg)
	} else {
		// fail out if the message authenticity cannot be verified
		checkError(errors.New("authentication failed"))
	}
	return err
}

// Signs and encrypts a marshalled protobuf message using the given encrypt/decrypt key and hmac key
func EncryptProtobuf(pb proto.Message, typeVal int,  encryptKey *[32]byte, hmacKey *[32]byte) []byte {
	slog.Debug("Encrypting %v", pb)
	defer recoverPanic(func(e error) {
		slog.Fail("decryption issue: %v", e)
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
	slog.Debug("Encrypt complete, result: %v", result)
	return result
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
