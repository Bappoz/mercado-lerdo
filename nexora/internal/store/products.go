package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Product é um item do catálogo.
type Product struct {
	ID          int64     `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Category    string    `json:"category"`
	PriceCents  int       `json:"price_cents"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListProducts devolve uma página do catálogo, mais novos primeiro (por id).
func (s *Store) ListProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sku, name, category, price_cents, stock, created_at
		FROM products
		ORDER BY id DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listar produtos: %w", err)
	}
	return collectProducts(rows)
}

// GetProduct devolve um produto, com a descrição.
func (s *Store) GetProduct(ctx context.Context, id int64) (Product, error) {
	var p Product
	err := s.pool.QueryRow(ctx, `
		SELECT id, sku, name, description, category, price_cents, stock, created_at
		FROM products
		WHERE id = $1`, id).
		Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category, &p.PriceCents, &p.Stock, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("buscar o produto %d: %w", id, err)
	}
	return p, nil
}

// SearchProducts devolve os produtos cujo nome contém term (sem diferenciar
// maiúsculas), os mais recentes primeiro.
func (s *Store) SearchProducts(ctx context.Context, term string, limit int) ([]Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sku, name, category, price_cents, stock, created_at
		FROM products
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2`, escapeLike(term), limit)
	if err != nil {
		return nil, fmt.Errorf("buscar produtos: %w", err)
	}
	return collectProducts(rows)
}

// escapeLike faz os curingas do LIKE no texto do usuário valerem literalmente.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func collectProducts(rows pgx.Rows) ([]Product, error) {
	products, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Product, error) {
		var p Product
		err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Category, &p.PriceCents, &p.Stock, &p.CreatedAt)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("ler produtos: %w", err)
	}
	return products, nil
}
