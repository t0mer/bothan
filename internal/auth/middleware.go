package auth

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const principalKey ctxKey = 0

// PrincipalFrom returns the authenticated principal on the context, if any.
func PrincipalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey).(*Principal)
	return p
}

// Protect wraps the API routes: when auth is enabled it requires a valid
// session or bearer token with sufficient scope; otherwise it passes through.
// The required scope is derived from the request: reads need "read", mutations
// need "write", and token/config administration needs "admin".
func (s *Service) Protect(unauthorized func(w http.ResponseWriter, r *http.Request, code, msg string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.enabled() {
				next.ServeHTTP(w, r)
				return
			}
			p := s.Authenticate(r)
			if p == nil {
				unauthorized(w, r, "unauthorized", "authentication required")
				return
			}
			if !p.HasScope(requiredScope(r)) {
				unauthorized(w, r, "forbidden", "insufficient scope")
				return
			}
			ctx := context.WithValue(r.Context(), principalKey, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requiredScope maps a request to the scope it needs.
func requiredScope(r *http.Request) string {
	if strings.Contains(r.URL.Path, "/config") || strings.Contains(r.URL.Path, "/tokens") {
		return ScopeAdmin
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		return ScopeRead
	default:
		return ScopeWrite
	}
}
