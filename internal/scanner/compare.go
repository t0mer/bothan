package scanner

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/ssllabs"
)

// ScanRef identifies one side of a comparison.
type ScanRef struct {
	ID        int64     `json:"id"`
	Grade     string    `json:"grade"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CertInfo summarises a certificate for diffing.
type CertInfo struct {
	Subject  string     `json:"subject,omitempty"`
	Issuer   string     `json:"issuer,omitempty"`
	NotAfter *time.Time `json:"not_after,omitempty"`
}

// EndpointDiff is the comparison for a single IP across two scans.
type EndpointDiff struct {
	IPAddress        string    `json:"ip_address"`
	Change           string    `json:"change"` // added | removed | changed | unchanged
	FromGrade        string    `json:"from_grade,omitempty"`
	ToGrade          string    `json:"to_grade,omitempty"`
	GradeChanged     bool      `json:"grade_changed"`
	FromCert         *CertInfo `json:"from_cert,omitempty"`
	ToCert           *CertInfo `json:"to_cert,omitempty"`
	CertChanged      bool      `json:"cert_changed"`
	ProtocolsAdded   []string  `json:"protocols_added,omitempty"`
	ProtocolsRemoved []string  `json:"protocols_removed,omitempty"`
	VulnsAdded       []string  `json:"vulns_added,omitempty"`
	VulnsRemoved     []string  `json:"vulns_removed,omitempty"`
}

// Diff is a structured comparison of two scans of the same host.
type Diff struct {
	HostID              int64          `json:"host_id"`
	From                ScanRef        `json:"from"`
	To                  ScanRef        `json:"to"`
	OverallGradeChanged bool           `json:"overall_grade_changed"`
	Endpoints           []EndpointDiff `json:"endpoints"`
}

// Compare produces a structured diff between two scans of the same host. The
// raw JSON is used for protocol/cert/vuln detail; endpoint grades come from the
// persisted rows.
func Compare(from, to *model.Scan, fromRaw, toRaw []byte) *Diff {
	d := &Diff{
		HostID:              to.HostID,
		From:                scanRef(from),
		To:                  scanRef(to),
		OverallGradeChanged: from.OverallGrade != to.OverallGrade,
	}

	fromEP := endpointDetail(from, fromRaw)
	toEP := endpointDetail(to, toRaw)

	ips := unionIPs(fromEP, toEP)
	for _, ip := range ips {
		f, hasF := fromEP[ip]
		t, hasT := toEP[ip]
		ed := EndpointDiff{IPAddress: ip}
		switch {
		case hasF && !hasT:
			ed.Change = "removed"
			ed.FromGrade, ed.FromCert = f.grade, f.cert
		case !hasF && hasT:
			ed.Change = "added"
			ed.ToGrade, ed.ToCert = t.grade, t.cert
		default:
			ed.FromGrade, ed.ToGrade = f.grade, t.grade
			ed.FromCert, ed.ToCert = f.cert, t.cert
			ed.GradeChanged = f.grade != t.grade
			ed.CertChanged = certChanged(f.cert, t.cert)
			ed.ProtocolsAdded, ed.ProtocolsRemoved = diffSets(f.protocols, t.protocols)
			ed.VulnsAdded, ed.VulnsRemoved = diffSets(f.vulns, t.vulns)
			if ed.GradeChanged || ed.CertChanged || len(ed.ProtocolsAdded) > 0 ||
				len(ed.ProtocolsRemoved) > 0 || len(ed.VulnsAdded) > 0 || len(ed.VulnsRemoved) > 0 {
				ed.Change = "changed"
			} else {
				ed.Change = "unchanged"
			}
		}
		d.Endpoints = append(d.Endpoints, ed)
	}
	return d
}

type epDetail struct {
	grade     string
	cert      *CertInfo
	protocols []string
	vulns     []string
}

// endpointDetail merges persisted grades/cert with raw protocol/vuln info.
func endpointDetail(scan *model.Scan, raw []byte) map[string]epDetail {
	out := map[string]epDetail{}
	for _, ep := range scan.Endpoints {
		d := epDetail{grade: ep.Grade}
		if ep.CertNotAfter != nil {
			d.cert = &CertInfo{NotAfter: ep.CertNotAfter}
		}
		out[ep.IPAddress] = d
	}

	var host ssllabs.Host
	if len(raw) > 0 && json.Unmarshal(raw, &host) == nil {
		certByID := map[string]ssllabs.Cert{}
		for _, c := range host.Certs {
			certByID[c.ID] = c
		}
		for _, e := range host.Endpoints {
			d := out[e.IPAddress]
			if d.grade == "" {
				d.grade = e.Grade
			}
			if e.Details != nil {
				d.vulns = e.Vulnerabilities()
				for _, p := range e.Details.Protocols {
					d.protocols = append(d.protocols, p.String())
				}
				if ci := leafCertInfo(e, certByID); ci != nil {
					d.cert = ci
				}
			}
			out[e.IPAddress] = d
		}
	}
	return out
}

func leafCertInfo(e ssllabs.Endpoint, byID map[string]ssllabs.Cert) *CertInfo {
	if e.Details == nil {
		return nil
	}
	for _, chain := range e.Details.CertChains {
		for _, id := range chain.CertIDs {
			if c, ok := byID[id]; ok {
				info := &CertInfo{Subject: c.Subject, Issuer: c.Issuer}
				if c.NotAfter > 0 {
					t := time.UnixMilli(c.NotAfter).UTC()
					info.NotAfter = &t
				}
				return info
			}
		}
	}
	return nil
}

func certChanged(a, b *CertInfo) bool {
	if a == nil || b == nil {
		return a != b
	}
	if a.Subject != b.Subject || a.Issuer != b.Issuer {
		return true
	}
	return !sameTime(a.NotAfter, b.NotAfter)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func scanRef(s *model.Scan) ScanRef {
	return ScanRef{ID: s.ID, Grade: s.OverallGrade, Status: s.Status, CreatedAt: s.CreatedAt}
}

func unionIPs(a, b map[string]epDetail) []string {
	set := map[string]bool{}
	for ip := range a {
		set[ip] = true
	}
	for ip := range b {
		set[ip] = true
	}
	ips := make([]string, 0, len(set))
	for ip := range set {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips
}

// diffSets returns items added in b (not in a) and removed from a (not in b).
func diffSets(a, b []string) (added, removed []string) {
	as, bs := toSet(a), toSet(b)
	for v := range bs {
		if !as[v] {
			added = append(added, v)
		}
	}
	for v := range as {
		if !bs[v] {
			removed = append(removed, v)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}
