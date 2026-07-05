package store

import (
	"context"
	"database/sql"
	"fmt"
)

// SettingsRepo provides key/value access to the settings table.
type SettingsRepo struct {
	db *sql.DB
}

// Settings returns a settings repository bound to this store's database.
func (s *Store) Settings() *SettingsRepo { return &SettingsRepo{db: s.db} }

// GetAll returns every stored setting as a key/value map.
func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("querying settings: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scanning setting: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SeedDefaults inserts any missing default keys without overwriting existing
// values. Safe to call on every startup.
func (r *SettingsRepo) SeedDefaults(ctx context.Context, defaults map[string]string) error {
	return r.withTx(ctx, func(tx *sql.Tx) error {
		for k, v := range defaults {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`,
				k, v); err != nil {
				return fmt.Errorf("seeding setting %q: %w", k, err)
			}
		}
		return nil
	})
}

// SetMany upserts the given key/value pairs in a single transaction.
func (r *SettingsRepo) SetMany(ctx context.Context, values map[string]string) error {
	return r.withTx(ctx, func(tx *sql.Tx) error {
		for k, v := range values {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO settings(key, value) VALUES (?, ?)
				ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
				k, v); err != nil {
				return fmt.Errorf("setting %q: %w", k, err)
			}
		}
		return nil
	})
}

func (r *SettingsRepo) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
