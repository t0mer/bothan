package model

import "time"

// DashboardSummary is the aggregate view for the dashboard.
type DashboardSummary struct {
	TotalHosts    int          `json:"total_hosts"`
	EnabledHosts  int          `json:"enabled_hosts"`
	DisabledHosts int          `json:"disabled_hosts"`
	NeverScanned  int          `json:"never_scanned"`
	GradeCounts   []GradeCount `json:"grade_counts"`

	CertExpiryWindowDays int          `json:"cert_expiry_window_days"`
	CertsExpiringSoon    []CertExpiry `json:"certs_expiring_soon"`

	RecentScans []RecentScan `json:"recent_scans"`
}

// GradeCount is the number of hosts currently at a grade (by latest ready scan).
type GradeCount struct {
	Grade string `json:"grade"`
	Count int    `json:"count"`
}

// CertExpiry is a host whose earliest certificate expires within the window.
type CertExpiry struct {
	HostID   int64     `json:"host_id"`
	Hostname string    `json:"hostname"`
	NotAfter time.Time `json:"not_after"`
	Days     int       `json:"days"`
}

// RecentScan is a recent scan across all hosts.
type RecentScan struct {
	ScanID      int64      `json:"scan_id"`
	HostID      int64      `json:"host_id"`
	Hostname    string     `json:"hostname"`
	Grade       string     `json:"grade"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// GradeDisplayOrder is the canonical order for showing grade buckets.
var GradeDisplayOrder = []string{"A+", "A", "A-", "B", "C", "D", "E", "F", "T", "M", "none"}
