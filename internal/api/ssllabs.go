package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/settings"
	"github.com/t0mer/bothan/internal/ssllabs"
)

// SSLLabsClient is the SSL Labs behaviour the handler needs.
type SSLLabsClient interface {
	Info(ctx context.Context) (*ssllabs.Info, error)
	Register(ctx context.Context, req ssllabs.RegisterRequest) error
}

// SSLLabsClientFactory builds a client from the current settings.
type SSLLabsClientFactory func(*settings.Settings) SSLLabsClient

// SSLLabs holds the SSL Labs info/registration handlers.
type SSLLabs struct {
	settings SettingsService
	factory  SSLLabsClientFactory
}

// NewSSLLabs builds the SSL Labs handlers.
func NewSSLLabs(svc SettingsService, factory SSLLabsClientFactory) *SSLLabs {
	return &SSLLabs{settings: svc, factory: factory}
}

// Routes mounts the SSL Labs endpoints onto r.
func (h *SSLLabs) Routes(r chi.Router) {
	r.Get("/info", h.info)
	r.Post("/register", h.register)
}

type infoResponse struct {
	APIVersion         string   `json:"api_version"`
	Email              string   `json:"email"`
	Registered         bool     `json:"registered"`
	EngineVersion      string   `json:"engine_version"`
	CriteriaVersion    string   `json:"criteria_version"`
	MaxAssessments     int      `json:"max_assessments"`
	CurrentAssessments int      `json:"current_assessments"`
	CoolOffMillis      int64    `json:"cool_off_millis"`
	Messages           []string `json:"messages"`
}

func (h *SSLLabs) info(w http.ResponseWriter, r *http.Request) {
	cur := h.settings.Current()
	resp := infoResponse{
		APIVersion: cur.SSLLabs.APIVersion,
		Email:      cur.SSLLabs.Email,
		Registered: cur.SSLLabs.APIVersion == "v3" || cur.SSLLabs.Email != "",
	}

	client := h.factory(cur)
	info, err := client.Info(r.Context())
	if err != nil {
		// Capacity is best-effort; still return configuration status.
		WriteJSON(w, http.StatusOK, resp)
		return
	}
	resp.EngineVersion = info.EngineVersion
	resp.CriteriaVersion = info.CriteriaVersion
	resp.MaxAssessments = info.MaxAssessments
	resp.CurrentAssessments = info.CurrentAssessments
	resp.CoolOffMillis = info.NewAssessmentCoolOff
	resp.Messages = info.Messages
	WriteJSON(w, http.StatusOK, resp)
}

type registerRequest struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
}

func (h *SSLLabs) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	if req.Name == "" || req.Email == "" {
		WriteError(w, http.StatusBadRequest, "invalid", "name and email are required")
		return
	}

	first, last := splitName(req.Name)
	client := h.factory(h.settings.Current())
	if err := client.Register(r.Context(), ssllabs.RegisterRequest{
		FirstName: first, LastName: last, Email: req.Email, Organization: req.Organization,
	}); err != nil {
		WriteError(w, http.StatusBadGateway, "registration_failed", "SSL Labs registration failed: "+err.Error())
		return
	}

	// Persist the registered email so v4 assessments can proceed.
	if err := h.settings.Update(r.Context(), map[string]string{settings.KeySSLLabsEmail: req.Email}); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "registered but failed to persist email: "+err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"registered": true, "email": req.Email})
}

func splitName(name string) (first, last string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}
