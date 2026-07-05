package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/scanner"
	"github.com/t0mer/bothan/internal/store"
)

// HostRepo is the subset of the store the host handlers depend on.
type HostRepo interface {
	Create(ctx context.Context, h *model.Host) error
	Get(ctx context.Context, id int64) (*model.Host, error)
	List(ctx context.Context) ([]model.Host, error)
	Update(ctx context.Context, h *model.Host) error
	SetEnabled(ctx context.Context, id int64, enabled bool) error
	Delete(ctx context.Context, id int64) error
}

// Scanner triggers scans (satisfied by *scanner.Service).
type Scanner interface {
	Trigger(ctx context.Context, hostID int64, trigger string) (*model.Scan, error)
}

// ScanReader reads persisted scans (satisfied by *store.ScanRepo).
type ScanReader interface {
	Get(ctx context.Context, id int64) (*model.Scan, error)
	GetRaw(ctx context.Context, id int64) ([]byte, error)
	ListByHost(ctx context.Context, hostID int64, limit int) ([]model.Scan, error)
	LatestByHosts(ctx context.Context) (map[int64]model.HostScanSummary, error)
}

// HostScheduleLinker manages a host's schedule links.
type HostScheduleLinker interface {
	SetHostSchedules(ctx context.Context, hostID int64, scheduleIDs []int64) error
	SchedulesForHost(ctx context.Context, hostID int64) ([]model.Schedule, error)
}

// Hosts holds the host resource handlers.
type Hosts struct {
	repo           HostRepo
	defaultPublish func() bool
	scanner        Scanner
	scans          ScanReader
	schedules      HostScheduleLinker
	sched          SchedulerControl
}

// HostsDeps are the collaborators the host handlers need.
type HostsDeps struct {
	Repo           HostRepo
	DefaultPublish func() bool
	Scanner        Scanner
	Scans          ScanReader
	Schedules      HostScheduleLinker
	Scheduler      SchedulerControl
}

// NewHosts builds the host handlers.
func NewHosts(d HostsDeps) *Hosts {
	return &Hosts{
		repo:           d.Repo,
		defaultPublish: d.DefaultPublish,
		scanner:        d.Scanner,
		scans:          d.Scans,
		schedules:      d.Schedules,
		sched:          d.Scheduler,
	}
}

// Routes mounts the host endpoints onto r.
func (h *Hosts) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
		r.Post("/enable", h.enable)
		r.Post("/disable", h.disable)
		r.Post("/scan", h.scan)
		r.Get("/scans", h.scanHistory)
		r.Get("/schedules", h.getSchedules)
		r.Put("/schedules", h.setSchedules)
	})
}

func (h *Hosts) getSchedules(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	scheds, err := h.schedules.SchedulesForHost(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to load schedules")
		return
	}
	WriteJSON(w, http.StatusOK, scheds)
}

type idListRequest struct {
	IDs []int64 `json:"ids"`
}

func (h *Hosts) setSchedules(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	var req idListRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return
	}
	if err := h.schedules.SetHostSchedules(r.Context(), id, req.IDs); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "failed to set schedules: "+err.Error())
		return
	}
	if h.sched != nil {
		_ = h.sched.Rebuild(r.Context())
	}
	scheds, _ := h.schedules.SchedulesForHost(r.Context(), id)
	WriteJSON(w, http.StatusOK, scheds)
}

func (h *Hosts) scan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sc, err := h.scanner.Trigger(r.Context(), id, model.TriggerManual)
	if err != nil {
		if errors.Is(err, scanner.ErrScanInProgress) {
			WriteError(w, http.StatusConflict, "scan_in_progress", err.Error())
			return
		}
		if errors.Is(err, scanner.ErrNoEmail) {
			WriteError(w, http.StatusPreconditionFailed, "ssllabs_unregistered", err.Error())
			return
		}
		writeStoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, sc)
}

func (h *Hosts) scanHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	limit := queryInt(r, "limit", 50)
	scans, err := h.scans.ListByHost(r.Context(), id, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list scans")
		return
	}
	WriteJSON(w, http.StatusOK, scans)
}

// hostRequest is the create/update payload. Pointers distinguish "omitted" from
// "false" so create defaults can apply.
type hostRequest struct {
	Hostname       string `json:"hostname"`
	Enabled        *bool  `json:"enabled"`
	Publish        *bool  `json:"publish"`
	IgnoreMismatch *bool  `json:"ignore_mismatch"`
	FromCache      *bool  `json:"from_cache"`
	MaxAgeHours    *int   `json:"max_age_hours"`
	Notes          string `json:"notes"`
}

// hostListItem is a host enriched with its latest scan summary for list views.
type hostListItem struct {
	model.Host
	LatestGrade    string     `json:"latest_grade,omitempty"`
	LastScanStatus string     `json:"last_scan_status,omitempty"`
	LastScanAt     *time.Time `json:"last_scan_at,omitempty"`
}

func (h *Hosts) list(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.repo.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list hosts")
		return
	}

	var summaries map[int64]model.HostScanSummary
	if h.scans != nil {
		summaries, _ = h.scans.LatestByHosts(r.Context())
	}

	items := make([]hostListItem, 0, len(hosts))
	for _, host := range hosts {
		item := hostListItem{Host: host}
		if s, ok := summaries[host.ID]; ok {
			item.LatestGrade = s.Grade
			item.LastScanStatus = s.Status
			item.LastScanAt = s.CompletedAt
		}
		items = append(items, item)
	}
	WriteJSON(w, http.StatusOK, items)
}

func (h *Hosts) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeHostRequest(w, r)
	if !ok {
		return
	}
	host := &model.Host{
		Hostname:       req.Hostname,
		Enabled:        boolOr(req.Enabled, true),
		Publish:        boolOr(req.Publish, h.defaultPublish()),
		IgnoreMismatch: boolOr(req.IgnoreMismatch, false),
		FromCache:      boolOr(req.FromCache, false),
		MaxAgeHours:    req.MaxAgeHours,
		Notes:          req.Notes,
	}
	if msg := validateHost(host); msg != "" {
		WriteError(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	if err := h.repo.Create(r.Context(), host); err != nil {
		writeStoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, host)
}

func (h *Hosts) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	host, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, host)
}

func (h *Hosts) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	req, ok := decodeHostRequest(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// Apply fields; unspecified booleans retain their current value.
	existing.Hostname = req.Hostname
	existing.Enabled = boolOr(req.Enabled, existing.Enabled)
	existing.Publish = boolOr(req.Publish, existing.Publish)
	existing.IgnoreMismatch = boolOr(req.IgnoreMismatch, existing.IgnoreMismatch)
	existing.FromCache = boolOr(req.FromCache, existing.FromCache)
	existing.MaxAgeHours = req.MaxAgeHours
	existing.Notes = req.Notes

	if msg := validateHost(existing); msg != "" {
		WriteError(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	if err := h.repo.Update(r.Context(), existing); err != nil {
		writeStoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, existing)
}

func (h *Hosts) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Hosts) enable(w http.ResponseWriter, r *http.Request)  { h.setEnabled(w, r, true) }
func (h *Hosts) disable(w http.ResponseWriter, r *http.Request) { h.setEnabled(w, r, false) }

func (h *Hosts) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.SetEnabled(r.Context(), id, enabled); err != nil {
		writeStoreError(w, err)
		return
	}
	host, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, host)
}

// --- helpers ---

func decodeHostRequest(w http.ResponseWriter, r *http.Request) (hostRequest, bool) {
	var req hostRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return hostRequest{}, false
	}
	return req, true
}

func validateHost(h *model.Host) string {
	h.Hostname = strings.TrimSpace(h.Hostname)
	if h.Hostname == "" {
		return "hostname is required"
	}
	if strings.ContainsAny(h.Hostname, " /:") {
		return "hostname must be a bare host (no scheme, port, or path)"
	}
	if h.MaxAgeHours != nil && *h.MaxAgeHours < 1 {
		return "max_age_hours must be >= 1 when set"
	}
	return ""
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		WriteError(w, http.StatusBadRequest, "invalid", "invalid host id")
		return 0, false
	}
	return id, true
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, store.ErrConflict):
		WriteError(w, http.StatusConflict, "conflict", "a host with that hostname already exists")
	default:
		WriteError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
