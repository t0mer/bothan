package scanner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/t0mer/bothan/internal/config"
	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/settings"
	"github.com/t0mer/bothan/internal/ssllabs"
	"github.com/t0mer/bothan/internal/store"
)

// fakeAnalyzer returns a scripted sequence of analyze responses.
type fakeAnalyzer struct {
	mu        sync.Mutex
	responses []*ssllabs.Host
	errs      []error
	i         int
	info      *ssllabs.Info
}

func (f *fakeAnalyzer) Info(context.Context) (*ssllabs.Info, error) {
	if f.info != nil {
		return f.info, nil
	}
	return &ssllabs.Info{MaxAssessments: 25, CurrentAssessments: 0}, nil
}

func (f *fakeAnalyzer) AnalyzeRaw(context.Context, ssllabs.AnalyzeParams) (*ssllabs.Host, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := f.i
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	f.i++
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, nil, f.errs[idx]
	}
	h := f.responses[idx]
	return h, []byte(`{"host":"` + h.Host + `"}`), nil
}

func newTestService(t *testing.T, a Analyzer) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc, err := settings.New(context.Background(), st.Settings(), config.Bootstrap{})
	if err != nil {
		t.Fatal(err)
	}
	// fast poll + no real waits
	svc.Update(context.Background(), map[string]string{
		settings.KeySSLLabsPollInterval: "10ms",
		settings.KeySSLLabsAPIVersion:   "v3", // avoid email requirement
	})
	bo := Backoff{RateLimited: time.Millisecond, Unavailable: time.Millisecond, Overloaded: time.Millisecond, Max: time.Millisecond, ServerErrorRetries: 3}
	sc := New(Options{
		Store:    st,
		Settings: svc,
		Factory:  func(*settings.Settings) (Analyzer, error) { return a, nil },
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backoff:  &bo,
	})
	return sc, st
}

func seedHost(t *testing.T, st *store.Store) *model.Host {
	t.Helper()
	h := &model.Host{Hostname: "example.com", Enabled: true}
	if err := st.Hosts().Create(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	return h
}

func waitScan(t *testing.T, svc *Service, st *store.Store, scanID int64) *model.Scan {
	t.Helper()
	svc.Wait()
	sc, err := st.Scans().Get(context.Background(), scanID)
	if err != nil {
		t.Fatalf("get scan: %v", err)
	}
	return sc
}

func TestTrigger_PollsToReady(t *testing.T) {
	inProgress := &ssllabs.Host{Host: "example.com", Status: "IN_PROGRESS"}
	ready := &ssllabs.Host{
		Host: "example.com", Status: "READY", EngineVersion: "2.3.0", CriteriaVersion: "2009",
		Endpoints: []ssllabs.Endpoint{
			{IPAddress: "1.1.1.1", Grade: "A+", StatusMessage: "Ready", Progress: 100},
			{IPAddress: "2.2.2.2", Grade: "B", StatusMessage: "Ready", Progress: 100},
		},
		Certs: []ssllabs.Cert{{ID: "c1", NotAfter: time.Now().Add(48 * time.Hour).UnixMilli()}},
	}
	a := &fakeAnalyzer{responses: []*ssllabs.Host{inProgress, inProgress, ready}}
	svc, st := newTestService(t, a)
	h := seedHost(t, st)

	sc, err := svc.Trigger(context.Background(), h.ID, model.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	got := waitScan(t, svc, st, sc.ID)

	if got.Status != model.ScanStatusReady {
		t.Fatalf("status = %q, want ready (err=%q)", got.Status, got.ErrorMessage)
	}
	if got.OverallGrade != "B" {
		t.Errorf("overall grade = %q, want B (lowest)", got.OverallGrade)
	}
	if len(got.Endpoints) != 2 {
		t.Errorf("endpoints = %d, want 2", len(got.Endpoints))
	}
	if got.Endpoints[0].CertNotAfter == nil {
		t.Errorf("cert_not_after not set")
	}
}

func TestTrigger_AssessmentError(t *testing.T) {
	errHost := &ssllabs.Host{Host: "x", Status: "ERROR", StatusMessage: "Unable to connect"}
	a := &fakeAnalyzer{responses: []*ssllabs.Host{errHost}}
	svc, st := newTestService(t, a)
	h := seedHost(t, st)

	sc, _ := svc.Trigger(context.Background(), h.ID, model.TriggerManual)
	got := waitScan(t, svc, st, sc.ID)
	if got.Status != model.ScanStatusError {
		t.Fatalf("status = %q, want error", got.Status)
	}
	if got.ErrorMessage == "" {
		t.Errorf("expected error message")
	}
}

func TestTrigger_RetriesOnRateLimit(t *testing.T) {
	ready := &ssllabs.Host{Host: "x", Status: "READY", Endpoints: []ssllabs.Endpoint{{Grade: "A", StatusMessage: "Ready"}}}
	a := &fakeAnalyzer{
		responses: []*ssllabs.Host{nil, ready},
		errs:      []error{&ssllabs.APIError{StatusCode: 429}, nil},
	}
	svc, st := newTestService(t, a)
	h := seedHost(t, st)

	sc, _ := svc.Trigger(context.Background(), h.ID, model.TriggerManual)
	got := waitScan(t, svc, st, sc.ID)
	if got.Status != model.ScanStatusReady {
		t.Fatalf("status = %q, want ready after retry", got.Status)
	}
}

func TestTrigger_DedupInProgress(t *testing.T) {
	// A response that never becomes ready keeps the first scan running.
	stuck := &ssllabs.Host{Host: "x", Status: "IN_PROGRESS"}
	a := &fakeAnalyzer{responses: []*ssllabs.Host{stuck}}
	svc, st := newTestService(t, a)
	h := seedHost(t, st)

	if _, err := svc.Trigger(context.Background(), h.ID, model.TriggerManual); err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	// Give the goroutine a moment to mark the scan running.
	time.Sleep(30 * time.Millisecond)
	_, err := svc.Trigger(context.Background(), h.ID, model.TriggerManual)
	if !errors.Is(err, ErrScanInProgress) {
		t.Errorf("second trigger err = %v, want ErrScanInProgress", err)
	}
}

func TestTrigger_HostNotFound(t *testing.T) {
	a := &fakeAnalyzer{responses: []*ssllabs.Host{{Status: "READY"}}}
	svc, _ := newTestService(t, a)
	_, err := svc.Trigger(context.Background(), 999, model.TriggerManual)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDefaultFactory_V4RequiresEmail(t *testing.T) {
	f := DefaultFactory("", nil)
	_, err := f(&settings.Settings{SSLLabs: settings.SSLLabsSettings{APIVersion: "v4", Email: ""}})
	if !errors.Is(err, ErrNoEmail) {
		t.Errorf("err = %v, want ErrNoEmail", err)
	}
	if _, err := f(&settings.Settings{SSLLabs: settings.SSLLabsSettings{APIVersion: "v4", Email: "a@b.com"}}); err != nil {
		t.Errorf("unexpected err with email: %v", err)
	}
}
