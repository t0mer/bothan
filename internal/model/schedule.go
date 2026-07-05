package model

import "time"

// Schedule is a reusable cron schedule that can be attached to many hosts.
type Schedule struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Spec      string    `json:"spec"` // normalized cron expr or @descriptor
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
