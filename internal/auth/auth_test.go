package auth

import (
	"testing"
	"time"
)

func TestPassword_HashAndVerify(t *testing.T) {
	h, err := HashPassword("s3cret!")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("s3cret!", h) {
		t.Error("correct password should verify")
	}
	if VerifyPassword("wrong", h) {
		t.Error("wrong password should not verify")
	}
	if VerifyPassword("s3cret!", "garbage") {
		t.Error("garbage hash should not verify")
	}
}

func TestSession_IssueVerify(t *testing.T) {
	m := NewSessionManager("signing-secret", time.Hour)
	now := time.Unix(1_000_000, 0)
	tok := m.Issue("admin", now)

	u, ok := m.Verify(tok, now)
	if !ok || u != "admin" {
		t.Errorf("verify = %q, %v; want admin, true", u, ok)
	}
	// Expired.
	if _, ok := m.Verify(tok, now.Add(2*time.Hour)); ok {
		t.Error("expired token should not verify")
	}
	// Tampered signature.
	if _, ok := m.Verify(tok+"x", now); ok {
		t.Error("tampered token should not verify")
	}
	// Wrong secret.
	other := NewSessionManager("different", time.Hour)
	if _, ok := other.Verify(tok, now); ok {
		t.Error("token from a different secret should not verify")
	}
}

func TestToken_GenerateAndHash(t *testing.T) {
	pt, hash, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if HashToken(pt) != hash {
		t.Error("hash mismatch")
	}
	if HashToken("other") == hash {
		t.Error("different tokens should hash differently")
	}
}

func TestHasScope(t *testing.T) {
	cases := []struct {
		scopes, required string
		want             bool
	}{
		{"read", "read", true},
		{"read", "write", false},
		{"write", "read", true},
		{"write", "write", true},
		{"write", "admin", false},
		{"admin", "admin", true},
		{"admin", "write", true},
		{"read,write", "write", true},
	}
	for _, c := range cases {
		if got := HasScope(c.scopes, c.required); got != c.want {
			t.Errorf("HasScope(%q, %q) = %v, want %v", c.scopes, c.required, got, c.want)
		}
	}
}
