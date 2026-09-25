package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Credentials is what login needs to check a password.
type Credentials struct {
	UserID       int64
	PasswordHash string
}

// CredentialsByEmail looks up a user's password hash.
func (s *Store) CredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	var c Credentials
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE email = $1`, email).
		Scan(&c.UserID, &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("credentials by email: %w", err)
	}
	return c, nil
}

// CreateSession stores a session token for userID.
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, now() + $3::interval)`,
		token, userID, ttl.String())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// UserBySession resolves a non-expired session token to its user id.
func (s *Store) UserBySession(ctx context.Context, token string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM sessions WHERE token = $1 AND expires_at > now()`, token).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("user by session: %w", err)
	}
	return userID, nil
}
