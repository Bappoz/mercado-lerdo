// Package store holds every SQL statement the Mercado Lerdo API runs.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a single-row lookup finds nothing.
var ErrNotFound = errors.New("store: not found")

// Store wraps the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}
