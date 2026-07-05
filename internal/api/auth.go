package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/auth"
	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

// TokenRepo is the store behaviour the token handlers need.
type TokenRepo interface {
	Create(ctx context.Context, t *model.APIToken) error
	List(ctx context.Context) ([]model.APIToken, error)
	Delete(ctx context.Context, id int64) error
}

// Auth holds authentication handlers (login/logout/me and tokens).
type Auth struct {
	svc    *auth.Service
	tokens TokenRepo
}

// NewAuth builds the auth handlers.
func NewAuth(svc *auth.Service, tokens TokenRepo) *Auth {
	return &Auth{svc: svc, tokens: tokens}
}

// Routes mounts the auth session endpoints (unprotected).
func (h *Auth) Routes(r chi.Router) {
	r.Post("/login", h.login)
	r.Post("/logout", h.logout)
	r.Get("/me", h.me)
}

// TokenRoutes mounts the token-management endpoints (admin-protected).
func (h *Auth) TokenRoutes(r chi.Router) {
	r.Get("/", h.listTokens)
	r.Post("/", h.createToken)
	r.Delete("/{id}", h.deleteToken)
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Auth) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body")
		return
	}
	token, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}
	http.SetCookie(w, h.svc.SessionCookie(token, secureRequest(r)))
	WriteJSON(w, http.StatusOK, map[string]string{"username": req.Username})
}

func (h *Auth) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.svc.ClearCookie(secureRequest(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Auth) me(w http.ResponseWriter, r *http.Request) {
	if !h.svc.Enabled() {
		WriteJSON(w, http.StatusOK, map[string]any{"auth_enabled": false})
		return
	}
	p := h.svc.Authenticate(r)
	if p == nil {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"auth_enabled": true, "kind": p.Kind, "username": p.Username, "scopes": p.Scopes,
	})
}

// --- token management ---

type createTokenRequest struct {
	Name      string `json:"name"`
	Scopes    string `json:"scopes"`
	ExpiresIn string `json:"expires_in"` // optional Go duration, e.g. "720h"
}

func (h *Auth) createToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "invalid", "name is required")
		return
	}
	for _, s := range strings.Split(req.Scopes, ",") {
		if !auth.ValidScope(strings.TrimSpace(s)) {
			WriteError(w, http.StatusBadRequest, "invalid", "scopes must be a csv of read,write,admin")
			return
		}
	}

	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to generate token")
		return
	}
	tok := &model.APIToken{Name: req.Name, TokenHash: hash, Scopes: req.Scopes}
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil || d <= 0 {
			WriteError(w, http.StatusBadRequest, "invalid", "expires_in must be a positive duration")
			return
		}
		exp := time.Now().Add(d).UTC()
		tok.ExpiresAt = &exp
	}
	if err := h.tokens.Create(r.Context(), tok); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to create token")
		return
	}
	// The plaintext is returned exactly once.
	WriteJSON(w, http.StatusCreated, map[string]any{"token": tok, "plaintext": plaintext})
}

func (h *Auth) listTokens(w http.ResponseWriter, r *http.Request) {
	list, err := h.tokens.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list tokens")
		return
	}
	WriteJSON(w, http.StatusOK, list)
}

func (h *Auth) deleteToken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.tokens.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "token not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal", "failed to delete token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
