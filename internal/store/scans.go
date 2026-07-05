package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/t0mer/bothan/internal/model"
)

// ScanRepo provides access to the scans and scan_endpoints tables.
type ScanRepo struct {
	db *sql.DB
}

// Scans returns a scan repository bound to this store's database.
func (s *Store) Scans() *ScanRepo { return &ScanRepo{db: s.db} }

// Create inserts a new scan row (typically status=pending) and sets its ID and
// CreatedAt.
func (r *ScanRepo) Create(ctx context.Context, sc *model.Scan) error {
	var createdAt string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO scans (host_id, status, trigger)
		VALUES (?, ?, ?)
		RETURNING id, created_at`,
		sc.HostID, sc.Status, sc.Trigger,
	).Scan(&sc.ID, &createdAt)
	if err != nil {
		return fmt.Errorf("creating scan: %w", err)
	}
	sc.CreatedAt = parseTime(createdAt)
	return nil
}

// SetStatus updates a scan's status (and started_at when moving to running).
func (r *ScanRepo) SetStatus(ctx context.Context, id int64, status string) error {
	var startedClause string
	if status == model.ScanStatusRunning {
		startedClause = ", started_at = CURRENT_TIMESTAMP"
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE scans SET status = ?`+startedClause+` WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("updating scan %d status: %w", id, err)
	}
	return nil
}

// Fail marks a scan as errored with a message and completion time.
func (r *ScanRepo) Fail(ctx context.Context, id int64, message string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scans SET status = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?`, model.ScanStatusError, message, id)
	if err != nil {
		return fmt.Errorf("failing scan %d: %w", id, err)
	}
	return nil
}

// SaveResult persists a completed scan: it updates the scan row (grade,
// versions, raw JSON, completion) and replaces its endpoints, transactionally.
func (r *ScanRepo) SaveResult(ctx context.Context, sc *model.Scan, rawJSON []byte) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `
		UPDATE scans SET
			status = ?, overall_grade = ?, engine_version = ?, criteria_version = ?,
			raw_json = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		sc.Status, sc.OverallGrade, sc.EngineVersion, sc.CriteriaVersion, rawJSON, sc.ID,
	); err != nil {
		return fmt.Errorf("saving scan %d: %w", sc.ID, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM scan_endpoints WHERE scan_id = ?`, sc.ID); err != nil {
		return fmt.Errorf("clearing endpoints for scan %d: %w", sc.ID, err)
	}
	for i := range sc.Endpoints {
		e := &sc.Endpoints[i]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scan_endpoints
				(scan_id, ip_address, server_name, grade, grade_trust_ignored,
				 has_warnings, is_exceptional, status_message, cert_not_after, progress)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sc.ID, e.IPAddress, e.ServerName, e.Grade, e.GradeTrustIgnored,
			boolToInt(e.HasWarnings), boolToInt(e.IsExceptional), e.StatusMessage,
			nullableTime(e.CertNotAfter), e.Progress,
		); err != nil {
			return fmt.Errorf("saving endpoint for scan %d: %w", sc.ID, err)
		}
	}
	return tx.Commit()
}

const scanColumns = `id, host_id, status, trigger, COALESCE(overall_grade, ''),
	COALESCE(engine_version, ''), COALESCE(criteria_version, ''),
	COALESCE(error_message, ''), started_at, completed_at, created_at`

// Get returns a scan with its endpoints, or ErrNotFound.
func (r *ScanRepo) Get(ctx context.Context, id int64) (*model.Scan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+scanColumns+` FROM scans WHERE id = ?`, id)
	sc, err := scanScan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting scan %d: %w", id, err)
	}
	eps, err := r.endpoints(ctx, id)
	if err != nil {
		return nil, err
	}
	sc.Endpoints = eps
	return sc, nil
}

// GetRaw returns the stored raw SSL Labs Host JSON for a scan.
func (r *ScanRepo) GetRaw(ctx context.Context, id int64) ([]byte, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT raw_json FROM scans WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting raw scan %d: %w", id, err)
	}
	return raw, nil
}

// ListByHost returns a host's scans (newest first), up to limit.
func (r *ScanRepo) ListByHost(ctx context.Context, hostID int64, limit int) ([]model.Scan, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+scanColumns+` FROM scans WHERE host_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`,
		hostID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing scans for host %d: %w", hostID, err)
	}
	defer rows.Close()

	scans := []model.Scan{}
	for rows.Next() {
		sc, err := scanScan(rows)
		if err != nil {
			return nil, err
		}
		scans = append(scans, *sc)
	}
	return scans, rows.Err()
}

// LatestReadyByHost returns a host's most recent ready scan, or ErrNotFound.
func (r *ScanRepo) LatestReadyByHost(ctx context.Context, hostID int64) (*model.Scan, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+scanColumns+` FROM scans WHERE host_id = ? AND status = ?
		 ORDER BY created_at DESC, id DESC LIMIT 1`,
		hostID, model.ScanStatusReady)
	sc, err := scanScan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("latest ready scan for host %d: %w", hostID, err)
	}
	return sc, nil
}

// HasActive reports whether a host already has a pending or running scan.
func (r *ScanRepo) HasActive(ctx context.Context, hostID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM scans WHERE host_id = ? AND status IN (?, ?)`,
		hostID, model.ScanStatusPending, model.ScanStatusRunning).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking active scans for host %d: %w", hostID, err)
	}
	return n > 0, nil
}

// PendingScans returns all scans currently pending or running (for restart
// recovery in the scheduler/worker).
func (r *ScanRepo) PendingScans(ctx context.Context) ([]model.Scan, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+scanColumns+` FROM scans WHERE status IN (?, ?) ORDER BY created_at`,
		model.ScanStatusPending, model.ScanStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("listing pending scans: %w", err)
	}
	defer rows.Close()

	scans := []model.Scan{}
	for rows.Next() {
		sc, err := scanScan(rows)
		if err != nil {
			return nil, err
		}
		scans = append(scans, *sc)
	}
	return scans, rows.Err()
}

// LatestByHosts returns each host's most recent scan summary, keyed by host id.
func (r *ScanRepo) LatestByHosts(ctx context.Context) (map[int64]model.HostScanSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT host_id, status, COALESCE(overall_grade, ''), completed_at FROM (
			SELECT host_id, status, overall_grade, completed_at,
				ROW_NUMBER() OVER (PARTITION BY host_id ORDER BY created_at DESC, id DESC) AS rn
			FROM scans
		) WHERE rn = 1`)
	if err != nil {
		return nil, fmt.Errorf("latest scans by host: %w", err)
	}
	defer rows.Close()

	out := map[int64]model.HostScanSummary{}
	for rows.Next() {
		var (
			hostID      int64
			summary     model.HostScanSummary
			completedAt sql.NullString
		)
		if err := rows.Scan(&hostID, &summary.Status, &summary.Grade, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid && completedAt.String != "" {
			t := parseTime(completedAt.String)
			summary.CompletedAt = &t
		}
		out[hostID] = summary
	}
	return out, rows.Err()
}

func (r *ScanRepo) endpoints(ctx context.Context, scanID int64) ([]model.ScanEndpoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, scan_id, ip_address, COALESCE(server_name, ''), COALESCE(grade, ''),
			COALESCE(grade_trust_ignored, ''), COALESCE(has_warnings, 0),
			COALESCE(is_exceptional, 0), COALESCE(status_message, ''),
			cert_not_after, COALESCE(progress, 0)
		FROM scan_endpoints WHERE scan_id = ? ORDER BY ip_address`, scanID)
	if err != nil {
		return nil, fmt.Errorf("listing endpoints for scan %d: %w", scanID, err)
	}
	defer rows.Close()

	eps := []model.ScanEndpoint{}
	for rows.Next() {
		var (
			e                        model.ScanEndpoint
			hasWarnings, exceptional int
			certNotAfter             sql.NullString
		)
		if err := rows.Scan(&e.ID, &e.ScanID, &e.IPAddress, &e.ServerName, &e.Grade,
			&e.GradeTrustIgnored, &hasWarnings, &exceptional, &e.StatusMessage,
			&certNotAfter, &e.Progress); err != nil {
			return nil, err
		}
		e.HasWarnings = hasWarnings != 0
		e.IsExceptional = exceptional != 0
		if certNotAfter.Valid && certNotAfter.String != "" {
			t := parseTime(certNotAfter.String)
			if !t.IsZero() {
				e.CertNotAfter = &t
			}
		}
		eps = append(eps, e)
	}
	return eps, rows.Err()
}

func scanScan(s rowScanner) (*model.Scan, error) {
	var (
		sc                     model.Scan
		startedAt, completedAt sql.NullString
		createdAt              string
	)
	if err := s.Scan(&sc.ID, &sc.HostID, &sc.Status, &sc.Trigger, &sc.OverallGrade,
		&sc.EngineVersion, &sc.CriteriaVersion, &sc.ErrorMessage,
		&startedAt, &completedAt, &createdAt); err != nil {
		return nil, err
	}
	sc.CreatedAt = parseTime(createdAt)
	if startedAt.Valid && startedAt.String != "" {
		t := parseTime(startedAt.String)
		sc.StartedAt = &t
	}
	if completedAt.Valid && completedAt.String != "" {
		t := parseTime(completedAt.String)
		sc.CompletedAt = &t
	}
	return &sc, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}
