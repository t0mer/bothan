// Package crypto provides AES-256-GCM encryption for secrets at rest, keyed by
// the operator-provisioned instance key (never stored in the database).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ErrNoKey is returned when an operation needs the encryption key but none is set.
var ErrNoKey = errors.New("encryption key is not configured")

// Cipher performs AES-256-GCM sealing/opening with a fixed 32-byte key.
type Cipher struct {
	aead cipher.AEAD
	key  []byte
}

// New parses a 32-byte key (base64, hex, or raw) and builds a Cipher. An empty
// key yields ErrNoKey.
func New(key string) (*Cipher, error) {
	if key == "" {
		return nil, ErrNoKey
	}
	raw, err := parseKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return &Cipher{aead: aead, key: raw}, nil
}

// parseKey decodes a key to exactly 32 bytes, trying base64 then hex then raw.
func parseKey(key string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(key); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := hex.DecodeString(key); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(key) == 32 {
		return []byte(key), nil
	}
	return nil, fmt.Errorf("encryption key must be 32 bytes (base64, hex, or raw); got %d chars", len(key))
}

// Encrypt seals plaintext, returning nonce||ciphertext.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens a nonce||ciphertext blob produced by Encrypt.
func (c *Cipher) Decrypt(blob []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	plaintext, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	return plaintext, nil
}

// Fingerprint returns a non-reversible identifier of the key, used to check
// that two instances share the same key before importing encrypted data.
func (c *Cipher) Fingerprint() string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte("bothan-key-id"))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
