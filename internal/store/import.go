package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/t0mer/bothan/internal/model"
)

// Import modes.
const (
	ImportMerge   = "merge"
	ImportReplace = "replace"
)

// ImportChannel is a channel to import, with its resolved ciphertext.
type ImportChannel struct {
	Name             string
	Type             string
	Enabled          bool
	ConfigEncrypted  []byte // nil when secrets are absent (needs credentials)
	NeedsCredentials bool
}

// ImportRule is a rule to import, referencing its host by name (empty = global).
type ImportRule struct {
	Name           string
	Hostname       string
	ConditionType  string
	ThresholdGrade string
	ExpiryDays     *int
	Enabled        bool
}

// LinkPair references two entities by natural key.
type LinkPair struct {
	Host   string
	Target string
}

// ImportData is a fully-resolved bundle ready to apply (secrets already decoded
// to ciphertext for the destination key).
type ImportData struct {
	Hosts         []model.Host
	Schedules     []model.Schedule
	Channels      []ImportChannel
	Rules         []ImportRule
	HostSchedules []LinkPair
	HostChannels  []LinkPair
}

// ImportReport summarises what an import did (or would do, for a dry run).
type ImportReport struct {
	HostsCreated               int `json:"hosts_created"`
	HostsUpdated               int `json:"hosts_updated"`
	SchedulesCreated           int `json:"schedules_created"`
	SchedulesUpdated           int `json:"schedules_updated"`
	ChannelsCreated            int `json:"channels_created"`
	ChannelsUpdated            int `json:"channels_updated"`
	RulesCreated               int `json:"rules_created"`
	RulesUpdated               int `json:"rules_updated"`
	Removed                    int `json:"removed"`
	LinksCreated               int `json:"links_created"`
	ChannelsNeedingCredentials int `json:"channels_needing_credentials"`
}

// ApplyImport applies resolved bundle data in a single transaction. In replace
// mode it first wipes schedules, channels, and rules (and their links); hosts
// are always upserted so scan history is preserved. When dryRun is true nothing
// is committed but the report reflects what would happen.
func (s *Store) ApplyImport(ctx context.Context, data ImportData, mode string, dryRun bool) (*ImportReport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	rep := &ImportReport{}

	if mode == ImportReplace {
		for _, table := range []string{"rules", "channels", "schedules"} {
			res, err := tx.ExecContext(ctx, `DELETE FROM `+table)
			if err != nil {
				return nil, fmt.Errorf("wiping %s: %w", table, err)
			}
			n, _ := res.RowsAffected()
			rep.Removed += int(n)
		}
	}

	for _, h := range data.Hosts {
		created, err := upsertHost(ctx, tx, h)
		if err != nil {
			return nil, err
		}
		if created {
			rep.HostsCreated++
		} else {
			rep.HostsUpdated++
		}
	}
	for _, sc := range data.Schedules {
		created, err := upsertSchedule(ctx, tx, sc)
		if err != nil {
			return nil, err
		}
		if created {
			rep.SchedulesCreated++
		} else {
			rep.SchedulesUpdated++
		}
	}
	for _, ch := range data.Channels {
		created, err := upsertChannel(ctx, tx, ch)
		if err != nil {
			return nil, err
		}
		if created {
			rep.ChannelsCreated++
		} else {
			rep.ChannelsUpdated++
		}
		if ch.NeedsCredentials {
			rep.ChannelsNeedingCredentials++
		}
	}
	for _, ru := range data.Rules {
		created, err := upsertRule(ctx, tx, ru)
		if err != nil {
			return nil, err
		}
		if created {
			rep.RulesCreated++
		} else {
			rep.RulesUpdated++
		}
	}

	for _, l := range data.HostSchedules {
		n, err := linkByNames(ctx, tx, "host_schedules", "schedule_id", "schedules", l)
		if err != nil {
			return nil, err
		}
		rep.LinksCreated += n
	}
	for _, l := range data.HostChannels {
		n, err := linkByNames(ctx, tx, "host_channels", "channel_id", "channels", l)
		if err != nil {
			return nil, err
		}
		rep.LinksCreated += n
	}

	if dryRun {
		return rep, nil // rollback via defer
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing import: %w", err)
	}
	return rep, nil
}

func upsertHost(ctx context.Context, tx *sql.Tx, h model.Host) (created bool, err error) {
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM hosts WHERE hostname = ?`, h.Hostname).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO hosts (hostname, enabled, publish, ignore_mismatch, from_cache, max_age_hours, notes)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			h.Hostname, boolToInt(h.Enabled), boolToInt(h.Publish), boolToInt(h.IgnoreMismatch),
			boolToInt(h.FromCache), nullableInt(h.MaxAgeHours), h.Notes)
		return true, err
	}
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE hosts SET enabled = ?, publish = ?, ignore_mismatch = ?, from_cache = ?,
			max_age_hours = ?, notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		boolToInt(h.Enabled), boolToInt(h.Publish), boolToInt(h.IgnoreMismatch),
		boolToInt(h.FromCache), nullableInt(h.MaxAgeHours), h.Notes, id)
	return false, err
}

func upsertSchedule(ctx context.Context, tx *sql.Tx, sc model.Schedule) (created bool, err error) {
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM schedules WHERE name = ?`, sc.Name).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `INSERT INTO schedules (name, spec, enabled) VALUES (?, ?, ?)`,
			sc.Name, sc.Spec, boolToInt(sc.Enabled))
		return true, err
	}
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE schedules SET spec = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		sc.Spec, boolToInt(sc.Enabled), id)
	return false, err
}

func upsertChannel(ctx context.Context, tx *sql.Tx, ch ImportChannel) (created bool, err error) {
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM channels WHERE name = ?`, ch.Name).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO channels (name, type, config_encrypted, needs_credentials, enabled)
			VALUES (?, ?, ?, ?, ?)`,
			ch.Name, ch.Type, ch.ConfigEncrypted, boolToInt(ch.NeedsCredentials), boolToInt(ch.Enabled))
		return true, err
	}
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE channels SET type = ?, config_encrypted = ?, needs_credentials = ?, enabled = ?,
			updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		ch.Type, ch.ConfigEncrypted, boolToInt(ch.NeedsCredentials), boolToInt(ch.Enabled), id)
	return false, err
}

func upsertRule(ctx context.Context, tx *sql.Tx, ru ImportRule) (created bool, err error) {
	var hostID any
	if ru.Hostname != "" {
		var hid int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM hosts WHERE hostname = ?`, ru.Hostname).Scan(&hid); err != nil {
			return false, fmt.Errorf("rule %q references unknown host %q: %w", ru.Name, ru.Hostname, err)
		}
		hostID = hid
	}
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM rules WHERE name = ?`, ru.Name).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO rules (host_id, name, condition_type, threshold_grade, expiry_days, enabled)
			VALUES (?, ?, ?, ?, ?, ?)`,
			hostID, ru.Name, ru.ConditionType, nullableStr(ru.ThresholdGrade), nullableInt(ru.ExpiryDays), boolToInt(ru.Enabled))
		return true, err
	}
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE rules SET host_id = ?, condition_type = ?, threshold_grade = ?, expiry_days = ?,
			enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		hostID, ru.ConditionType, nullableStr(ru.ThresholdGrade), nullableInt(ru.ExpiryDays), boolToInt(ru.Enabled), id)
	return false, err
}

// linkByNames resolves a (host, target) name pair to ids and inserts the link.
func linkByNames(ctx context.Context, tx *sql.Tx, linkTable, targetCol, targetTable string, l LinkPair) (int, error) {
	var hostID, targetID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM hosts WHERE hostname = ?`, l.Host).Scan(&hostID); err != nil {
		return 0, fmt.Errorf("link references unknown host %q: %w", l.Host, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM `+targetTable+` WHERE name = ?`, l.Target).Scan(&targetID); err != nil {
		return 0, fmt.Errorf("link references unknown %s %q: %w", targetTable, l.Target, err)
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO `+linkTable+` (host_id, `+targetCol+`) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		hostID, targetID)
	if err != nil {
		return 0, fmt.Errorf("linking %s: %w", linkTable, err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
