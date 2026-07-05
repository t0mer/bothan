package model

import "time"

// Rule condition types (§9).
const (
	CondGradeBelow      = "grade_below"
	CondGradeChanged    = "grade_changed"
	CondGradeDowngraded = "grade_downgraded"
	CondGradeImproved   = "grade_improved"
	CondCertExpiry      = "cert_expiry"
	CondScanFailed      = "scan_failed"
	CondVulnDetected    = "vuln_detected"
	CondScanCompleted   = "scan_completed"
)

// ConditionTypes is the set of valid rule conditions.
var ConditionTypes = map[string]bool{
	CondGradeBelow: true, CondGradeChanged: true, CondGradeDowngraded: true,
	CondGradeImproved: true, CondCertExpiry: true, CondScanFailed: true,
	CondVulnDetected: true, CondScanCompleted: true,
}

// Rule is a notification rule. A nil HostID makes it a global rule applied to
// all hosts.
type Rule struct {
	ID             int64     `json:"id"`
	HostID         *int64    `json:"host_id,omitempty"`
	Name           string    `json:"name"`
	ConditionType  string    `json:"condition_type"`
	ThresholdGrade string    `json:"threshold_grade,omitempty"`
	ExpiryDays     *int      `json:"expiry_days,omitempty"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
