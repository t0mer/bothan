package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SessionManager issues and verifies stateless, HMAC-signed session tokens.
type SessionManager struct {
	secret []byte
	ttl    time.Duration
}

// NewSessionManager builds a session manager with the given signing secret.
func NewSessionManager(secret string, ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SessionManager{secret: []byte(secret), ttl: ttl}
}

// TTL returns the session lifetime.
func (m *SessionManager) TTL() time.Duration { return m.ttl }

// Issue returns a signed token for the username, valid for the manager's TTL.
func (m *SessionManager) Issue(username string, now time.Time) string {
	payload := fmt.Sprintf("%s|%d", username, now.Add(m.ttl).Unix())
	enc := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return enc + "." + m.sign(enc)
}

// Verify checks a token's signature and expiry and returns the username.
func (m *SessionManager) Verify(token string, now time.Time) (string, bool) {
	enc, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(sig), []byte(m.sign(enc))) != 1 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", false
	}
	username, expStr, ok := strings.Cut(string(raw), "|")
	if !ok {
		return "", false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || now.Unix() >= exp {
		return "", false
	}
	return username, true
}

func (m *SessionManager) sign(msg string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
