package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/crypto"
	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

// ChannelRepo is the store behaviour the channel handlers need.
type ChannelRepo interface {
	Create(ctx context.Context, c *model.Channel) error
	Get(ctx context.Context, id int64) (*model.Channel, error)
	List(ctx context.Context) ([]model.Channel, error)
	Update(ctx context.Context, c *model.Channel, changeConfig bool) error
	Delete(ctx context.Context, id int64) error
}

// TestSender sends a message to a provider given plaintext config JSON.
type TestSender interface {
	Send(ctx context.Context, channelType string, configJSON []byte, message string) error
}

// Channels holds the channel resource handlers.
type Channels struct {
	repo   ChannelRepo
	cipher *crypto.Cipher
	sender TestSender
}

// NewChannels builds the channel handlers. cipher may be nil when no encryption
// key is configured, in which case create/update/test are rejected.
func NewChannels(repo ChannelRepo, cipher *crypto.Cipher, sender TestSender) *Channels {
	return &Channels{repo: repo, cipher: cipher, sender: sender}
}

// Routes mounts the channel endpoints onto r.
func (h *Channels) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Post("/test", h.testUnsaved)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
		r.Post("/test", h.testSaved)
	})
}

type channelRequest struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Enabled *bool           `json:"enabled"`
	Config  json.RawMessage `json:"config"`
}

func validChannelType(t string) bool {
	switch t {
	case model.ChannelShoutrrr, model.ChannelWhatsAppGreenAPI, model.ChannelWhatsAppMultidevice:
		return true
	}
	return false
}

func (h *Channels) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list channels")
		return
	}
	WriteJSON(w, http.StatusOK, list)
}

func (h *Channels) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeChannel(w, r)
	if !ok {
		return
	}
	if req.Name == "" || !validChannelType(req.Type) {
		WriteError(w, http.StatusBadRequest, "invalid", "name and a valid type are required")
		return
	}
	if len(req.Config) == 0 {
		WriteError(w, http.StatusBadRequest, "invalid", "config is required")
		return
	}
	enc, ok := h.encrypt(w, req.Config)
	if !ok {
		return
	}
	ch := &model.Channel{
		Name: req.Name, Type: req.Type, ConfigEncrypted: enc,
		Enabled: boolOr(req.Enabled, true),
	}
	if err := h.repo.Create(r.Context(), ch); err != nil {
		writeChannelErr(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, ch)
}

func (h *Channels) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ch, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeChannelErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ch)
}

func (h *Channels) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	req, ok := decodeChannel(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeChannelErr(w, err)
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Type != "" {
		if !validChannelType(req.Type) {
			WriteError(w, http.StatusBadRequest, "invalid", "invalid channel type")
			return
		}
		existing.Type = req.Type
	}
	existing.Enabled = boolOr(req.Enabled, existing.Enabled)

	changeConfig := len(req.Config) > 0
	if changeConfig {
		enc, ok := h.encrypt(w, req.Config)
		if !ok {
			return
		}
		existing.ConfigEncrypted = enc
		existing.NeedsCredentials = false // secrets (re)entered
	}
	if err := h.repo.Update(r.Context(), existing, changeConfig); err != nil {
		writeChannelErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, existing)
}

func (h *Channels) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeChannelErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const testMessage = "✅ Bothan test message — your notification channel is working."

// testUnsaved sends a test using config supplied in the request body.
func (h *Channels) testUnsaved(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeChannel(w, r)
	if !ok {
		return
	}
	if !validChannelType(req.Type) || len(req.Config) == 0 {
		WriteError(w, http.StatusBadRequest, "invalid", "type and config are required")
		return
	}
	if err := h.sender.Send(r.Context(), req.Type, req.Config, testMessage); err != nil {
		WriteError(w, http.StatusBadGateway, "send_failed", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

// testSaved sends a test using a stored channel's decrypted config.
func (h *Channels) testSaved(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ch, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeChannelErr(w, err)
		return
	}
	if h.cipher == nil {
		WriteError(w, http.StatusPreconditionFailed, "no_key", "encryption key not configured")
		return
	}
	cfg, err := h.cipher.Decrypt(ch.ConfigEncrypted)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to decrypt channel config")
		return
	}
	if err := h.sender.Send(r.Context(), ch.Type, cfg, testMessage); err != nil {
		WriteError(w, http.StatusBadGateway, "send_failed", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

func (h *Channels) encrypt(w http.ResponseWriter, config json.RawMessage) ([]byte, bool) {
	if h.cipher == nil {
		WriteError(w, http.StatusPreconditionFailed, "no_key",
			"encryption key not configured; set BOTHAN_CRYPTO_ENCRYPTION_KEY")
		return nil, false
	}
	enc, err := h.cipher.Encrypt(config)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to encrypt config")
		return nil, false
	}
	return enc, true
}

func decodeChannel(w http.ResponseWriter, r *http.Request) (channelRequest, bool) {
	var req channelRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return channelRequest{}, false
	}
	return req, true
}

func writeChannelErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "channel not found")
	case errors.Is(err, store.ErrConflict):
		WriteError(w, http.StatusConflict, "conflict", "a channel with that name already exists")
	default:
		WriteError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}
