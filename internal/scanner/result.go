package scanner

import (
	"time"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/ssllabs"
)

// overallGrade is the lowest-ranked grade across the host's endpoints (§5).
func overallGrade(h *ssllabs.Host) string {
	grades := make([]string, 0, len(h.Endpoints))
	for _, e := range h.Endpoints {
		grades = append(grades, e.Grade)
	}
	return model.LowestGrade(grades)
}

// mapEndpoints converts SSL Labs endpoints to persisted scan endpoints.
func mapEndpoints(h *ssllabs.Host) []model.ScanEndpoint {
	certByID := map[string]ssllabs.Cert{}
	for _, c := range h.Certs {
		certByID[c.ID] = c
	}

	eps := make([]model.ScanEndpoint, 0, len(h.Endpoints))
	for _, e := range h.Endpoints {
		ep := model.ScanEndpoint{
			IPAddress:         e.IPAddress,
			ServerName:        e.ServerName,
			Grade:             e.Grade,
			GradeTrustIgnored: e.GradeTrustIgnored,
			HasWarnings:       e.HasWarnings,
			IsExceptional:     e.IsExceptional,
			StatusMessage:     e.StatusMessage,
			Progress:          e.Progress,
		}
		if t := earliestCert(e, certByID, h.Certs); t != nil {
			ep.CertNotAfter = t
		}
		eps = append(eps, ep)
	}
	return eps
}

// earliestCert returns the earliest certificate expiry relevant to an endpoint:
// the earliest notAfter among the certs referenced by its chains, falling back
// to the earliest across all of the host's certs.
func earliestCert(e ssllabs.Endpoint, byID map[string]ssllabs.Cert, all []ssllabs.Cert) *time.Time {
	var earliestMs int64
	found := false

	consider := func(ms int64) {
		if ms <= 0 {
			return
		}
		if !found || ms < earliestMs {
			earliestMs, found = ms, true
		}
	}

	if e.Details != nil {
		for _, chain := range e.Details.CertChains {
			for _, id := range chain.CertIDs {
				if c, ok := byID[id]; ok {
					consider(c.NotAfter)
				}
			}
		}
	}
	if !found {
		for _, c := range all {
			consider(c.NotAfter)
		}
	}
	if !found {
		return nil
	}
	t := time.UnixMilli(earliestMs).UTC()
	return &t
}
