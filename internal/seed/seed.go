// Package seed generates the Nexora dataset deterministically: the same
// Config always yields the same rows, independent of the machine or run.
package seed

import (
	"fmt"
	"iter"
	"math/rand/v2"
	"strings"
	"time"
)

// Base is the fixed "now" of the dataset; nothing depends on the wall clock.
var Base = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// Config sizes the dataset.
type Config struct {
	Seed     uint64
	Users    int
	Products int
	Orders   int
}

// DefaultConfig is the Stage 1 dataset.
func DefaultConfig() Config {
	return Config{Seed: 4242, Users: 5_000, Products: 100_000, Orders: 20_000}
}

// Validate rejects sizes the generator cannot satisfy.
func (c Config) Validate() error {
	if c.Users < 1 || c.Products < 1 || c.Orders < 0 {
		return fmt.Errorf("seed: users and products must be >= 1 and orders >= 0 (got %+v)", c)
	}
	if c.Users > 99_999 || c.Products > 999_999 {
		return fmt.Errorf("seed: at most 99999 users and 999999 products (got %+v)", c)
	}
	return nil
}

// User is one row of users (password hash is set by the caller).
type User struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
}

// Product is one row of products.
type Product struct {
	ID          int64
	SKU         string
	Name        string
	Description string
	Category    string
	PriceCents  int
	Stock       int
	CreatedAt   time.Time
}

// Order is one row of orders plus its items.
type Order struct {
	ID         int64
	UserID     int64
	OrderRef   string
	TotalCents int
	CreatedAt  time.Time
	Items      []OrderItem
}

// OrderItem is one row of order_items.
type OrderItem struct {
	ProductID  int64
	Quantity   int
	PriceCents int
}

// Each table draws from its own stream so resizing one table never changes another.
const (
	streamUsers uint64 = iota + 1
	streamProducts
	streamOrders
)

func (c Config) rng(stream uint64) *rand.Rand {
	return rand.New(rand.NewPCG(c.Seed, stream)) //nolint:gosec // G404: a seeded PRNG is the point; nothing here is secret
}

// UserRows yields c.Users users with ids 1..n.
func (c Config) UserRows() iter.Seq[User] {
	return func(yield func(User) bool) {
		r := c.rng(streamUsers)
		for i := 1; i <= c.Users; i++ {
			u := User{
				ID:        int64(i),
				Email:     fmt.Sprintf("user%05d@nexora.test", i),
				Name:      pick(r, firstNames) + " " + pick(r, lastNames),
				CreatedAt: Base.Add(-randDuration(r, 730*24*time.Hour)),
			}
			if !yield(u) {
				return
			}
		}
	}
}

// ProductRows yields c.Products products with ids 1..n. created_at is random,
// so id order and creation order differ.
func (c Config) ProductRows() iter.Seq[Product] {
	return func(yield func(Product) bool) {
		r := c.rng(streamProducts)
		for i := 1; i <= c.Products; i++ {
			cat := categories[r.IntN(len(categories))]
			adj, mat, noun := pick(r, adjectives), pick(r, materials), pick(r, cat.nouns)
			p := Product{
				ID:          int64(i),
				SKU:         fmt.Sprintf("NX-%06d", i),
				Name:        fmt.Sprintf("%s %s %s %s", adj, mat, noun, model(r)),
				Description: describe(r, adj, mat, noun, cat.name),
				Category:    cat.name,
				PriceCents:  price(r),
				Stock:       50 + r.IntN(950),
				CreatedAt:   Base.Add(-randDuration(r, 730*24*time.Hour)),
			}
			if !yield(p) {
				return
			}
		}
	}
}

// OrderRows yields c.Orders orders with ids 1..n. prices must hold the price of
// product id i at index i-1 (as produced by ProductRows).
func (c Config) OrderRows(prices []int) iter.Seq[Order] {
	return func(yield func(Order) bool) {
		r := c.rng(streamOrders)
		for i := 1; i <= c.Orders; i++ {
			o := Order{
				ID:        int64(i),
				UserID:    int64(1 + r.IntN(c.Users)),
				OrderRef:  fmt.Sprintf("seed-%08d", i),
				CreatedAt: Base.Add(-randDuration(r, 180*24*time.Hour)),
			}
			n := 1 + r.IntN(3)
			seen := make(map[int64]bool, n)
			for len(o.Items) < n {
				pid := int64(1 + r.IntN(len(prices)))
				if seen[pid] {
					continue
				}
				seen[pid] = true
				it := OrderItem{ProductID: pid, Quantity: 1 + r.IntN(3), PriceCents: prices[pid-1]}
				o.Items = append(o.Items, it)
				o.TotalCents += it.PriceCents * it.Quantity
			}
			if !yield(o) {
				return
			}
		}
	}
}

func pick(r *rand.Rand, words []string) string { return words[r.IntN(len(words))] }

func randDuration(r *rand.Rand, limit time.Duration) time.Duration {
	return time.Duration(r.Int64N(int64(limit/time.Second))) * time.Second
}

func model(r *rand.Rand) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return fmt.Sprintf("%c%d", letters[r.IntN(len(letters))], 10+r.IntN(990))
}

// price is skewed towards cheap items: most below $50, a few up to ~$1000.
func price(r *rand.Rand) int {
	base := 199 + r.IntN(4800)
	if r.IntN(10) == 0 {
		base *= 1 + r.IntN(20)
	}
	return base
}

func describe(r *rand.Rand, adj, mat, noun, category string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A %s %s %s for your %s. ", strings.ToLower(adj), strings.ToLower(mat), strings.ToLower(noun), strings.ToLower(category))
	b.WriteString(pick(r, sentences))
	b.WriteByte(' ')
	b.WriteString(pick(r, sentences))
	return b.String()
}
