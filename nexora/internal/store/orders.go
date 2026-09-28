package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrOutOfStock é devolvido quando falta estoque para um item.
var ErrOutOfStock = errors.New("store: estoque insuficiente")

// Item é uma linha de um pedido de checkout.
type Item struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

// Order é um pedido feito.
type Order struct {
	ID         int64     `json:"id"`
	OrderRef   string    `json:"order_ref"`
	Status     string    `json:"status"`
	TotalCents int       `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

// ChargeFunc cobra amountCents do cliente; é chamada quando o total do
// pedido já é conhecido.
type ChargeFunc func(ctx context.Context, orderRef string, amountCents int) error

// PlaceOrder reserva o estoque, cobra o cliente e grava o pedido.
func (s *Store) PlaceOrder(ctx context.Context, userID int64, orderRef string, items []Item, charge ChargeFunc) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("iniciar o checkout: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // não faz nada depois do commit

	total := 0
	prices := make([]int, len(items))
	for i, it := range items {
		var price, stock int
		err := tx.QueryRow(ctx,
			`SELECT price_cents, stock FROM products WHERE id = $1 FOR UPDATE`, it.ProductID).
			Scan(&price, &stock)
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, fmt.Errorf("produto %d: %w", it.ProductID, ErrNotFound)
		}
		if err != nil {
			return Order{}, fmt.Errorf("travar o produto %d: %w", it.ProductID, err)
		}
		if stock < it.Quantity {
			return Order{}, fmt.Errorf("produto %d: %w", it.ProductID, ErrOutOfStock)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock = stock - $1 WHERE id = $2`, it.Quantity, it.ProductID); err != nil {
			return Order{}, fmt.Errorf("reservar o produto %d: %w", it.ProductID, err)
		}
		prices[i] = price
		total += price * it.Quantity
	}

	if err := charge(ctx, orderRef, total); err != nil {
		return Order{}, fmt.Errorf("cobrança: %w", err)
	}

	o := Order{OrderRef: orderRef, Status: "paid", TotalCents: total}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, order_ref, status, total_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`, userID, orderRef, o.Status, total).Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		return Order{}, fmt.Errorf("gravar o pedido: %w", err)
	}
	for i, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, product_id, quantity, price_cents)
			VALUES ($1, $2, $3, $4)`, o.ID, it.ProductID, it.Quantity, prices[i]); err != nil {
			return Order{}, fmt.Errorf("gravar item do pedido: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("confirmar o checkout: %w", err)
	}
	return o, nil
}

// OrdersByUser lista os pedidos de um usuário, mais novos primeiro.
func (s *Store) OrdersByUser(ctx context.Context, userID int64, limit int) ([]Order, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, order_ref, status, total_cents, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("pedidos do usuário: %w", err)
	}
	orders, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Order])
	if err != nil {
		return nil, fmt.Errorf("ler pedidos: %w", err)
	}
	return orders, nil
}
