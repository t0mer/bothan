package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t0mer/bothan/internal/config"
	"github.com/t0mer/bothan/internal/metrics"
	"github.com/t0mer/bothan/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "bothan.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{}
	cfg.Metrics.Enabled = true
	cfg.Server.BasePath = "/"

	h, err := New(Deps{
		Config:  cfg,
		Store:   st,
		Metrics: metrics.New(),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	return h
}

func get(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestHealthz(t *testing.T) {
	resp := get(t, testHandler(t), "/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestReadyz(t *testing.T) {
	resp := get(t, testHandler(t), "/readyz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMetrics(t *testing.T) {
	resp := get(t, testHandler(t), "/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "go_goroutines") {
		t.Errorf("metrics output missing go collector series")
	}
}

func TestAPINotFoundEnvelope(t *testing.T) {
	resp := get(t, testHandler(t), "/api/v1/nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want not_found", env.Error.Code)
	}
}

func TestSPAFallback(t *testing.T) {
	// An unknown, non-API path should serve the SPA index shell.
	resp := get(t, testHandler(t), "/hosts/123")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<div id=\"root\">") {
		t.Errorf("SPA fallback did not serve index.html")
	}
}
