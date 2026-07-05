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
//
// Administration surfaces require admin: token management, config
// export/import, and — crucially — settings *mutations* (a write-scoped token
// must not be able to change security settings such as disabling auth, which
// would be a privilege escalation). Reading settings needs only read since the
// API never returns secrets.
func requiredScope(r *http.Request) string {
	path := r.URL.Path
	if strings.Contains(path, "/config") || strings.Contains(path, "/tokens") {
		return ScopeAdmin
	}
	isRead := r.Method == http.MethodGet || r.Method == http.MethodHead
	if strings.Contains(path, "/settings") && !isRead {
		return ScopeAdmin
	}
	if isRead {
		return ScopeRead
	}
	return ScopeWrite
}
