package bundle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
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

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// seed a store with a host, schedule, channel, rule, and links.
func seed(t *testing.T, st *store.Store, cipher *crypto.Cipher) {
	t.Helper()
	ctx := context.Background()
	h := &model.Host{Hostname: "example.com", Enabled: true, Publish: true, Notes: "primary"}
	st.Hosts().Create(ctx, h)
	sc := &model.Schedule{Name: "nightly", Spec: "@daily", Enabled: true}
	st.Schedules().Create(ctx, sc)
	enc, _ := cipher.Encrypt([]byte(`{"url":"slack://tok@ops"}`))
	ch := &model.Channel{Name: "ops", Type: model.ChannelShoutrrr, ConfigEncrypted: enc, Enabled: true}
	st.Channels().Create(ctx, ch)
	rule := &model.Rule{HostID: &h.ID, Name: "below-A", ConditionType: model.CondGradeBelow, ThresholdGrade: "A", Enabled: true}
	st.Rules().Create(ctx, rule)
	st.Schedules().SetHostSchedules(ctx, h.ID, []int64{sc.ID})
	st.Channels().SetHostChannels(ctx, h.ID, []int64{ch.ID})
}

func TestExportImport_InstanceKeyRoundTrip(t *testing.T) {
	ctx := context.Background()
	cipher := newCipher(t)
	src := newStore(t)
	seed(t, src, cipher)

	b, err := Export(ctx, src, cipher, ExportOptions{Mode: SecretInstanceKey, AppVersion: "test", Now: time.Now()})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if b.KeyFingerprint != cipher.Fingerprint() || len(b.Channels) != 1 || b.Channels[0].Ciphertext == "" {
		t.Fatalf("bundle missing instance_key secrets: %+v", b)
	}

	// Import into a fresh store with the SAME key.
	dst := newStore(t)
	rep, err := Import(ctx, dst, cipher, b, ImportOptions{Mode: store.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if rep.HostsCreated != 1 || rep.SchedulesCreated != 1 || rep.ChannelsCreated != 1 || rep.RulesCreated != 1 {
		t.Errorf("import counts wrong: %+v", rep)
	}
	if rep.LinksCreated != 2 {
		t.Errorf("links created = %d, want 2", rep.LinksCreated)
	}

	// The channel config must decrypt with the same key.
	ch, _ := dst.Channels().List(ctx)
	if len(ch) != 1 {
		t.Fatalf("channel not imported")
	}
	plain, err := cipher.Decrypt(ch[0].ConfigEncrypted)
	if err != nil || string(plain) != `{"url":"slack://tok@ops"}` {
		t.Errorf("channel config did not survive round-trip: %q err=%v", plain, err)
	}
}

func TestImport_InstanceKeyFingerprintMismatch(t *testing.T) {
	ctx := context.Background()
	cipher := newCipher(t)
	src := newStore(t)
	seed(t, src, cipher)
	b, _ := Export(ctx, src, cipher, ExportOptions{Mode: SecretInstanceKey, Now: time.Now()})

	other := newCipher(t) // different key
	dst := newStore(t)
	_, err := Import(ctx, dst, other, b, ImportOptions{Mode: store.ImportMerge})
	if err == nil {
		t.Fatal("expected fingerprint mismatch error")
	}
}

func TestExportImport_PassphraseAcrossKeys(t *testing.T) {
	ctx := context.Background()
	srcKey := newCipher(t)
	src := newStore(t)
	seed(t, src, srcKey)

	pass := "correct horse battery staple"
	b, err := Export(ctx, src, srcKey, ExportOptions{Mode: SecretPassphrase, Passphrase: pass, Now: time.Now()})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if b.KDF == nil || b.Channels[0].Ciphertext == "" {
		t.Fatalf("passphrase bundle missing kdf/ciphertext")
	}

	// Import into a store with a DIFFERENT instance key, using the passphrase.
	dstKey := newCipher(t)
	dst := newStore(t)
	if _, err := Import(ctx, dst, dstKey, b, ImportOptions{Mode: store.ImportMerge, Passphrase: pass}); err != nil {
		t.Fatalf("import: %v", err)
	}
	ch, _ := dst.Channels().List(ctx)
	plain, err := dstKey.Decrypt(ch[0].ConfigEncrypted)
	if err != nil || string(plain) != `{"url":"slack://tok@ops"}` {
		t.Errorf("passphrase round-trip failed: %q err=%v", plain, err)
	}

	// Wrong passphrase must fail.
	if _, err := Import(ctx, newStore(t), dstKey, b, ImportOptions{Passphrase: "wrong"}); err == nil {
		t.Error("expected wrong-passphrase error")
	}
}

func TestExport_NoneAndImportNeedsCredentials(t *testing.T) {
	ctx := context.Background()
	cipher := newCipher(t)
	src := newStore(t)
	seed(t, src, cipher)

	b, err := Export(ctx, src, nil, ExportOptions{Mode: SecretNone, Now: time.Now()})
	if err != nil {
		t.Fatalf("export none: %v", err)
	}
	if b.Channels[0].Ciphertext != "" {
		t.Errorf("none mode should not carry ciphertext")
	}
	dst := newStore(t)
	rep, err := Import(ctx, dst, nil, b, ImportOptions{Mode: store.ImportMerge})
	if err != nil {
		t.Fatalf("import none: %v", err)
	}
	if rep.ChannelsNeedingCredentials != 1 {
		t.Errorf("channel should be flagged needs_credentials")
	}
	ch, _ := dst.Channels().List(ctx)
	if ch[0].Enabled || !ch[0].NeedsCredentials {
		t.Errorf("none-imported channel should be disabled + needs_credentials: %+v", ch[0])
	}
}

func TestImport_DryRunAppliesNothing(t *testing.T) {
	ctx := context.Background()
	cipher := newCipher(t)
	src := newStore(t)
	seed(t, src, cipher)
	b, _ := Export(ctx, src, cipher, ExportOptions{Mode: SecretInstanceKey, Now: time.Now()})

	dst := newStore(t)
	rep, err := Import(ctx, dst, cipher, b, ImportOptions{Mode: store.ImportMerge, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if rep.HostsCreated != 1 {
		t.Errorf("dry run should report 1 host created, got %d", rep.HostsCreated)
	}
	hosts, _ := dst.Hosts().List(ctx)
	if len(hosts) != 0 {
		t.Errorf("dry run must not persist: found %d hosts", len(hosts))
	}
}

func TestImport_ReplacePreservesHostsWipesRest(t *testing.T) {
	ctx := context.Background()
	cipher := newCipher(t)
	src := newStore(t)
	seed(t, src, cipher)
	b, _ := Export(ctx, src, cipher, ExportOptions{Mode: SecretInstanceKey, Now: time.Now()})

	// Destination already has an extra schedule that should be wiped by replace.
	dst := newStore(t)
	dst.Schedules().Create(ctx, &model.Schedule{Name: "stale", Spec: "@weekly", Enabled: true})

	if _, err := Import(ctx, dst, cipher, b, ImportOptions{Mode: store.ImportReplace}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	scheds, _ := dst.Schedules().List(ctx)
	if len(scheds) != 1 || scheds[0].Name != "nightly" {
		t.Errorf("replace should leave only bundle schedules: %+v", scheds)
	}
}
