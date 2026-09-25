// Command nexora-seed applies Nexora migrations and loads the deterministic
// dataset. It is platform tooling: it runs as the database owner role, never
// inside the player's app.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Bappoz/software-engineering-simulator/internal/seed"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	def := seed.DefaultConfig()
	cfg := def
	var migrations, password string
	var force bool
	fs := flag.NewFlagSet("nexora-seed", flag.ContinueOnError)
	fs.Uint64Var(&cfg.Seed, "seed", def.Seed, "dataset seed")
	fs.IntVar(&cfg.Users, "users", def.Users, "number of users")
	fs.IntVar(&cfg.Products, "products", def.Products, "number of products")
	fs.IntVar(&cfg.Orders, "orders", def.Orders, "number of orders")
	fs.StringVar(&migrations, "migrations", "migrations", "directory with *.sql migrations")
	fs.StringVar(&password, "password", "nexora-demo", "password shared by every seeded user")
	fs.BoolVar(&force, "force", false, "load even if products already has rows (fails on duplicate ids)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	dsn := os.Getenv("NEXORA_SEED_DATABASE_URL")
	if dsn == "" {
		return errors.New("NEXORA_SEED_DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conn, err := connect(ctx, dsn, log)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	applied, err := seed.Migrate(ctx, conn, os.DirFS(migrations))
	if err != nil {
		return err
	}
	log.Info("migrations applied", "versions", applied)

	seeded, err := seed.Seeded(ctx, conn)
	if err != nil {
		return err
	}
	if seeded && !force {
		log.Info("dataset already loaded; skipping (use -force to reload)")
	} else {
		start := time.Now()
		if err := seed.Load(ctx, conn, cfg, password); err != nil {
			return err
		}
		log.Info("dataset loaded", "config", fmt.Sprintf("%+v", cfg), "elapsed_ms", time.Since(start).Milliseconds())
	}
	hash, err := seed.DatasetHash(ctx, conn)
	if err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

// connect retries while the database is still starting.
func connect(ctx context.Context, dsn string, log *slog.Logger) (*pgx.Conn, error) {
	deadline := time.Now().Add(60 * time.Second)
	for {
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("connect: %w", err)
		}
		log.Info("waiting for database", "err", err.Error())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
