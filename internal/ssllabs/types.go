// Package ssllabs is a client for the Qualys SSL Labs Assessment API (v4, with
// v3 as a legacy fallback). It covers /info, /analyze polling, and v4 email
// registration, with rate-limit and back-off discipline.
package ssllabs

// Info is the response from the /info endpoint.
type Info struct {
	EngineVersion        string   `json:"engineVersion"`
	CriteriaVersion      string   `json:"criteriaVersion"`
	MaxAssessments       int      `json:"maxAssessments"`
	CurrentAssessments   int      `json:"currentAssessments"`
	NewAssessmentCoolOff int64    `json:"newAssessmentCoolOff"` // milliseconds
	Messages             []string `json:"messages"`
}

// Host is the response from the /analyze endpoint — one assessment.
type Host struct {
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Protocol        string     `json:"protocol"`
	Status          string     `json:"status"` // DNS | IN_PROGRESS | READY | ERROR
	StatusMessage   string     `json:"statusMessage"`
	StartTime       int64      `json:"startTime"`
	TestTime        int64      `json:"testTime"`
	EngineVersion   string     `json:"engineVersion"`
	CriteriaVersion string     `json:"criteriaVersion"`
	Endpoints       []Endpoint `json:"endpoints"`
	Certs           []Cert     `json:"certs"`
}

// Endpoint is a single IP assessed for a host.
type Endpoint struct {
	IPAddress         string           `json:"ipAddress"`
	ServerName        string           `json:"serverName"`
	StatusMessage     string           `json:"statusMessage"`
	Grade             string           `json:"grade"`
	GradeTrustIgnored string           `json:"gradeTrustIgnored"`
	HasWarnings       bool             `json:"hasWarnings"`
	IsExceptional     bool             `json:"isExceptional"`
	Progress          int              `json:"progress"`
	Details           *EndpointDetails `json:"details"`
}

// EndpointDetails carries the deep assessment data. Only the fields Bothan uses
// directly are typed; the full object is preserved in the scan's raw JSON.
type EndpointDetails struct {
	CertChains []CertChain `json:"certChains"`

	// Known vulnerability flags (see §9 vuln_detected).
	VulnBeast       bool `json:"vulnBeast"`
	Heartbleed      bool `json:"heartbleed"`
	Poodle          bool `json:"poodle"`
	PoodleTLS       int  `json:"poodleTls"`
	Freak           bool `json:"freak"`
	Logjam          bool `json:"logjam"`
	DrownVulnerable bool `json:"drownVulnerable"`
}

// CertChain references certificates by id.
type CertChain struct {
	CertIDs []string `json:"certIds"`
}

// Cert is a certificate in the top-level certs array.
type Cert struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	NotAfter int64  `json:"notAfter"` // epoch milliseconds
	Issuer   string `json:"issuerSubject"`
}

// IsReady reports whether the assessment finished successfully.
func (h *Host) IsReady() bool { return h.Status == "READY" }

// IsError reports whether the assessment ended in error.
func (h *Host) IsError() bool { return h.Status == "ERROR" }

// EndpointReady reports whether an individual endpoint finished ("Ready").
func (e *Endpoint) EndpointReady() bool { return e.StatusMessage == "Ready" }

// Vulnerabilities returns the names of detected vulnerabilities on the endpoint.
func (e *Endpoint) Vulnerabilities() []string {
	if e.Details == nil {
		return nil
	}
	var v []string
	d := e.Details
	if d.Heartbleed {
		v = append(v, "Heartbleed")
	}
	if d.Poodle {
		v = append(v, "POODLE")
	}
	if d.PoodleTLS > 1 {
		v = append(v, "POODLE-TLS")
	}
	if d.Freak {
		v = append(v, "FREAK")
	}
	if d.Logjam {
		v = append(v, "Logjam")
	}
	if d.DrownVulnerable {
		v = append(v, "DROWN")
	}
	if d.VulnBeast {
		v = append(v, "BEAST")
	}
	return v
}
