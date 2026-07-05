package store

import (
	"path/filepath"
	"testing"
)

func TestOpen_CreatesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bothan.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	want := []string{
		"hosts", "schedules", "host_schedules", "channels", "host_channels",
		"rules", "scans", "scan_endpoints", "users", "api_tokens",
		"schema_migrations",
	}
	for _, table := range want {
		var name string
		err := s.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not created: %v", table, err)
		}
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bothan.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	// Re-opening the same file re-runs migrate(); it must not fail or reapply.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()

	var applied int
	if err := s2.DB().QueryRow(
		`SELECT COUNT(1) FROM schema_migrations`,
	).Scan(&applied); err != nil {
		t.Fatalf("counting migrations: %v", err)
	}
	if applied != 1 {
		t.Errorf("schema_migrations rows = %d, want 1", applied)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bothan.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// scans.host_id references hosts(id); inserting an orphan must be rejected.
	_, err = s.DB().Exec(
		`INSERT INTO scans(host_id, status, trigger) VALUES (999, 'pending', 'manual')`,
	)
	if err == nil {
		t.Errorf("expected foreign key violation, got nil")
	}
}
