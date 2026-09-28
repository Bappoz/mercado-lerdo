// Package store reúne todo o SQL que a API do Mercado Lerdo executa.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound é devolvido quando a busca de uma linha não encontra nada.
var ErrNotFound = errors.New("store: não encontrado")

// Store envolve o pool de conexões.
type Store struct {
	pool *pgxpool.Pool
}

// New devolve um Store sobre pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}
