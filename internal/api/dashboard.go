package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/model"
)

// DashboardRepo is the store behaviour the dashboard handler needs.
type DashboardRepo interface {
	Summary(ctx context.Context, certWindowDays, recentLimit int) (*model.DashboardSummary, error)
}

// Dashboard holds the dashboard handler.
type Dashboard struct {
	repo DashboardRepo
}

// NewDashboard builds the dashboard handler.
func NewDashboard(repo DashboardRepo) *Dashboard { return &Dashboard{repo: repo} }

// Routes mounts the dashboard endpoints onto r.
func (h *Dashboard) Routes(r chi.Router) {
	r.Get("/summary", h.summary)
}

func (h *Dashboard) summary(w http.ResponseWriter, r *http.Request) {
	window := queryInt(r, "cert_days", 30)
	recent := queryInt(r, "recent", 10)
	sum, err := h.repo.Summary(r.Context(), window, recent)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to build dashboard summary")
		return
	}
	WriteJSON(w, http.StatusOK, sum)
}
