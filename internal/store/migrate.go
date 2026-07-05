package store

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/t0mer/bothan/migrations"
)

// migrate applies every embedded migration that has not yet been recorded,
// each within its own transaction. It is idempotent and safe to call on every
// startup.
func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return fmt.Errorf("listing migrations: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")

		var count int
		if err := s.db.QueryRow(
			`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version,
		).Scan(&count); err != nil {
			return fmt.Errorf("checking migration %q: %w", version, err)
		}
		if count > 0 {
			continue
		}

		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("reading migration %q: %w", name, err)
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration %q: %w", version, err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("applying migration %q: %w", version, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations(version) VALUES (?)`, version,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration %q: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %q: %w", version, err)
		}
	}
	return nil
}
