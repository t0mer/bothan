package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/t0mer/bothan/internal/model"
)

// RuleRepo provides access to the rules table.
type RuleRepo struct {
	db *sql.DB
}

// Rules returns a rule repository bound to this store's database.
func (s *Store) Rules() *RuleRepo { return &RuleRepo{db: s.db} }

const ruleColumns = `id, host_id, name, condition_type, COALESCE(threshold_grade, ''),
	expiry_days, enabled, created_at, updated_at`

// Create inserts a rule and populates its ID and timestamps.
func (r *RuleRepo) Create(ctx context.Context, rule *model.Rule) error {
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO rules (host_id, name, condition_type, threshold_grade, expiry_days, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`,
		nullableInt64(rule.HostID), rule.Name, rule.ConditionType,
		nullableStr(rule.ThresholdGrade), nullableInt(rule.ExpiryDays), boolToInt(rule.Enabled),
	).Scan(&rule.ID, &createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("creating rule: %w", err)
	}
	rule.CreatedAt = parseTime(createdAt)
	rule.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Get returns a rule by id, or ErrNotFound.
func (r *RuleRepo) Get(ctx context.Context, id int64) (*model.Rule, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+ruleColumns+` FROM rules WHERE id = ?`, id)
	rule, err := scanRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting rule %d: %w", id, err)
	}
	return rule, nil
}

// List returns all rules (global and per-host).
func (r *RuleRepo) List(ctx context.Context) ([]model.Rule, error) {
	return r.query(ctx, `SELECT `+ruleColumns+` FROM rules ORDER BY name`)
}

// RulesForHost returns the enabled rules applicable to a host: global rules
// (host_id NULL) plus the host's own rules.
func (r *RuleRepo) RulesForHost(ctx context.Context, hostID int64) ([]model.Rule, error) {
	return r.query(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE enabled = 1 AND (host_id IS NULL OR host_id = ?) ORDER BY name`,
		hostID)
}

// ListByHost returns the rules explicitly attached to a host.
func (r *RuleRepo) ListByHost(ctx context.Context, hostID int64) ([]model.Rule, error) {
	return r.query(ctx, `SELECT `+ruleColumns+` FROM rules WHERE host_id = ? ORDER BY name`, hostID)
}

// Update writes a rule's fields and refreshes updated_at.
func (r *RuleRepo) Update(ctx context.Context, rule *model.Rule) error {
	var updatedAt string
	err := r.db.QueryRowContext(ctx, `
		UPDATE rules SET host_id = ?, name = ?, condition_type = ?, threshold_grade = ?,
			expiry_days = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? RETURNING updated_at`,
		nullableInt64(rule.HostID), rule.Name, rule.ConditionType,
		nullableStr(rule.ThresholdGrade), nullableInt(rule.ExpiryDays), boolToInt(rule.Enabled), rule.ID,
	).Scan(&updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("updating rule %d: %w", rule.ID, err)
	}
	rule.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Delete removes a rule.
func (r *RuleRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting rule %d: %w", id, err)
	}
	return requireOneRow(res)
}

func (r *RuleRepo) query(ctx context.Context, q string, args ...any) ([]model.Rule, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying rules: %w", err)
	}
	defer rows.Close()
	out := []model.Rule{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rule)
	}
	return out, rows.Err()
}

func scanRule(s rowScanner) (*model.Rule, error) {
	var (
		rule                 model.Rule
		hostID               sql.NullInt64
		expiryDays           sql.NullInt64
		enabled              int
		createdAt, updatedAt string
	)
	if err := s.Scan(&rule.ID, &hostID, &rule.Name, &rule.ConditionType, &rule.ThresholdGrade,
		&expiryDays, &enabled, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if hostID.Valid {
		v := hostID.Int64
		rule.HostID = &v
	}
	if expiryDays.Valid {
		v := int(expiryDays.Int64)
		rule.ExpiryDays = &v
	}
	rule.Enabled = enabled != 0
	rule.CreatedAt = parseTime(createdAt)
	rule.UpdatedAt = parseTime(updatedAt)
	return &rule, nil
}

func nullableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
