package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/t0mer/bothan/internal/crypto"
	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

func newCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	b := make([]byte, 32)
	rand.Read(b)
	c, err := crypto.New(hex.EncodeToString(b))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// captureDispatcher records sends instead of hitting the network.
type captureDispatcher struct {
	mu       sync.Mutex
	messages []string
}

func newCaptureEngine(t *testing.T, cap *captureDispatcher) (*Engine, *store.Store, *crypto.Cipher) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cipher := newCipher(t)
	e := NewEngine(EngineOptions{
		Store:  st,
		Cipher: cipher,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	// Swap in a dispatcher backed by a test HTTP client that records via a hook.
	e.dispatcher = &Dispatcher{http: nil}
	e.sendHook = func(ct string, cfg []byte, msg string) error {
		cap.mu.Lock()
		defer cap.mu.Unlock()
		cap.messages = append(cap.messages, msg)
		return nil
	}
	return e, st, cipher
}

// seed a host with an enabled channel linked.
func seedHostChannel(t *testing.T, st *store.Store, cipher *crypto.Cipher) *model.Host {
	t.Helper()
	ctx := context.Background()
	h := &model.Host{Hostname: "example.com", Enabled: true}
	st.Hosts().Create(ctx, h)
	enc, _ := cipher.Encrypt([]byte(`{"url":"generic://localhost"}`))
	ch := &model.Channel{Name: "ops", Type: model.ChannelShoutrrr, ConfigEncrypted: enc, Enabled: true}
	st.Channels().Create(ctx, ch)
	st.Channels().SetHostChannels(ctx, h.ID, []int64{ch.ID})
	return h
}

func addRule(t *testing.T, st *store.Store, hostID *int64, cond, threshold string) {
	t.Helper()
	r := &model.Rule{HostID: hostID, Name: cond, ConditionType: cond, ThresholdGrade: threshold, Enabled: true}
	if err := st.Rules().Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
}

func readyScan(t *testing.T, st *store.Store, hostID int64, grade string) *model.Scan {
	t.Helper()
	ctx := context.Background()
	sc := &model.Scan{HostID: hostID, Status: model.ScanStatusPending, Trigger: "manual"}
	st.Scans().Create(ctx, sc)
	sc.Status = model.ScanStatusReady
	sc.OverallGrade = grade
	sc.Endpoints = []model.ScanEndpoint{{IPAddress: "1.2.3.4", Grade: grade}}
	if err := st.Scans().SaveResult(ctx, sc, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	full, _ := st.Scans().Get(ctx, sc.ID)
	return full
}

func TestEngine_GradeBelowFiresAndSuppresses(t *testing.T) {
	cap := &captureDispatcher{}
	e, st, cipher := newCaptureEngine(t, cap)
	h := seedHostChannel(t, st, cipher)
	addRule(t, st, &h.ID, model.CondGradeBelow, "A")

	// First B scan: below A → fires.
	s1 := readyScan(t, st, h.ID, "B")
	e.Evaluate(context.Background(), s1)
	if len(cap.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cap.messages))
	}

	// Second B scan (unchanged failing grade) → suppressed.
	s2 := readyScan(t, st, h.ID, "B")
	e.Evaluate(context.Background(), s2)
	if len(cap.messages) != 1 {
		t.Errorf("unchanged grade should be suppressed, got %d messages", len(cap.messages))
	}

	// Drop to C (changed) → fires again.
	s3 := readyScan(t, st, h.ID, "C")
	e.Evaluate(context.Background(), s3)
	if len(cap.messages) != 2 {
		t.Errorf("changed grade should re-alert, got %d messages", len(cap.messages))
	}
}

func TestEngine_ScanFailed(t *testing.T) {
	cap := &captureDispatcher{}
	e, st, cipher := newCaptureEngine(t, cap)
	h := seedHostChannel(t, st, cipher)
	addRule(t, st, nil, model.CondScanFailed, "") // global rule

	ctx := context.Background()
	sc := &model.Scan{HostID: h.ID, Status: model.ScanStatusPending, Trigger: "manual"}
	st.Scans().Create(ctx, sc)
	st.Scans().Fail(ctx, sc.ID, "connection refused")
	full, _ := st.Scans().Get(ctx, sc.ID)

	e.Evaluate(ctx, full)
	if len(cap.messages) != 1 {
		t.Errorf("scan_failed rule should fire, got %d messages", len(cap.messages))
	}
}

func TestEngine_CertExpiry(t *testing.T) {
	cap := &captureDispatcher{}
	e, st, cipher := newCaptureEngine(t, cap)
	h := seedHostChannel(t, st, cipher)
	days := 30
	r := &model.Rule{HostID: &h.ID, Name: "cert", ConditionType: model.CondCertExpiry, ExpiryDays: &days, Enabled: true}
	st.Rules().Create(context.Background(), r)

	ctx := context.Background()
	sc := &model.Scan{HostID: h.ID, Status: model.ScanStatusPending, Trigger: "manual"}
	st.Scans().Create(ctx, sc)
	soon := time.Now().UTC().Add(10 * 24 * time.Hour)
	sc.Status = model.ScanStatusReady
	sc.OverallGrade = "A"
	sc.Endpoints = []model.ScanEndpoint{{IPAddress: "1.2.3.4", Grade: "A", CertNotAfter: &soon}}
	st.Scans().SaveResult(ctx, sc, []byte(`{}`))
	full, _ := st.Scans().Get(ctx, sc.ID)

	e.Evaluate(ctx, full)
	if len(cap.messages) != 1 {
		t.Errorf("cert_expiry rule should fire for cert expiring in 10 days, got %d", len(cap.messages))
	}
}

func TestEngine_NoChannelsNoSend(t *testing.T) {
	cap := &captureDispatcher{}
	e, st, cipher := newCaptureEngine(t, cap)
	_ = cipher
	h := &model.Host{Hostname: "nochan.com", Enabled: true}
	st.Hosts().Create(context.Background(), h)
	addRule(t, st, &h.ID, model.CondScanCompleted, "")

	s := readyScan(t, st, h.ID, "A")
	e.Evaluate(context.Background(), s)
	if len(cap.messages) != 0 {
		t.Errorf("no channels → no messages, got %d", len(cap.messages))
	}
}
