// Package crypto provides AES-GCM encryption helpers for WebSocket messages.
//
// Threat model: defense in depth on top of TLS. Each room carries its own
// 32-byte AES-256-GCM key. The server stores plaintext (for persistence,
// AI, conflict resolution) and re-encrypts on broadcast. This is NOT
// end-to-end encryption.
//
// Wire format for an encrypted payload:
//
//	{ "iv": "<base64 12-byte nonce>", "ciphertext": "<base64 ciphertext+tag>" }
//
// The ciphertext field is the standard AES-GCM output (ciphertext concatenated
// with the 16-byte authentication tag). The tag is verified on Open; tampered
// payloads return an error and are rejected upstream.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeySize is the required key length for AES-256-GCM.
const KeySize = 32

// NonceSize is the standard 96-bit GCM nonce size.
const NonceSize = 12

// EncryptedEnvelope is the wire format for an AES-GCM payload exchanged
// over the WebSocket. Both fields are base64 (standard padding).
type EncryptedEnvelope struct {
	IV         string `json:"iv"`
	Ciphertext string `json:"ciphertext"`
}

// GenerateKey returns a fresh 32-byte AES-256 key from crypto/rand.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return key, nil
}

// EncodeKey serialises a key for storage.
func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodeKey reverses EncodeKey and validates length.
func DecodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key length: got %d, want %d", len(key), KeySize)
	}
	return key, nil
}

// Seal encrypts plaintext with key and returns an envelope ready for JSON
// marshalling. A fresh nonce is generated per call.
func Seal(key, plaintext []byte) (*EncryptedEnvelope, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key length: got %d, want %d", len(key), KeySize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	ct := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedEnvelope{
		IV:         base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
	}, nil
}

// Open decrypts an envelope using key. Returns an error if the tag is
// invalid (tampered payload or wrong key).
func Open(key []byte, env *EncryptedEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("nil envelope")
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key length: got %d, want %d", len(key), KeySize)
	}

	nonce, err := base64.StdEncoding.DecodeString(env.IV)
	if err != nil {
		return nil, fmt.Errorf("decode iv: %w", err)
	}
	if len(nonce) != NonceSize {
		return nil, fmt.Errorf("invalid nonce length: got %d, want %d", len(nonce), NonceSize)
	}

	ct, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open (tampered or wrong key): %w", err)
	}

	return pt, nil
}
