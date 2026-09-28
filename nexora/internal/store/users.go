package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Credentials é o que o login precisa para conferir uma senha.
type Credentials struct {
	UserID       int64
	PasswordHash string
}

// CredentialsByEmail busca o hash da senha de um usuário.
func (s *Store) CredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	var c Credentials
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE email = $1`, email).
		Scan(&c.UserID, &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("credenciais por e-mail: %w", err)
	}
	return c, nil
}

// CreateSession grava um token de sessão para userID.
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, now() + $3::interval)`,
		token, userID, ttl.String())
	if err != nil {
		return fmt.Errorf("criar a sessão: %w", err)
	}
	return nil
}

// UserBySession traduz um token de sessão válido no id do usuário.
func (s *Store) UserBySession(ctx context.Context, token string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM sessions WHERE token = $1 AND expires_at > now()`, token).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("usuário pela sessão: %w", err)
	}
	return userID, nil
}
