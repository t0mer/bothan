package model

import "time"

// Scan statuses.
const (
	ScanStatusPending = "pending"
	ScanStatusRunning = "running"
	ScanStatusReady   = "ready"
	ScanStatusError   = "error"
)

// Scan triggers.
const (
	TriggerManual = "manual"
	TriggerAPI    = "api"
	// schedule triggers are "schedule:<name>"
)

// Scan is one assessment run for a host.
type Scan struct {
	ID              int64          `json:"id"`
	HostID          int64          `json:"host_id"`
	Status          string         `json:"status"`
	Trigger         string         `json:"trigger"`
	OverallGrade    string         `json:"overall_grade"`
	EngineVersion   string         `json:"engine_version"`
	CriteriaVersion string         `json:"criteria_version"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	StartedAt       *time.Time     `json:"started_at,omitempty"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	Endpoints       []ScanEndpoint `json:"endpoints,omitempty"`
}

// HostScanSummary is a host's most recent scan status/grade, for list views.
type HostScanSummary struct {
	Status      string     `json:"status"`
	Grade       string     `json:"grade"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ScanEndpoint is a per-IP result within a scan.
type ScanEndpoint struct {
	ID                int64      `json:"id"`
	ScanID            int64      `json:"scan_id"`
	IPAddress         string     `json:"ip_address"`
	ServerName        string     `json:"server_name,omitempty"`
	Grade             string     `json:"grade"`
	GradeTrustIgnored string     `json:"grade_trust_ignored,omitempty"`
	HasWarnings       bool       `json:"has_warnings"`
	IsExceptional     bool       `json:"is_exceptional"`
	StatusMessage     string     `json:"status_message,omitempty"`
	CertNotAfter      *time.Time `json:"cert_not_after,omitempty"`
	Progress          int        `json:"progress"`
}
