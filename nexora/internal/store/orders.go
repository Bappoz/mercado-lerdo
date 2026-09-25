package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrOutOfStock is returned when an item cannot be fulfilled.
var ErrOutOfStock = errors.New("store: out of stock")

// Item is one line of a checkout request.
type Item struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

// Order is a placed order.
type Order struct {
	ID         int64     `json:"id"`
	OrderRef   string    `json:"order_ref"`
	Status     string    `json:"status"`
	TotalCents int       `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

// ChargeFunc charges the customer for amountCents; it is called once the
// order total is known.
type ChargeFunc func(ctx context.Context, orderRef string, amountCents int) error

// PlaceOrder reserves stock, charges the customer and records the order.
func (s *Store) PlaceOrder(ctx context.Context, userID int64, orderRef string, items []Item, charge ChargeFunc) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("begin checkout: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	total := 0
	prices := make([]int, len(items))
	for i, it := range items {
		var price, stock int
		err := tx.QueryRow(ctx,
			`SELECT price_cents, stock FROM products WHERE id = $1 FOR UPDATE`, it.ProductID).
			Scan(&price, &stock)
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, fmt.Errorf("product %d: %w", it.ProductID, ErrNotFound)
		}
		if err != nil {
			return Order{}, fmt.Errorf("lock product %d: %w", it.ProductID, err)
		}
		if stock < it.Quantity {
			return Order{}, fmt.Errorf("product %d: %w", it.ProductID, ErrOutOfStock)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock = stock - $1 WHERE id = $2`, it.Quantity, it.ProductID); err != nil {
			return Order{}, fmt.Errorf("reserve product %d: %w", it.ProductID, err)
		}
		prices[i] = price
		total += price * it.Quantity
	}

	if err := charge(ctx, orderRef, total); err != nil {
		return Order{}, fmt.Errorf("charge: %w", err)
	}

	o := Order{OrderRef: orderRef, Status: "paid", TotalCents: total}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, order_ref, status, total_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`, userID, orderRef, o.Status, total).Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}
	for i, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, product_id, quantity, price_cents)
			VALUES ($1, $2, $3, $4)`, o.ID, it.ProductID, it.Quantity, prices[i]); err != nil {
			return Order{}, fmt.Errorf("insert order item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("commit checkout: %w", err)
	}
	return o, nil
}

// OrdersByUser lists a user's orders, newest first.
func (s *Store) OrdersByUser(ctx context.Context, userID int64, limit int) ([]Order, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, order_ref, status, total_cents, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("orders by user: %w", err)
	}
	orders, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Order])
	if err != nil {
		return nil, fmt.Errorf("scan orders: %w", err)
	}
	return orders, nil
}
