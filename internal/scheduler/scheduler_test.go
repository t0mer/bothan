package scheduler

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

func TestNormalizeSpec(t *testing.T) {
	ok := map[string]string{
		"Everyday":  "@daily",
		"daily":     "@daily",
		"Hourly":    "@hourly",
		"Weekly":    "@weekly",
		"Monthly":   "@monthly",
		"0 3 * * *": "0 3 * * *",
		"@daily":    "@daily",
	}
	for in, want := range ok {
		got, err := NormalizeSpec(in)
		if err != nil {
			t.Errorf("NormalizeSpec(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeSpec(%q) = %q, want %q", in, got, want)
		}
	}

	for _, bad := range []string{"", "   ", "not a cron", "99 99 * * *"} {
		if _, err := NormalizeSpec(bad); err == nil {
			t.Errorf("NormalizeSpec(%q) should have errored", bad)
		}
	}
}

type fakeEnqueuer struct {
	mu       sync.Mutex
	triggers []int64
}

func (f *fakeEnqueuer) Trigger(_ context.Context, hostID int64, _ string) (*model.Scan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggers = append(f.triggers, hostID)
	return &model.Scan{HostID: hostID}, nil
}

func TestFire_EnqueuesEnabledLinkedHosts(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	// Two hosts: one enabled, one disabled; both linked to the schedule.
	enabled := &model.Host{Hostname: "on.com", Enabled: true}
	disabled := &model.Host{Hostname: "off.com", Enabled: false}
	st.Hosts().Create(ctx, enabled)
	st.Hosts().Create(ctx, disabled)

	sched := &model.Schedule{Name: "nightly", Spec: "@daily", Enabled: true}
	if err := st.Schedules().Create(ctx, sched); err != nil {
		t.Fatal(err)
	}
	if err := st.Schedules().SetHostSchedules(ctx, enabled.ID, []int64{sched.ID}); err != nil {
		t.Fatal(err)
	}
	if err := st.Schedules().SetHostSchedules(ctx, disabled.ID, []int64{sched.ID}); err != nil {
		t.Fatal(err)
	}

	enq := &fakeEnqueuer{}
	svc := New(st, enq, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.fire(*sched)

	if len(enq.triggers) != 1 || enq.triggers[0] != enabled.ID {
		t.Errorf("triggers = %v, want only enabled host id %d", enq.triggers, enabled.ID)
	}
}

func TestRebuild_NoSchedules(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, &fakeEnqueuer{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := svc.Rebuild(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	svc.Stop()
}
