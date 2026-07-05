package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/t0mer/bothan/internal/model"
)

func TestRuleRepo_SetHostRules(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	h := &model.Host{Hostname: "example.com", Enabled: true}
	st.Hosts().Create(ctx, h)

	// A global rule that must NOT be touched by SetHostRules.
	st.Rules().Create(ctx, &model.Rule{Name: "global-fail", ConditionType: model.CondScanFailed, Enabled: true})

	days := 30
	if err := st.Rules().SetHostRules(ctx, h.ID, []model.Rule{
		{HostID: &h.ID, Name: "below", ConditionType: model.CondGradeBelow, ThresholdGrade: "A", Enabled: true},
		{HostID: &h.ID, Name: "cert", ConditionType: model.CondCertExpiry, ExpiryDays: &days, Enabled: true},
	}); err != nil {
		t.Fatalf("set host rules: %v", err)
	}
	if got, _ := st.Rules().ListByHost(ctx, h.ID); len(got) != 2 {
		t.Fatalf("host rules = %d, want 2", len(got))
	}

	// Replacing with a single rule removes the others.
	if err := st.Rules().SetHostRules(ctx, h.ID, []model.Rule{
		{HostID: &h.ID, Name: "only", ConditionType: model.CondVulnDetected, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Rules().ListByHost(ctx, h.ID)
	if len(got) != 1 || got[0].ConditionType != model.CondVulnDetected {
		t.Errorf("after replace: %+v", got)
	}

	// The global rule survived (still applies to the host).
	all, _ := st.Rules().RulesForHost(ctx, h.ID)
	hasGlobal := false
	for _, r := range all {
		if r.HostID == nil && r.ConditionType == model.CondScanFailed {
			hasGlobal = true
		}
	}
	if !hasGlobal {
		t.Error("global rule should be untouched by SetHostRules")
	}

	// Clearing removes all host rules.
	if err := st.Rules().SetHostRules(ctx, h.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Rules().ListByHost(ctx, h.ID); len(got) != 0 {
		t.Errorf("host rules after clear = %d, want 0", len(got))
	}
}
