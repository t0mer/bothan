package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/t0mer/bothan/internal/model"
)

// HostRepo provides CRUD access to the hosts table.
type HostRepo struct {
	db *sql.DB
}

// Hosts returns a repository bound to this store's database.
func (s *Store) Hosts() *HostRepo { return &HostRepo{db: s.db} }

const hostColumns = `id, hostname, enabled, publish, ignore_mismatch,
	from_cache, max_age_hours, COALESCE(notes, ''), created_at, updated_at`

// Create inserts h and populates its ID and timestamps. It returns ErrConflict
// if the hostname already exists.
func (r *HostRepo) Create(ctx context.Context, h *model.Host) error {
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO hosts (hostname, enabled, publish, ignore_mismatch, from_cache, max_age_hours, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`,
		h.Hostname, boolToInt(h.Enabled), boolToInt(h.Publish), boolToInt(h.IgnoreMismatch),
		boolToInt(h.FromCache), nullableInt(h.MaxAgeHours), h.Notes,
	).Scan(&h.ID, &createdAt, &updatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("host %q: %w", h.Hostname, ErrConflict)
		}
		return fmt.Errorf("inserting host: %w", err)
	}
	h.CreatedAt = parseTime(createdAt)
	h.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Get returns the host with the given id, or ErrNotFound.
func (r *HostRepo) Get(ctx context.Context, id int64) (*model.Host, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+hostColumns+` FROM hosts WHERE id = ?`, id)
	h, err := scanHost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting host %d: %w", id, err)
	}
	return h, nil
}

// List returns all hosts ordered by hostname.
func (r *HostRepo) List(ctx context.Context) ([]model.Host, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+hostColumns+` FROM hosts ORDER BY hostname`)
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}
	defer rows.Close()

	hosts := []model.Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning host: %w", err)
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

// Update writes the mutable fields of h and refreshes updated_at. It returns
// ErrNotFound if the host does not exist, or ErrConflict on a hostname clash.
func (r *HostRepo) Update(ctx context.Context, h *model.Host) error {
	var updatedAt string
	err := r.db.QueryRowContext(ctx, `
		UPDATE hosts SET
			hostname = ?, enabled = ?, publish = ?, ignore_mismatch = ?,
			from_cache = ?, max_age_hours = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		RETURNING updated_at`,
		h.Hostname, boolToInt(h.Enabled), boolToInt(h.Publish), boolToInt(h.IgnoreMismatch),
		boolToInt(h.FromCache), nullableInt(h.MaxAgeHours), h.Notes, h.ID,
	).Scan(&updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("host %q: %w", h.Hostname, ErrConflict)
		}
		return fmt.Errorf("updating host %d: %w", h.ID, err)
	}
	h.UpdatedAt = parseTime(updatedAt)
	return nil
}

// SetEnabled toggles a host's enabled flag. Returns ErrNotFound if absent.
func (r *HostRepo) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE hosts SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("setting host %d enabled: %w", id, err)
	}
	return requireOneRow(res)
}

// Delete removes a host (cascading to its scans, links, and rules). Returns
// ErrNotFound if absent.
func (r *HostRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting host %d: %w", id, err)
	}
	return requireOneRow(res)
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanHost(s rowScanner) (*model.Host, error) {
	var (
		h                                           model.Host
		enabled, publish, ignoreMismatch, fromCache int
		maxAge                                      sql.NullInt64
		createdAt, updatedAt                        string
	)
	if err := s.Scan(&h.ID, &h.Hostname, &enabled, &publish, &ignoreMismatch,
		&fromCache, &maxAge, &h.Notes, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	h.Enabled = enabled != 0
	h.Publish = publish != 0
	h.IgnoreMismatch = ignoreMismatch != 0
	h.FromCache = fromCache != 0
	if maxAge.Valid {
		v := int(maxAge.Int64)
		h.MaxAgeHours = &v
	}
	h.CreatedAt = parseTime(createdAt)
	h.UpdatedAt = parseTime(updatedAt)
	return &h, nil
}

func requireOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// parseTime parses SQLite's CURRENT_TIMESTAMP text ("2006-01-02 15:04:05", UTC),
// falling back to RFC3339. A zero time is returned if neither layout matches.
func parseTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
