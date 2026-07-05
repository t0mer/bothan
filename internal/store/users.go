package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/t0mer/bothan/internal/model"
)

// UserRepo provides access to the users table.
type UserRepo struct {
	db *sql.DB
}

// Users returns a user repository bound to this store's database.
func (s *Store) Users() *UserRepo { return &UserRepo{db: s.db} }

// Count returns the number of users.
func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&n)
	return n, err
}

// Create inserts a user with a pre-hashed password.
func (r *UserRepo) Create(ctx context.Context, username, passwordHash string) (*model.User, error) {
	var (
		id        int64
		createdAt string
	)
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (username, password_hash) VALUES (?, ?) RETURNING id, created_at`,
		username, passwordHash).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("user %q: %w", username, ErrConflict)
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return &model.User{ID: id, Username: username, PasswordHash: passwordHash, CreatedAt: parseTime(createdAt)}, nil
}

// GetByUsername returns a user by name, or ErrNotFound.
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var (
		u         model.User
		createdAt string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user %q: %w", username, err)
	}
	u.CreatedAt = parseTime(createdAt)
	return &u, nil
}

// SetPassword updates a user's password hash.
func (r *UserRepo) SetPassword(ctx context.Context, username, passwordHash string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE username = ?`, passwordHash, username)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	return requireOneRow(res)
}

// --- API tokens ---

// TokenRepo provides access to the api_tokens table.
type TokenRepo struct {
	db *sql.DB
}

// Tokens returns a token repository bound to this store's database.
func (s *Store) Tokens() *TokenRepo { return &TokenRepo{db: s.db} }

// Create inserts an API token (storing only its hash).
func (r *TokenRepo) Create(ctx context.Context, t *model.APIToken) error {
	var createdAt string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO api_tokens (name, token_hash, scopes, expires_at)
		VALUES (?, ?, ?, ?) RETURNING id, created_at`,
		t.Name, t.TokenHash, t.Scopes, nullableTime(t.ExpiresAt)).Scan(&t.ID, &createdAt)
	if err != nil {
		return fmt.Errorf("creating token: %w", err)
	}
	t.CreatedAt = parseTime(createdAt)
	return nil
}

// GetByHash returns a non-expired token by its hash, or ErrNotFound.
func (r *TokenRepo) GetByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	t, err := r.scanOne(ctx, `SELECT id, name, token_hash, scopes, last_used_at, expires_at, created_at
		FROM api_tokens WHERE token_hash = ?`, hash)
	if err != nil {
		return nil, err
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return nil, ErrNotFound
	}
	return t, nil
}

// Touch updates a token's last_used_at timestamp.
func (r *TokenRepo) Touch(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// List returns all tokens (without secrets — only hashes are stored anyway).
func (r *TokenRepo) List(ctx context.Context) ([]model.APIToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, token_hash, scopes, last_used_at, expires_at, created_at
		FROM api_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing tokens: %w", err)
	}
	defer rows.Close()
	out := []model.APIToken{}
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Delete removes a token by id.
func (r *TokenRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting token %d: %w", id, err)
	}
	return requireOneRow(res)
}

func (r *TokenRepo) scanOne(ctx context.Context, q string, args ...any) (*model.APIToken, error) {
	t, err := scanToken(r.db.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning token: %w", err)
	}
	return t, nil
}

func scanToken(s rowScanner) (*model.APIToken, error) {
	var (
		t                   model.APIToken
		lastUsed, expiresAt sql.NullString
		createdAt           string
	)
	if err := s.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Scopes, &lastUsed, &expiresAt, &createdAt); err != nil {
		return nil, err
	}
	t.CreatedAt = parseTime(createdAt)
	if lastUsed.Valid && lastUsed.String != "" {
		v := parseTime(lastUsed.String)
		t.LastUsedAt = &v
	}
	if expiresAt.Valid && expiresAt.String != "" {
		v := parseTime(expiresAt.String)
		t.ExpiresAt = &v
	}
	return &t, nil
}
