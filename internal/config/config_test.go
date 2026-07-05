package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// newFlags builds a flag set with Bothan's flags registered, as main() would.
func newFlags(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	return fs
}

func TestLoad_Defaults(t *testing.T) {
	fs := newFlags(t)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(fs)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("server.host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("server.port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.SSLLabs.APIVersion != "v4" {
		t.Errorf("ssllabs.api_version = %q, want v4", cfg.SSLLabs.APIVersion)
	}
	if cfg.SSLLabs.PollInterval != 10*time.Second {
		t.Errorf("poll_interval = %s, want 10s", cfg.SSLLabs.PollInterval)
	}
	if cfg.SSLLabs.ScanTimeout != 20*time.Minute {
		t.Errorf("scan_timeout = %s, want 20m", cfg.SSLLabs.ScanTimeout)
	}
	if cfg.SSLLabs.MaxWorkers != 5 {
		t.Errorf("max_workers = %d, want 5", cfg.SSLLabs.MaxWorkers)
	}
	if cfg.Log.Level != "info" || cfg.Log.Format != "json" {
		t.Errorf("log = %+v, want level=info format=json", cfg.Log)
	}
	if !cfg.Metrics.Enabled {
		t.Errorf("metrics.enabled = false, want true")
	}
	if cfg.Auth.Enabled {
		t.Errorf("auth.enabled = true, want false")
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("server:\n  port: 9000\nssllabs:\n  email: yaml@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BOTHAN_SERVER_PORT", "9999")
	t.Setenv("BOTHAN_CRYPTO_ENCRYPTION_KEY", "envkey")

	fs := newFlags(t)
	if err := fs.Parse([]string{"--config", yamlPath}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(fs)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("server.port = %d, want 9999 (env over yaml)", cfg.Server.Port)
	}
	if cfg.SSLLabs.Email != "yaml@example.com" {
		t.Errorf("ssllabs.email = %q, want yaml value", cfg.SSLLabs.Email)
	}
	if cfg.Crypto.EncryptionKey != "envkey" {
		t.Errorf("crypto.encryption_key = %q, want envkey", cfg.Crypto.EncryptionKey)
	}
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	t.Setenv("BOTHAN_SERVER_PORT", "9999")

	fs := newFlags(t)
	if err := fs.Parse([]string{"--port", "7070"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(fs)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Port != 7070 {
		t.Errorf("server.port = %d, want 7070 (flag over env)", cfg.Server.Port)
	}
}

func TestLoad_ValidationRejectsBadValues(t *testing.T) {
	cases := map[string][]string{
		"bad api version": {"--config", writeYAML(t, "ssllabs:\n  api_version: v5\n")},
		"bad log level":   {"--config", writeYAML(t, "log:\n  level: verbose\n")},
		"bad port":        {"--port", "70000"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			fs := newFlags(t)
			if err := fs.Parse(args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if _, err := Load(fs); err == nil {
				t.Errorf("expected validation error, got nil")
			}
		})
	}
}

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
