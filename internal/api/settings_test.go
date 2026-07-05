package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/config"
	"github.com/t0mer/bothan/internal/settings"
)

type memRepo struct{ data map[string]string }

func (m *memRepo) GetAll(context.Context) (map[string]string, error) { return m.data, nil }
func (m *memRepo) SeedDefaults(_ context.Context, d map[string]string) error {
	for k, v := range d {
		if _, ok := m.data[k]; !ok {
			m.data[k] = v
		}
	}
	return nil
}
func (m *memRepo) SetMany(_ context.Context, v map[string]string) error {
	for k, val := range v {
		m.data[k] = val
	}
	return nil
}

func newSettingsRouter(t *testing.T, bs config.Bootstrap) http.Handler {
	t.Helper()
	svc, err := settings.New(context.Background(), &memRepo{data: map[string]string{}}, bs)
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	r := chi.NewRouter()
	r.Route("/settings", NewSettings(svc).Routes)
	return r
}

func TestSettings_Get(t *testing.T) {
	h := newSettingsRouter(t, config.Bootstrap{DatabasePath: "/data/bothan.db", ServerPort: 9000, ServerPortSet: true})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.SSLLabs.APIVersion != "v4" || resp.SSLLabs.PollInterval != "10s" {
		t.Errorf("defaults wrong: %+v", resp.SSLLabs)
	}
	if resp.Bootstrap.DatabasePath != "/data/bothan.db" || resp.Bootstrap.EncryptionKeySet {
		t.Errorf("bootstrap DTO wrong: %+v", resp.Bootstrap)
	}
	if len(resp.Server.EnvOverridden) != 1 || resp.Server.EnvOverridden[0] != "port" {
		t.Errorf("env overridden = %v, want [port]", resp.Server.EnvOverridden)
	}
}

func TestSettings_UpdatePartial(t *testing.T) {
	h := newSettingsRouter(t, config.Bootstrap{})
	body := `{"ssllabs":{"email":"ops@example.com","max_workers":2},"log":{"level":"debug"}}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	var resp settingsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.SSLLabs.Email != "ops@example.com" || resp.SSLLabs.MaxWorkers != 2 {
		t.Errorf("ssllabs not updated: %+v", resp.SSLLabs)
	}
	if resp.Log.Level != "debug" {
		t.Errorf("log level = %q, want debug", resp.Log.Level)
	}
}

func TestSettings_UpdateInvalid(t *testing.T) {
	h := newSettingsRouter(t, config.Bootstrap{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(`{"ssllabs":{"api_version":"v9"}}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSettings_UpdateUnknownField(t *testing.T) {
	h := newSettingsRouter(t, config.Bootstrap{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(`{"bogus":true}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
