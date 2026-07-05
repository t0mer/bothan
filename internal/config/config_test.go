package config

import (
	"testing"

	"github.com/spf13/pflag"
)

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
	b, err := Load(fs)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if b.DatabasePath != defaultDatabasePath {
		t.Errorf("db path = %q, want %q", b.DatabasePath, defaultDatabasePath)
	}
	if b.EncryptionKey != "" {
		t.Errorf("encryption key = %q, want empty", b.EncryptionKey)
	}
	if b.ServerHostSet || b.ServerPortSet {
		t.Errorf("server overrides should be unset by default")
	}
}

func TestLoad_EnvBootstrap(t *testing.T) {
	t.Setenv("BOTHAN_DATABASE_PATH", "/var/data/b.db")
	t.Setenv("BOTHAN_CRYPTO_ENCRYPTION_KEY", "secretkey")
	t.Setenv("BOTHAN_SERVER_PORT", "9000")

	fs := newFlags(t)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	b, err := Load(fs)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if b.DatabasePath != "/var/data/b.db" {
		t.Errorf("db path = %q", b.DatabasePath)
	}
	if b.EncryptionKey != "secretkey" {
		t.Errorf("key = %q", b.EncryptionKey)
	}
	if !b.ServerPortSet || b.ServerPort != 9000 {
		t.Errorf("port override = %d set=%v, want 9000 set", b.ServerPort, b.ServerPortSet)
	}
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	t.Setenv("BOTHAN_DATABASE_PATH", "/env/path.db")
	fs := newFlags(t)
	if err := fs.Parse([]string{"--db-path", "/flag/path.db", "--port", "7070"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	b, err := Load(fs)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if b.DatabasePath != "/flag/path.db" {
		t.Errorf("db path = %q, want flag value", b.DatabasePath)
	}
	if !b.ServerPortSet || b.ServerPort != 7070 {
		t.Errorf("port = %d, want 7070", b.ServerPort)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	fs := newFlags(t)
	if err := fs.Parse([]string{"--port", "70000"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := Load(fs); err == nil {
		t.Errorf("expected error for out-of-range port")
	}
}
