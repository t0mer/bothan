// Package api holds Bothan's /api/v1 HTTP handlers and shared response helpers.
// Phase 1 provides only the response envelope; resource handlers arrive in
// later phases.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorEnvelope is the consistent error response body used across the API:
//
//	{ "error": { "code": "...", "message": "..." } }
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries a machine-readable code and a human-readable message.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding JSON response", slog.String("error", err.Error()))
	}
}

// WriteError writes a standard error envelope with the given status.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}
