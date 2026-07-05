package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
)

func randKey(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

func TestNew_KeyFormats(t *testing.T) {
	raw := randKey(t)
	for name, key := range map[string]string{
		"base64": base64.StdEncoding.EncodeToString(raw),
		"hex":    hex.EncodeToString(raw),
		"raw":    string(raw),
	} {
		if _, err := New(key); err != nil {
			t.Errorf("%s key rejected: %v", name, err)
		}
	}
	if _, err := New(""); !errors.Is(err, ErrNoKey) {
		t.Errorf("empty key err = %v, want ErrNoKey", err)
	}
	if _, err := New("too-short"); err == nil {
		t.Errorf("short key should be rejected")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := New(hex.EncodeToString(randKey(t)))
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte(`{"url":"slack://token@channel"}`)
	blob, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(blob, plaintext) {
		t.Error("ciphertext contains plaintext")
	}
	got, err := c.Decrypt(blob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch: %s", got)
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	c1, _ := New(hex.EncodeToString(randKey(t)))
	c2, _ := New(hex.EncodeToString(randKey(t)))
	blob, _ := c1.Encrypt([]byte("secret"))
	if _, err := c2.Decrypt(blob); err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestFingerprint_StableAndKeyed(t *testing.T) {
	k := hex.EncodeToString(randKey(t))
	c1, _ := New(k)
	c2, _ := New(k)
	if c1.Fingerprint() != c2.Fingerprint() {
		t.Error("fingerprint should be stable for the same key")
	}
	c3, _ := New(hex.EncodeToString(randKey(t)))
	if c1.Fingerprint() == c3.Fingerprint() {
		t.Error("different keys should have different fingerprints")
	}
}
