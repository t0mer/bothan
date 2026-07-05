package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

// RuleRepo is the store behaviour the rule handlers need.
type RuleRepo interface {
	Create(ctx context.Context, r *model.Rule) error
	Get(ctx context.Context, id int64) (*model.Rule, error)
	List(ctx context.Context) ([]model.Rule, error)
	Update(ctx context.Context, r *model.Rule) error
	Delete(ctx context.Context, id int64) error
}

// Rules holds the rule resource handlers.
type Rules struct {
	repo RuleRepo
}

// NewRules builds the rule handlers.
func NewRules(repo RuleRepo) *Rules { return &Rules{repo: repo} }

// Routes mounts the rule endpoints onto r.
func (h *Rules) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
	})
}

type ruleRequest struct {
	HostID         *int64 `json:"host_id"`
	Name           string `json:"name"`
	ConditionType  string `json:"condition_type"`
	ThresholdGrade string `json:"threshold_grade"`
	ExpiryDays     *int   `json:"expiry_days"`
	Enabled        *bool  `json:"enabled"`
}

func (h *Rules) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list rules")
		return
	}
	WriteJSON(w, http.StatusOK, list)
}

func (h *Rules) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRule(w, r)
	if !ok {
		return
	}
	rule := &model.Rule{
		HostID:         req.HostID,
		Name:           req.Name,
		ConditionType:  req.ConditionType,
		ThresholdGrade: req.ThresholdGrade,
		ExpiryDays:     req.ExpiryDays,
		Enabled:        boolOr(req.Enabled, true),
	}
	if msg := validateRule(rule); msg != "" {
		WriteError(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	if err := h.repo.Create(r.Context(), rule); err != nil {
		writeRuleErr(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, rule)
}

func (h *Rules) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rule, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRuleErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, rule)
}

func (h *Rules) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	req, ok := decodeRule(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRuleErr(w, err)
		return
	}
	existing.HostID = req.HostID
	existing.Name = req.Name
	existing.ConditionType = req.ConditionType
	existing.ThresholdGrade = req.ThresholdGrade
	existing.ExpiryDays = req.ExpiryDays
	existing.Enabled = boolOr(req.Enabled, existing.Enabled)
	if msg := validateRule(existing); msg != "" {
		WriteError(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	if err := h.repo.Update(r.Context(), existing); err != nil {
		writeRuleErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, existing)
}

func (h *Rules) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeRuleErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateRule(rule *model.Rule) string {
	if rule.Name == "" {
		return "name is required"
	}
	if !model.ConditionTypes[rule.ConditionType] {
		return "invalid condition_type"
	}
	if rule.ConditionType == model.CondGradeBelow {
		if rule.ThresholdGrade == "" || model.GradeRank(rule.ThresholdGrade) == model.GradeRankUnknown {
			return "grade_below requires a valid threshold_grade"
		}
	}
	if rule.ConditionType == model.CondCertExpiry {
		if rule.ExpiryDays == nil || *rule.ExpiryDays < 1 {
			return "cert_expiry requires expiry_days >= 1"
		}
	}
	return ""
}

func decodeRule(w http.ResponseWriter, r *http.Request) (ruleRequest, bool) {
	var req ruleRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return ruleRequest{}, false
	}
	return req, true
}

func writeRuleErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "not_found", "rule not found")
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal", "internal error")
}
