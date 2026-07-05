package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/t0mer/bothan/internal/store"
)

// CookieName is the session cookie name.
const CookieName = "bothan_session"

// ErrInvalidCredentials is returned by Login on a bad username/password.
var ErrInvalidCredentials = errors.New("invalid username or password")

// Service ties authentication settings, storage, and sessions together.
type Service struct {
	users    *store.UserRepo
	tokens   *store.TokenRepo
	sessions *SessionManager

	enabled        func() bool
	protectMetrics func() bool
}

// NewService builds an auth service. enabled/protectMetrics read live settings.
func NewService(st *store.Store, sessionSecret string, ttl time.Duration, enabled, protectMetrics func() bool) *Service {
	return &Service{
		users:          st.Users(),
		tokens:         st.Tokens(),
		sessions:       NewSessionManager(sessionSecret, ttl),
		enabled:        enabled,
		protectMetrics: protectMetrics,
	}
}

// Enabled reports whether authentication is active.
func (s *Service) Enabled() bool { return s.enabled() }

// ProtectMetrics reports whether /metrics requires auth.
func (s *Service) ProtectMetrics() bool { return s.protectMetrics() }

// Login verifies credentials and returns a signed session token.
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if !VerifyPassword(password, u.PasswordHash) {
		return "", ErrInvalidCredentials
	}
	return s.sessions.Issue(username, time.Now()), nil
}

// SessionCookie builds the session cookie for a token value.
func (s *Service) SessionCookie(value string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.sessions.TTL().Seconds()),
	}
}

// ClearCookie builds a cookie that clears the session.
func (s *Service) ClearCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	}
}

// Principal is an authenticated caller.
type Principal struct {
	Kind     string // "session" | "token"
	Username string
	Scopes   string
}

// HasScope reports whether the principal is allowed the required scope. Session
// (interactive admin) logins have full access.
func (p *Principal) HasScope(required string) bool {
	if p.Kind == "session" {
		return true
	}
	return HasScope(p.Scopes, required)
}

// Authenticate resolves the caller from a session cookie or bearer token, or
// returns nil if unauthenticated.
func (s *Service) Authenticate(r *http.Request) *Principal {
	if c, err := r.Cookie(CookieName); err == nil {
		if username, ok := s.sessions.Verify(c.Value, time.Now()); ok {
			return &Principal{Kind: "session", Username: username}
		}
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		plaintext := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if t, err := s.tokens.GetByHash(r.Context(), HashToken(plaintext)); err == nil {
			_ = s.tokens.Touch(r.Context(), t.ID)
			return &Principal{Kind: "token", Scopes: t.Scopes}
		}
	}
	return nil
}
