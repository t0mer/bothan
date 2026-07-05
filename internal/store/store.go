// Package store provides SQLite persistence for Bothan using the pure-Go
// modernc driver (CGO-free). It opens the database, applies embedded
// migrations, and exposes the underlying *sql.DB to repositories.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store owns the database handle and its lifecycle.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path, applies pragmas,
// and runs all pending migrations.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %q: %w", path, err)
	}
	// SQLite is a single-writer store; serialising connections avoids
	// "database is locked" under concurrent writes at Bothan's scale.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging sqlite %q: %w", path, err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB returns the underlying database handle for repositories.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }
