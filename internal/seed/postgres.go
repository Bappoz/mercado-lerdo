package seed

import (
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io/fs"
	"iter"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Migrate applies every *.sql file in dir (lexical order) that is not yet
// recorded in schema_migrations, each in its own transaction.
func Migrate(ctx context.Context, conn *pgx.Conn, dir fs.FS) ([]string, error) {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	files, err := fs.Glob(dir, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	slices.Sort(files)
	var applied []string
	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		var done bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&done); err != nil {
			return applied, fmt.Errorf("check migration %s: %w", version, err)
		}
		if done {
			continue
		}
		sql, err := fs.ReadFile(dir, name)
		if err != nil {
			return applied, fmt.Errorf("read migration %s: %w", name, err)
		}
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
			return err
		})
		if err != nil {
			return applied, fmt.Errorf("apply migration %s: %w", version, err)
		}
		applied = append(applied, version)
	}
	return applied, nil
}

// Seeded reports whether the products table already has rows.
func Seeded(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var seeded bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products)`).Scan(&seeded)
	if err != nil {
		return false, fmt.Errorf("check seeded: %w", err)
	}
	return seeded, nil
}

// Load writes the whole dataset in one transaction. Every seeded user gets
// the same password, hashed with a salt derived from the seed.
func Load(ctx context.Context, conn *pgx.Conn, c Config, password string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	hash, err := PasswordHash(password, c.Seed)
	if err != nil {
		return err
	}
	prices := make([]int, 0, c.Products)

	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if err := copyRows(ctx, tx, "users", []string{"id", "email", "name", "password_hash", "created_at"},
			c.UserRows(), func(u User) []any { return []any{u.ID, u.Email, u.Name, hash, u.CreatedAt} }); err != nil {
			return err
		}
		if err := copyRows(ctx, tx, "products",
			[]string{"id", "sku", "name", "description", "category", "price_cents", "stock", "created_at"},
			c.ProductRows(), func(p Product) []any {
				prices = append(prices, p.PriceCents)
				return []any{p.ID, p.SKU, p.Name, p.Description, p.Category, p.PriceCents, p.Stock, p.CreatedAt}
			}); err != nil {
			return err
		}
		orders := slices.Collect(c.OrderRows(prices))
		if err := copyRows(ctx, tx, "orders", []string{"id", "user_id", "order_ref", "status", "total_cents", "created_at"},
			slices.Values(orders), func(o Order) []any {
				return []any{o.ID, o.UserID, o.OrderRef, "paid", o.TotalCents, o.CreatedAt}
			}); err != nil {
			return err
		}
		items := func(yield func([]any) bool) {
			for _, o := range orders {
				for _, it := range o.Items {
					if !yield([]any{o.ID, it.ProductID, it.Quantity, it.PriceCents}) {
						return
					}
				}
			}
		}
		if err := copyRows(ctx, tx, "order_items", []string{"order_id", "product_id", "quantity", "price_cents"},
			items, func(row []any) []any { return row }); err != nil {
			return err
		}
		for _, table := range []string{"users", "products", "orders"} {
			if _, err := tx.Exec(ctx, fmt.Sprintf(
				`SELECT setval(pg_get_serial_sequence('%s', 'id'), (SELECT max(id) FROM %s))`, table, table)); err != nil {
				return fmt.Errorf("reset %s sequence: %w", table, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `ANALYZE`); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	return nil
}

func copyRows[T any](ctx context.Context, tx pgx.Tx, table string, cols []string, rows iter.Seq[T], toRow func(T) []any) error {
	next, stop := iter.Pull(rows)
	defer stop()
	_, err := tx.CopyFrom(ctx, pgx.Identifier{table}, cols, pgx.CopyFromFunc(func() ([]any, error) {
		v, ok := next()
		if !ok {
			return nil, nil
		}
		return toRow(v), nil
	}))
	if err != nil {
		return fmt.Errorf("copy %s: %w", table, err)
	}
	return nil
}

// DatasetHash fingerprints the seeded tables; equal data gives an equal hash.
func DatasetHash(ctx context.Context, conn *pgx.Conn) (string, error) {
	if _, err := conn.Exec(ctx, `SET TIME ZONE 'UTC'`); err != nil {
		return "", fmt.Errorf("set time zone: %w", err)
	}
	var h string
	err := conn.QueryRow(ctx, `
		SELECT md5(string_agg(h, '' ORDER BY t)) FROM (
			SELECT 1 AS t, md5(string_agg(md5(x::text), '' ORDER BY id)) AS h FROM users x
			UNION ALL SELECT 2, md5(string_agg(md5(x::text), '' ORDER BY id)) FROM products x
			UNION ALL SELECT 3, md5(string_agg(md5(x::text), '' ORDER BY id)) FROM orders x
			UNION ALL SELECT 4, md5(string_agg(md5(x::text), '' ORDER BY order_id, product_id)) FROM order_items x
		) parts`).Scan(&h)
	if err != nil {
		return "", fmt.Errorf("dataset hash: %w", err)
	}
	return h, nil
}

// PasswordHash mirrors the format of nexora/internal/auth so seeded users can
// log in. The salt comes from the seed to keep the dataset deterministic.
func PasswordHash(password string, seed uint64) (string, error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], seed)
	sum := sha256.Sum256(append([]byte("nexora-seed-salt:"), buf[:]...))
	salt := sum[:16]
	const iterations = 100_000
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return "", fmt.Errorf("seed: password hash: %w", err)
	}
	enc := base64.RawURLEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}
