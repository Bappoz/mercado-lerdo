package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Product is a catalog item.
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

// ListProducts returns one page of the catalog, newest first by id.
func (s *Store) ListProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sku, name, category, price_cents, stock, created_at
		FROM products
		ORDER BY id DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return collectProducts(rows)
}

// GetProduct returns a single product with its description.
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
		return Product{}, fmt.Errorf("get product %d: %w", id, err)
	}
	return p, nil
}

// SearchProducts returns products whose name contains term (case-insensitive),
// most recently added first.
func (s *Store) SearchProducts(ctx context.Context, term string, limit int) ([]Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sku, name, category, price_cents, stock, created_at
		FROM products
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2`, escapeLike(term), limit)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	return collectProducts(rows)
}

// escapeLike makes LIKE wildcards in user input match literally.
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
		return nil, fmt.Errorf("scan products: %w", err)
	}
	return products, nil
}
