package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/t0mer/bothan/internal/model"
)

// ChannelRepo provides access to channels and host_channels. Config encryption
// happens above this layer; the repo stores/loads the ciphertext blob only.
type ChannelRepo struct {
	db *sql.DB
}

// Channels returns a channel repository bound to this store's database.
func (s *Store) Channels() *ChannelRepo { return &ChannelRepo{db: s.db} }

const channelColumns = `id, name, type, config_encrypted, needs_credentials, enabled, created_at, updated_at`

// Create inserts a channel and populates its ID and timestamps.
func (r *ChannelRepo) Create(ctx context.Context, c *model.Channel) error {
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO channels (name, type, config_encrypted, needs_credentials, enabled)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`,
		c.Name, c.Type, c.ConfigEncrypted, boolToInt(c.NeedsCredentials), boolToInt(c.Enabled),
	).Scan(&c.ID, &createdAt, &updatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("channel %q: %w", c.Name, ErrConflict)
		}
		return fmt.Errorf("creating channel: %w", err)
	}
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Get returns a channel by id, or ErrNotFound.
func (r *ChannelRepo) Get(ctx context.Context, id int64) (*model.Channel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+channelColumns+` FROM channels WHERE id = ?`, id)
	c, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting channel %d: %w", id, err)
	}
	return c, nil
}

// List returns all channels ordered by name.
func (r *ChannelRepo) List(ctx context.Context) ([]model.Channel, error) {
	return r.query(ctx, `SELECT `+channelColumns+` FROM channels ORDER BY name`)
}

// Update writes a channel's mutable fields. When configEncrypted is nil the
// stored config is left unchanged (so edits without re-entering secrets keep them).
func (r *ChannelRepo) Update(ctx context.Context, c *model.Channel, changeConfig bool) error {
	var (
		updatedAt string
		err       error
	)
	if changeConfig {
		err = r.db.QueryRowContext(ctx, `
			UPDATE channels SET name = ?, type = ?, config_encrypted = ?, needs_credentials = ?,
				enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? RETURNING updated_at`,
			c.Name, c.Type, c.ConfigEncrypted, boolToInt(c.NeedsCredentials), boolToInt(c.Enabled), c.ID,
		).Scan(&updatedAt)
	} else {
		err = r.db.QueryRowContext(ctx, `
			UPDATE channels SET name = ?, type = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? RETURNING updated_at`,
			c.Name, c.Type, boolToInt(c.Enabled), c.ID,
		).Scan(&updatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("channel %q: %w", c.Name, ErrConflict)
		}
		return fmt.Errorf("updating channel %d: %w", c.ID, err)
	}
	c.UpdatedAt = parseTime(updatedAt)
	return nil
}

// Delete removes a channel (cascading its host links).
func (r *ChannelRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting channel %d: %w", id, err)
	}
	return requireOneRow(res)
}

// SetHostChannels replaces the set of channels linked to a host.
func (r *ChannelRepo) SetHostChannels(ctx context.Context, hostID int64, channelIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM host_channels WHERE host_id = ?`, hostID); err != nil {
		return fmt.Errorf("clearing host channels: %w", err)
	}
	for _, cid := range channelIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO host_channels (host_id, channel_id) VALUES (?, ?)`, hostID, cid); err != nil {
			return fmt.Errorf("linking channel %d to host %d: %w", cid, hostID, err)
		}
	}
	return tx.Commit()
}

// ChannelsForHost returns the channels linked to a host.
func (r *ChannelRepo) ChannelsForHost(ctx context.Context, hostID int64) ([]model.Channel, error) {
	return r.query(ctx, `
		SELECT c.id, c.name, c.type, c.config_encrypted, c.needs_credentials, c.enabled, c.created_at, c.updated_at
		FROM channels c JOIN host_channels hc ON hc.channel_id = c.id
		WHERE hc.host_id = ? ORDER BY c.name`, hostID)
}

// EnabledChannelsForHost returns the enabled, usable channels linked to a host.
func (r *ChannelRepo) EnabledChannelsForHost(ctx context.Context, hostID int64) ([]model.Channel, error) {
	return r.query(ctx, `
		SELECT c.id, c.name, c.type, c.config_encrypted, c.needs_credentials, c.enabled, c.created_at, c.updated_at
		FROM channels c JOIN host_channels hc ON hc.channel_id = c.id
		WHERE hc.host_id = ? AND c.enabled = 1 AND c.needs_credentials = 0 ORDER BY c.name`, hostID)
}

func (r *ChannelRepo) query(ctx context.Context, q string, args ...any) ([]model.Channel, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying channels: %w", err)
	}
	defer rows.Close()
	out := []model.Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func scanChannel(s rowScanner) (*model.Channel, error) {
	var (
		c                    model.Channel
		needsCreds, enabled  int
		configEncrypted      []byte
		createdAt, updatedAt string
	)
	if err := s.Scan(&c.ID, &c.Name, &c.Type, &configEncrypted, &needsCreds, &enabled, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	c.ConfigEncrypted = configEncrypted
	c.NeedsCredentials = needsCreds != 0
	c.Enabled = enabled != 0
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	return &c, nil
}
