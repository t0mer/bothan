package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/t0mer/bothan/internal/model"
)

// ScheduleRepo provides access to schedules and host_schedules.
type ScheduleRepo struct {
	db *sql.DB
}

// Schedules returns a schedule repository bound to this store's database.
func (s *Store) Schedules() *ScheduleRepo { return &ScheduleRepo{db: s.db} }

const scheduleColumns = `id, name, spec, enabled, created_at, updated_at`

// Create inserts a schedule and populates its ID and timestamps.
func (r *ScheduleRepo) Create(ctx context.Context, s *model.Schedule) error {
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO schedules (name, spec, enabled) VALUES (?, ?, ?)
		RETURNING id, created_at, updated_at`,
		s.Name, s.Spec, boolToInt(s.Enabled),
	).Scan(&s.ID, &createdAt, &updatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("schedule %q: %w", s.Name, ErrConflict)
		}
		return fmt.Errorf("creating schedule: %w", err)
	}
	s.CreatedAt = parseTime(createdAt)
	s.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Get returns a schedule by id, or ErrNotFound.
func (r *ScheduleRepo) Get(ctx context.Context, id int64) (*model.Schedule, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+scheduleColumns+` FROM schedules WHERE id = ?`, id)
	s, err := scanSchedule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting schedule %d: %w", id, err)
	}
	return s, nil
}

// List returns all schedules ordered by name.
func (r *ScheduleRepo) List(ctx context.Context) ([]model.Schedule, error) {
	return r.query(ctx, `SELECT `+scheduleColumns+` FROM schedules ORDER BY name`)
}

// ListEnabled returns only enabled schedules.
func (r *ScheduleRepo) ListEnabled(ctx context.Context) ([]model.Schedule, error) {
	return r.query(ctx, `SELECT `+scheduleColumns+` FROM schedules WHERE enabled = 1 ORDER BY name`)
}

// Update writes a schedule's fields and refreshes updated_at.
func (r *ScheduleRepo) Update(ctx context.Context, s *model.Schedule) error {
	var updatedAt string
	err := r.db.QueryRowContext(ctx, `
		UPDATE schedules SET name = ?, spec = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? RETURNING updated_at`,
		s.Name, s.Spec, boolToInt(s.Enabled), s.ID,
	).Scan(&updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("schedule %q: %w", s.Name, ErrConflict)
		}
		return fmt.Errorf("updating schedule %d: %w", s.ID, err)
	}
	s.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Delete removes a schedule (cascading its host links).
func (r *ScheduleRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting schedule %d: %w", id, err)
	}
	return requireOneRow(res)
}

// SetHostSchedules replaces the set of schedules linked to a host.
func (r *ScheduleRepo) SetHostSchedules(ctx context.Context, hostID int64, scheduleIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM host_schedules WHERE host_id = ?`, hostID); err != nil {
		return fmt.Errorf("clearing host schedules: %w", err)
	}
	for _, sid := range scheduleIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO host_schedules (host_id, schedule_id) VALUES (?, ?)`, hostID, sid); err != nil {
			return fmt.Errorf("linking schedule %d to host %d: %w", sid, hostID, err)
		}
	}
	return tx.Commit()
}

// SchedulesForHost returns the schedules linked to a host.
func (r *ScheduleRepo) SchedulesForHost(ctx context.Context, hostID int64) ([]model.Schedule, error) {
	return r.query(ctx, `
		SELECT s.id, s.name, s.spec, s.enabled, s.created_at, s.updated_at
		FROM schedules s
		JOIN host_schedules hs ON hs.schedule_id = s.id
		WHERE hs.host_id = ? ORDER BY s.name`, hostID)
}

// EnabledHostsForSchedule returns the enabled hosts linked to a schedule.
func (r *ScheduleRepo) EnabledHostsForSchedule(ctx context.Context, scheduleID int64) ([]model.Host, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+hostColumns+` FROM hosts h
		JOIN host_schedules hs ON hs.host_id = h.id
		WHERE hs.schedule_id = ? AND h.enabled = 1 ORDER BY h.hostname`, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("enabled hosts for schedule %d: %w", scheduleID, err)
	}
	defer rows.Close()
	hosts := []model.Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

func (r *ScheduleRepo) query(ctx context.Context, q string, args ...any) ([]model.Schedule, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying schedules: %w", err)
	}
	defer rows.Close()
	out := []model.Schedule{}
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func scanSchedule(s rowScanner) (*model.Schedule, error) {
	var (
		sc                   model.Schedule
		enabled              int
		createdAt, updatedAt string
	)
	if err := s.Scan(&sc.ID, &sc.Name, &sc.Spec, &enabled, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	sc.Enabled = enabled != 0
	sc.CreatedAt = parseTime(createdAt)
	sc.UpdatedAt = parseTime(updatedAt)
	return &sc, nil
}
