package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Scans holds the scan resource handlers (read-only detail views).
type Scans struct {
	scans ScanReader
}

// NewScans builds the scan handlers.
func NewScans(scans ScanReader) *Scans { return &Scans{scans: scans} }

// Routes mounts the scan endpoints onto r.
func (h *Scans) Routes(r chi.Router) {
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Get("/raw", h.raw)
	})
}

func (h *Scans) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sc, err := h.scans.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, sc)
}

func (h *Scans) raw(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	raw, err := h.scans.GetRaw(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if len(raw) == 0 {
		WriteError(w, http.StatusNotFound, "not_found", "no raw result for this scan")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
