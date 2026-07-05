// Package model holds Bothan's domain types shared across the store and API.
package model

import "time"

// Host is a monitored hostname and its per-host scan options.
type Host struct {
	ID             int64     `json:"id"`
	Hostname       string    `json:"hostname"`
	Enabled        bool      `json:"enabled"`
	Publish        bool      `json:"publish"` // SSL Labs publish flag; false = private (default)
	IgnoreMismatch bool      `json:"ignore_mismatch"`
	FromCache      bool      `json:"from_cache"`
	MaxAgeHours    *int      `json:"max_age_hours,omitempty"` // used only when FromCache is true
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
