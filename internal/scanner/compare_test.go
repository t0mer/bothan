package scanner

import (
	"testing"
	"time"

	"github.com/t0mer/bothan/internal/model"
)

func TestCompare_GradeAndEndpointChanges(t *testing.T) {
	t1 := time.Now().Add(-time.Hour)
	t2 := time.Now()
	from := &model.Scan{
		ID: 1, HostID: 9, Status: "ready", OverallGrade: "A+", CreatedAt: t1,
		Endpoints: []model.ScanEndpoint{
			{IPAddress: "1.1.1.1", Grade: "A+"},
			{IPAddress: "2.2.2.2", Grade: "A"},
		},
	}
	to := &model.Scan{
		ID: 2, HostID: 9, Status: "ready", OverallGrade: "B", CreatedAt: t2,
		Endpoints: []model.ScanEndpoint{
			{IPAddress: "1.1.1.1", Grade: "B"}, // downgraded
			{IPAddress: "3.3.3.3", Grade: "A"}, // added; 2.2.2.2 removed
		},
	}

	d := Compare(from, to, nil, nil)
	if !d.OverallGradeChanged {
		t.Error("overall grade should be marked changed")
	}
	byIP := map[string]EndpointDiff{}
	for _, e := range d.Endpoints {
		byIP[e.IPAddress] = e
	}
	if byIP["1.1.1.1"].Change != "changed" || !byIP["1.1.1.1"].GradeChanged {
		t.Errorf("1.1.1.1 should be changed: %+v", byIP["1.1.1.1"])
	}
	if byIP["2.2.2.2"].Change != "removed" {
		t.Errorf("2.2.2.2 should be removed: %+v", byIP["2.2.2.2"])
	}
	if byIP["3.3.3.3"].Change != "added" {
		t.Errorf("3.3.3.3 should be added: %+v", byIP["3.3.3.3"])
	}
}

func TestCompare_ProtocolAndVulnDiffFromRaw(t *testing.T) {
	from := &model.Scan{ID: 1, HostID: 1, Status: "ready", OverallGrade: "A", Endpoints: []model.ScanEndpoint{{IPAddress: "1.1.1.1", Grade: "A"}}}
	to := &model.Scan{ID: 2, HostID: 1, Status: "ready", OverallGrade: "A", Endpoints: []model.ScanEndpoint{{IPAddress: "1.1.1.1", Grade: "A"}}}

	fromRaw := []byte(`{"endpoints":[{"ipAddress":"1.1.1.1","grade":"A","details":{"protocols":[{"name":"TLS","version":"1.2"},{"name":"TLS","version":"1.0"}],"heartbleed":false}}]}`)
	toRaw := []byte(`{"endpoints":[{"ipAddress":"1.1.1.1","grade":"A","details":{"protocols":[{"name":"TLS","version":"1.2"},{"name":"TLS","version":"1.3"}],"heartbleed":true}}]}`)

	d := Compare(from, to, fromRaw, toRaw)
	if len(d.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint diff, got %d", len(d.Endpoints))
	}
	ep := d.Endpoints[0]
	if !contains(ep.ProtocolsAdded, "TLS 1.3") {
		t.Errorf("expected TLS 1.3 added, got %v", ep.ProtocolsAdded)
	}
	if !contains(ep.ProtocolsRemoved, "TLS 1.0") {
		t.Errorf("expected TLS 1.0 removed, got %v", ep.ProtocolsRemoved)
	}
	if !contains(ep.VulnsAdded, "Heartbleed") {
		t.Errorf("expected Heartbleed added, got %v", ep.VulnsAdded)
	}
	if ep.Change != "changed" {
		t.Errorf("endpoint should be changed")
	}
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
