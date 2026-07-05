package crypto

import (
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// KDFParams describes an argon2id key-derivation configuration. It is stored in
// a passphrase-mode export bundle so import can re-derive the key.
type KDFParams struct {
	Algo    string `json:"algo"`    // "argon2id"
	Salt    []byte `json:"salt"`    // random per export
	Time    uint32 `json:"time"`    // iterations
	Memory  uint32 `json:"memory"`  // KiB
	Threads uint8  `json:"threads"` // parallelism
}

// DefaultKDFParams returns sensible argon2id parameters with a fresh salt.
func DefaultKDFParams() (KDFParams, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return KDFParams{}, fmt.Errorf("generating salt: %w", err)
	}
	return KDFParams{Algo: "argon2id", Salt: salt, Time: 3, Memory: 64 * 1024, Threads: 4}, nil
}

// NewFromPassphrase derives a 32-byte key from a passphrase via argon2id and
// returns a Cipher.
func NewFromPassphrase(passphrase string, p KDFParams) (*Cipher, error) {
	if passphrase == "" {
		return nil, fmt.Errorf("passphrase is required")
	}
	if p.Algo != "argon2id" {
		return nil, fmt.Errorf("unsupported KDF %q", p.Algo)
	}
	if len(p.Salt) == 0 || p.Time == 0 || p.Memory == 0 || p.Threads == 0 {
		return nil, fmt.Errorf("invalid KDF parameters")
	}
	key := argon2.IDKey([]byte(passphrase), p.Salt, p.Time, p.Memory, p.Threads, 32)
	return newFromRawKey(key)
}
