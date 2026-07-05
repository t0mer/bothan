package settings

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/t0mer/bothan/internal/config"
)

type memRepo struct{ data map[string]string }

func newMemRepo() *memRepo { return &memRepo{data: map[string]string{}} }

func (m *memRepo) GetAll(_ context.Context) (map[string]string, error) {
	return maps.Clone(m.data), nil
}
func (m *memRepo) SeedDefaults(_ context.Context, defaults map[string]string) error {
	for k, v := range defaults {
		if _, ok := m.data[k]; !ok {
			m.data[k] = v
		}
	}
	return nil
}
func (m *memRepo) SetMany(_ context.Context, values map[string]string) error {
	for k, v := range values {
		m.data[k] = v
	}
	return nil
}

func newService(t *testing.T, bs config.Bootstrap) *Service {
	t.Helper()
	svc, err := New(context.Background(), newMemRepo(), bs)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

func TestNew_SeedsDefaults(t *testing.T) {
	svc := newService(t, config.Bootstrap{})
	cur := svc.Current()
	if cur.Server.Port != 8080 || cur.SSLLabs.APIVersion != "v4" {
		t.Errorf("defaults not applied: %+v", cur)
	}
	if cur.SSLLabs.PollInterval != 10*time.Second {
		t.Errorf("poll interval = %s, want 10s", cur.SSLLabs.PollInterval)
	}
}

func TestUpdate_PersistsAndRefreshes(t *testing.T) {
	svc := newService(t, config.Bootstrap{})
	err := svc.Update(context.Background(), map[string]string{
		KeySSLLabsEmail:        "ops@example.com",
		KeySSLLabsMaxWorkers:   "3",
		KeySSLLabsPollInterval: "30s",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	cur := svc.Current()
	if cur.SSLLabs.Email != "ops@example.com" || cur.SSLLabs.MaxWorkers != 3 {
		t.Errorf("update not reflected: %+v", cur.SSLLabs)
	}
	if cur.SSLLabs.PollInterval != 30*time.Second {
		t.Errorf("poll interval = %s, want 30s", cur.SSLLabs.PollInterval)
	}
}

func TestUpdate_RejectsInvalidAtomically(t *testing.T) {
	svc := newService(t, config.Bootstrap{})
	err := svc.Update(context.Background(), map[string]string{
		KeySSLLabsEmail:      "keepme@example.com",
		KeySSLLabsAPIVersion: "v9", // invalid
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Nothing should have been applied.
	if svc.Current().SSLLabs.Email == "keepme@example.com" {
		t.Error("invalid update was partially applied")
	}
}

func TestOnChange_Fires(t *testing.T) {
	svc := newService(t, config.Bootstrap{})
	var got string
	svc.OnChange(func(s *Settings) { got = s.Log.Level })
	if err := svc.Update(context.Background(), map[string]string{KeyLogLevel: "debug"}); err != nil {
		t.Fatal(err)
	}
	if got != "debug" {
		t.Errorf("onChange level = %q, want debug", got)
	}
}

func TestEffectiveBind_EnvOverride(t *testing.T) {
	svc := newService(t, config.Bootstrap{ServerPort: 9999, ServerPortSet: true})
	host, port := svc.EffectiveBind()
	if host != "0.0.0.0" || port != 9999 {
		t.Errorf("bind = %s:%d, want 0.0.0.0:9999", host, port)
	}
	if ov := svc.EnvOverriddenBind(); len(ov) != 1 || ov[0] != "port" {
		t.Errorf("overridden = %v, want [port]", ov)
	}
}
