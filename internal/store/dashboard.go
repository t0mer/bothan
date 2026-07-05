package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/t0mer/bothan/internal/model"
)

// DashboardRepo computes the dashboard summary.
type DashboardRepo struct {
	db *sql.DB
}

// Dashboard returns a dashboard repository bound to this store's database.
func (s *Store) Dashboard() *DashboardRepo { return &DashboardRepo{db: s.db} }

// latestReadyCTE selects the latest ready scan id and grade per host.
const latestReadyCTE = `
	SELECT host_id, id, overall_grade FROM (
		SELECT host_id, id, overall_grade,
			ROW_NUMBER() OVER (PARTITION BY host_id ORDER BY created_at DESC, id DESC) AS rn
		FROM scans WHERE status = 'ready'
	) WHERE rn = 1`

// Summary assembles the dashboard summary. certWindowDays sets the cert-expiry
// warning window; recentLimit bounds the recent-scan list.
func (r *DashboardRepo) Summary(ctx context.Context, certWindowDays, recentLimit int) (*model.DashboardSummary, error) {
	if certWindowDays <= 0 {
		certWindowDays = 30
	}
	if recentLimit <= 0 {
		recentLimit = 10
	}
	sum := &model.DashboardSummary{
		CertExpiryWindowDays: certWindowDays,
		GradeCounts:          []model.GradeCount{},
		CertsExpiringSoon:    []model.CertExpiry{},
		RecentScans:          []model.RecentScan{},
	}

	if err := r.hostCountsAndGrades(ctx, sum); err != nil {
		return nil, err
	}
	if err := r.certExpiries(ctx, sum, certWindowDays); err != nil {
		return nil, err
	}
	if err := r.recentScans(ctx, sum, recentLimit); err != nil {
		return nil, err
	}
	return sum, nil
}

func (r *DashboardRepo) hostCountsAndGrades(ctx context.Context, sum *model.DashboardSummary) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT h.enabled, COALESCE(ls.overall_grade, '') AS grade
		FROM hosts h
		LEFT JOIN (`+latestReadyCTE+`) ls ON ls.host_id = h.id`)
	if err != nil {
		return fmt.Errorf("host counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var enabled int
		var grade string
		if err := rows.Scan(&enabled, &grade); err != nil {
			return err
		}
		sum.TotalHosts++
		if enabled != 0 {
			sum.EnabledHosts++
		} else {
			sum.DisabledHosts++
		}
		if grade == "" {
			sum.NeverScanned++
			counts["none"]++
		} else {
			counts[grade]++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, g := range model.GradeDisplayOrder {
		if c := counts[g]; c > 0 {
			sum.GradeCounts = append(sum.GradeCounts, model.GradeCount{Grade: g, Count: c})
		}
	}
	return nil
}

func (r *DashboardRepo) certExpiries(ctx context.Context, sum *model.DashboardSummary, days int) error {
	threshold := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour).Format("2006-01-02 15:04:05")
	rows, err := r.db.QueryContext(ctx, `
		SELECT h.id, h.hostname, MIN(se.cert_not_after) AS earliest
		FROM hosts h
		JOIN (`+latestReadyCTE+`) ls ON ls.host_id = h.id
		JOIN scan_endpoints se ON se.scan_id = ls.id
		WHERE se.cert_not_after IS NOT NULL AND se.cert_not_after <= ?
		GROUP BY h.id, h.hostname
		ORDER BY earliest ASC`, threshold)
	if err != nil {
		return fmt.Errorf("cert expiries: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var (
			ce       model.CertExpiry
			notAfter string
		)
		if err := rows.Scan(&ce.HostID, &ce.Hostname, &notAfter); err != nil {
			return err
		}
		ce.NotAfter = parseTime(notAfter)
		ce.Days = int(ce.NotAfter.Sub(now).Hours() / 24)
		sum.CertsExpiringSoon = append(sum.CertsExpiringSoon, ce)
	}
	return rows.Err()
}

// HostMetrics returns a per-host snapshot (grade + earliest cert expiry) for
// the Prometheus collector.
func (r *DashboardRepo) HostMetrics(ctx context.Context) ([]model.HostMetric, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT h.hostname, h.enabled, COALESCE(ls.overall_grade, '') AS grade, mc.earliest
		FROM hosts h
		LEFT JOIN (`+latestReadyCTE+`) ls ON ls.host_id = h.id
		LEFT JOIN (
			SELECT scan_id, MIN(cert_not_after) AS earliest
			FROM scan_endpoints WHERE cert_not_after IS NOT NULL GROUP BY scan_id
		) mc ON mc.scan_id = ls.id
		ORDER BY h.hostname`)
	if err != nil {
		return nil, fmt.Errorf("host metrics: %w", err)
	}
	defer rows.Close()

	out := []model.HostMetric{}
	for rows.Next() {
		var (
			hm       model.HostMetric
			enabled  int
			earliest sql.NullString
		)
		if err := rows.Scan(&hm.Hostname, &enabled, &hm.Grade, &earliest); err != nil {
			return nil, err
		}
		hm.Enabled = enabled != 0
		if earliest.Valid && earliest.String != "" {
			t := parseTime(earliest.String)
			if !t.IsZero() {
				hm.CertNotAfter = &t
			}
		}
		out = append(out, hm)
	}
	return out, rows.Err()
}

func (r *DashboardRepo) recentScans(ctx context.Context, sum *model.DashboardSummary, limit int) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.host_id, h.hostname, COALESCE(s.overall_grade, ''), s.status,
			s.completed_at, s.created_at
		FROM scans s JOIN hosts h ON h.id = s.host_id
		ORDER BY s.created_at DESC, s.id DESC LIMIT ?`, limit)
	if err != nil {
		return fmt.Errorf("recent scans: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			rs          model.RecentScan
			completedAt sql.NullString
			createdAt   string
		)
		if err := rows.Scan(&rs.ScanID, &rs.HostID, &rs.Hostname, &rs.Grade, &rs.Status,
			&completedAt, &createdAt); err != nil {
			return err
		}
		rs.CreatedAt = parseTime(createdAt)
		if completedAt.Valid && completedAt.String != "" {
			t := parseTime(completedAt.String)
			rs.CompletedAt = &t
		}
		sum.RecentScans = append(sum.RecentScans, rs)
	}
	return rows.Err()
}
