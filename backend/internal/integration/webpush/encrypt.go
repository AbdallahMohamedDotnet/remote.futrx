package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// recordSize is the aes128gcm record length advertised in the body header.
	// Everything we send fits in a single record.
	recordSize = 4096

	saltLength      = 16
	authLength      = 16
	publicKeyLength = 65
	// headerLength is salt || record size || key id length || key id.
	headerLength = saltLength + 4 + 1 + publicKeyLength
	// maxPlaintext leaves room for the padding delimiter and the GCM tag.
	maxPlaintext = recordSize - headerLength - 1 - 16
)

// ErrPayloadTooLarge means the notification would not fit in one aes128gcm
// record. Push services reject oversized bodies outright, so this is caught
// before the request leaves the process.
var ErrPayloadTooLarge = errors.New("push payload is too large")

// encrypt seals plaintext for a subscription following RFC 8291, returning the
// complete aes128gcm body: header, then one ciphertext record.
//
//	uaPublic   the subscription's p256dh key (uncompressed P-256 point)
//	authSecret the subscription's 16-byte auth secret
func encrypt(plaintext, uaPublic, authSecret []byte, entropy io.Reader) ([]byte, error) {
	if len(plaintext) > maxPlaintext {
		return nil, fmt.Errorf("%w: %d bytes exceeds %d", ErrPayloadTooLarge, len(plaintext), maxPlaintext)
	}
	if len(authSecret) != authLength {
		return nil, fmt.Errorf("subscription auth secret must be %d bytes, got %d", authLength, len(authSecret))
	}
	userAgentKey, err := publicKeyFromBytes(uaPublic)
	if err != nil {
		return nil, err
	}
	if entropy == nil {
		entropy = rand.Reader
	}

	serverKey, err := ecdh.P256().GenerateKey(entropy)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	serverPublic := serverKey.PublicKey().Bytes()

	shared, err := serverKey.ECDH(userAgentKey)
	if err != nil {
		return nil, fmt.Errorf("derive shared secret: %w", err)
	}

	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(entropy, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key, nonce, err := deriveRecordKey(shared, authSecret, salt, uaPublic, serverPublic)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}

	// RFC 8188 pads each record with a delimiter octet: 0x02 marks the last
	// record, which is the only one we ever write.
	record := make([]byte, 0, len(plaintext)+1)
	record = append(record, plaintext...)
	record = append(record, 0x02)

	body := make([]byte, 0, headerLength+len(record)+aead.Overhead())
	body = append(body, salt...)
	body = binary.BigEndian.AppendUint32(body, recordSize)
	body = append(body, byte(len(serverPublic)))
	body = append(body, serverPublic...)
	return aead.Seal(body, nonce, record, nil), nil
}

// deriveRecordKey runs the RFC 8291 key schedule: the ECDH secret and the
// subscription's auth secret produce the input keying material, which the
// per-message salt then expands into the content encryption key and nonce.
func deriveRecordKey(shared, authSecret, salt, uaPublic, serverPublic []byte) (key, nonce []byte, err error) {
	keyInfo := make([]byte, 0, len("WebPush: info")+1+len(uaPublic)+len(serverPublic))
	keyInfo = append(keyInfo, "WebPush: info"...)
	keyInfo = append(keyInfo, 0x00)
	keyInfo = append(keyInfo, uaPublic...)
	keyInfo = append(keyInfo, serverPublic...)

	ikm, err := hkdf.Key(sha256.New, shared, authSecret, string(keyInfo), 32)
	if err != nil {
		return nil, nil, fmt.Errorf("derive input keying material: %w", err)
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, nil, fmt.Errorf("extract content key: %w", err)
	}
	key, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, nil, fmt.Errorf("expand content key: %w", err)
	}
	nonce, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, nil, fmt.Errorf("expand nonce: %w", err)
	}
	return key, nonce, nil
}
