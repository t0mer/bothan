package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/t0mer/bothan/internal/model"
)

func TestDashboardSummary_EmptyReturnsNonNilSlices(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sum, err := st.Dashboard().Summary(context.Background(), 30, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Must be non-nil so JSON encodes [] (not null) for the frontend.
	if sum.GradeCounts == nil || sum.CertsExpiringSoon == nil || sum.RecentScans == nil {
		t.Errorf("empty summary has nil slices: %+v", sum)
	}
}

func TestDashboardSummary(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	// Host A: latest ready scan grade B, cert expiring in 10 days.
	a := &model.Host{Hostname: "a.com", Enabled: true}
	st.Hosts().Create(ctx, a)
	scA := &model.Scan{HostID: a.ID, Status: model.ScanStatusPending, Trigger: "manual"}
	st.Scans().Create(ctx, scA)
	soon := time.Now().UTC().Add(10 * 24 * time.Hour)
	scA.Status = model.ScanStatusReady
	scA.OverallGrade = "B"
	scA.Endpoints = []model.ScanEndpoint{{IPAddress: "1.1.1.1", Grade: "B", CertNotAfter: &soon}}
	st.Scans().SaveResult(ctx, scA, []byte(`{}`))

	// Host B: enabled, never scanned.
	b := &model.Host{Hostname: "b.com", Enabled: true}
	st.Hosts().Create(ctx, b)

	// Host C: disabled, ready scan A+.
	c := &model.Host{Hostname: "c.com", Enabled: false}
	st.Hosts().Create(ctx, c)
	scC := &model.Scan{HostID: c.ID, Status: model.ScanStatusPending, Trigger: "manual"}
	st.Scans().Create(ctx, scC)
	scC.Status = model.ScanStatusReady
	scC.OverallGrade = "A+"
	st.Scans().SaveResult(ctx, scC, []byte(`{}`))

	sum, err := st.Dashboard().Summary(ctx, 30, 10)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.TotalHosts != 3 || sum.EnabledHosts != 2 || sum.DisabledHosts != 1 {
		t.Errorf("counts wrong: %+v", sum)
	}
	if sum.NeverScanned != 1 {
		t.Errorf("never scanned = %d, want 1", sum.NeverScanned)
	}

	grades := map[string]int{}
	for _, gc := range sum.GradeCounts {
		grades[gc.Grade] = gc.Count
	}
	if grades["B"] != 1 || grades["A+"] != 1 || grades["none"] != 1 {
		t.Errorf("grade distribution wrong: %+v", sum.GradeCounts)
	}

	// Grade order: A+ before B before none.
	if len(sum.GradeCounts) >= 2 && model.GradeRank(sum.GradeCounts[0].Grade) < model.GradeRank(sum.GradeCounts[1].Grade) {
		t.Errorf("grade counts not in display order: %+v", sum.GradeCounts)
	}

	if len(sum.CertsExpiringSoon) != 1 || sum.CertsExpiringSoon[0].Hostname != "a.com" {
		t.Errorf("cert expiry wrong: %+v", sum.CertsExpiringSoon)
	}
	if len(sum.RecentScans) != 2 {
		t.Errorf("recent scans = %d, want 2", len(sum.RecentScans))
	}
}
