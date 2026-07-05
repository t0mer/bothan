package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/scheduler"
	"github.com/t0mer/bothan/internal/store"
)

// ScheduleRepo is the store behaviour the schedule handlers need.
type ScheduleRepo interface {
	Create(ctx context.Context, s *model.Schedule) error
	Get(ctx context.Context, id int64) (*model.Schedule, error)
	List(ctx context.Context) ([]model.Schedule, error)
	Update(ctx context.Context, s *model.Schedule) error
	Delete(ctx context.Context, id int64) error
}

// SchedulerControl rebuilds the cron registry after a change.
type SchedulerControl interface {
	Rebuild(ctx context.Context) error
}

// Schedules holds the schedule resource handlers.
type Schedules struct {
	repo  ScheduleRepo
	sched SchedulerControl
}

// NewSchedules builds the schedule handlers.
func NewSchedules(repo ScheduleRepo, sched SchedulerControl) *Schedules {
	return &Schedules{repo: repo, sched: sched}
}

// Routes mounts the schedule endpoints onto r.
func (h *Schedules) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
	})
}

type scheduleRequest struct {
	Name    string `json:"name"`
	Spec    string `json:"spec"`
	Enabled *bool  `json:"enabled"`
}

func (h *Schedules) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list schedules")
		return
	}
	WriteJSON(w, http.StatusOK, list)
}

func (h *Schedules) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeSchedule(w, r)
	if !ok {
		return
	}
	spec, err := scheduler.NormalizeSpec(req.Spec)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "invalid", "name is required")
		return
	}
	s := &model.Schedule{Name: req.Name, Spec: spec, Enabled: boolOr(req.Enabled, true)}
	if err := h.repo.Create(r.Context(), s); err != nil {
		writeScheduleErr(w, err)
		return
	}
	h.rebuild(r.Context())
	WriteJSON(w, http.StatusCreated, s)
}

func (h *Schedules) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeScheduleErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, s)
}

func (h *Schedules) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	req, ok := decodeSchedule(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeScheduleErr(w, err)
		return
	}
	spec, err := scheduler.NormalizeSpec(req.Spec)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "invalid", "name is required")
		return
	}
	existing.Name = req.Name
	existing.Spec = spec
	existing.Enabled = boolOr(req.Enabled, existing.Enabled)
	if err := h.repo.Update(r.Context(), existing); err != nil {
		writeScheduleErr(w, err)
		return
	}
	h.rebuild(r.Context())
	WriteJSON(w, http.StatusOK, existing)
}

func (h *Schedules) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeScheduleErr(w, err)
		return
	}
	h.rebuild(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Schedules) rebuild(ctx context.Context) {
	if h.sched != nil {
		_ = h.sched.Rebuild(ctx)
	}
}

func decodeSchedule(w http.ResponseWriter, r *http.Request) (scheduleRequest, bool) {
	var req scheduleRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return scheduleRequest{}, false
	}
	return req, true
}

func writeScheduleErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "schedule not found")
	case errors.Is(err, store.ErrConflict):
		WriteError(w, http.StatusConflict, "conflict", "a schedule with that name already exists")
	default:
		WriteError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}
