package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/bundle"
	"github.com/t0mer/bothan/internal/crypto"
	"github.com/t0mer/bothan/internal/store"
)

// ConfigHandler serves configuration export/import.
type ConfigHandler struct {
	store      *store.Store
	cipher     *crypto.Cipher
	appVersion string
	scheduler  SchedulerControl
}

// NewConfig builds the config export/import handler.
func NewConfig(st *store.Store, cipher *crypto.Cipher, appVersion string, sched SchedulerControl) *ConfigHandler {
	return &ConfigHandler{store: st, cipher: cipher, appVersion: appVersion, scheduler: sched}
}

// Routes mounts the config endpoints onto r.
func (h *ConfigHandler) Routes(r chi.Router) {
	r.Get("/export", h.exportNone)
	r.Post("/export", h.exportSecrets)
	r.Post("/import", h.importBundle)
}

func (h *ConfigHandler) exportNone(w http.ResponseWriter, r *http.Request) {
	h.writeExport(w, r, bundle.SecretNone, "")
}

type exportRequest struct {
	SecretEncryption string `json:"secret_encryption"`
	Passphrase       string `json:"passphrase"`
}

func (h *ConfigHandler) exportSecrets(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return
	}
	if req.SecretEncryption == "" {
		req.SecretEncryption = bundle.SecretNone
	}
	h.writeExport(w, r, req.SecretEncryption, req.Passphrase)
}

func (h *ConfigHandler) writeExport(w http.ResponseWriter, r *http.Request, mode, passphrase string) {
	b, err := bundle.Export(r.Context(), h.store, h.cipher, bundle.ExportOptions{
		Mode: mode, Passphrase: passphrase, AppVersion: h.appVersion, Now: time.Now(),
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "export_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="bothan-config.json"`)
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(b)
}

func (h *ConfigHandler) importBundle(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = store.ImportMerge
	}
	dryRun := r.URL.Query().Get("dry_run") == "true"
	passphrase := r.URL.Query().Get("passphrase")

	var b bundle.Bundle
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed bundle: "+err.Error())
		return
	}

	rep, err := bundle.Import(r.Context(), h.store, h.cipher, &b, bundle.ImportOptions{
		Mode: mode, DryRun: dryRun, Passphrase: passphrase,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "import_failed", err.Error())
		return
	}
	if !dryRun && h.scheduler != nil {
		_ = h.scheduler.Rebuild(context.Background())
	}
	WriteJSON(w, http.StatusOK, map[string]any{"dry_run": dryRun, "mode": mode, "report": rep})
}
