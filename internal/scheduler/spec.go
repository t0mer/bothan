// Package scheduler runs the cron registry that enqueues scheduled scans.
package scheduler

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// friendlyText maps human-friendly schedule words to cron descriptors.
var friendlyText = map[string]string{
	"everyday": "@daily",
	"daily":    "@daily",
	"hourly":   "@hourly",
	"weekly":   "@weekly",
	"monthly":  "@monthly",
}

// specParser accepts standard 5-field cron plus @descriptors.
var specParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// NormalizeSpec converts friendly text to a cron descriptor and validates the
// result. It returns the normalized spec or an error if it is not parseable.
func NormalizeSpec(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("schedule spec is required")
	}
	normalized := trimmed
	if desc, ok := friendlyText[strings.ToLower(trimmed)]; ok {
		normalized = desc
	}
	if _, err := specParser.Parse(normalized); err != nil {
		return "", fmt.Errorf("invalid schedule spec %q: %w", input, err)
	}
	return normalized, nil
}
