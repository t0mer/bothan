package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Scopes.
const (
	ScopeRead  = "read"
	ScopeWrite = "write"
	ScopeAdmin = "admin"
)

// GenerateToken returns a new random API token (shown once) and its SHA-256 hash
// (stored). The plaintext is prefixed "bth_" for recognisability.
func GenerateToken() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", "", fmt.Errorf("generating token: %w", err)
	}
	plaintext = "bth_" + base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashToken(plaintext), nil
}

// HashToken returns the hex SHA-256 hash of a token, for storage and lookup.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ValidScope reports whether s is a recognised scope.
func ValidScope(s string) bool {
	switch s {
	case ScopeRead, ScopeWrite, ScopeAdmin:
		return true
	}
	return false
}

// HasScope reports whether the csv scope list grants the required scope. admin
// implies write implies read.
func HasScope(scopesCSV, required string) bool {
	granted := map[string]bool{}
	for _, s := range strings.Split(scopesCSV, ",") {
		granted[strings.TrimSpace(s)] = true
	}
	if granted[ScopeAdmin] {
		return true
	}
	switch required {
	case ScopeRead:
		return granted[ScopeRead] || granted[ScopeWrite]
	case ScopeWrite:
		return granted[ScopeWrite]
	case ScopeAdmin:
		return granted[ScopeAdmin]
	}
	return false
}
