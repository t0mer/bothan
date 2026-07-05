package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func req(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

func TestRequiredScope(t *testing.T) {
	cases := []struct {
		method, path string
		want         string
	}{
		{"GET", "/api/v1/hosts", ScopeRead},
		{"POST", "/api/v1/hosts", ScopeWrite},
		{"DELETE", "/api/v1/hosts/1", ScopeWrite},
		{"GET", "/api/v1/settings", ScopeRead},
		// Settings mutations must require admin — a write token disabling auth
		// would be a privilege escalation.
		{"PUT", "/api/v1/settings", ScopeAdmin},
		{"GET", "/api/v1/config/export", ScopeAdmin},
		{"POST", "/api/v1/config/import", ScopeAdmin},
		{"GET", "/api/v1/tokens", ScopeAdmin},
		{"POST", "/api/v1/tokens", ScopeAdmin},
	}
	for _, c := range cases {
		if got := requiredScope(req(c.method, c.path)); got != c.want {
			t.Errorf("requiredScope(%s %s) = %q, want %q", c.method, c.path, got, c.want)
		}
	}
}

func TestWriteTokenCannotEscalateViaSettings(t *testing.T) {
	// A write-scoped principal must not satisfy the admin requirement that
	// guards settings mutations.
	writeToken := &Principal{Kind: "token", Scopes: "read,write"}
	if writeToken.HasScope(requiredScope(req("PUT", "/api/v1/settings"))) {
		t.Fatal("write token must not be allowed to mutate settings")
	}
	// A session (interactive admin) is allowed.
	session := &Principal{Kind: "session"}
	if !session.HasScope(requiredScope(req("PUT", "/api/v1/settings"))) {
		t.Fatal("session should be allowed to mutate settings")
	}
	// An admin token is allowed.
	adminToken := &Principal{Kind: "token", Scopes: "admin"}
	if !adminToken.HasScope(requiredScope(req("PUT", "/api/v1/settings"))) {
		t.Fatal("admin token should be allowed to mutate settings")
	}
}
