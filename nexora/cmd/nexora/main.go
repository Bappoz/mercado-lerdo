// Command nexora serves the Mercado Lerdo marketplace API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nexora/internal/api"
	"nexora/internal/metrics"
	"nexora/internal/payment"
	"nexora/internal/store"
)

type config struct {
	addr        string
	metricsAddr string
	databaseURL string
	paymentURL  string
	dbMaxConns  int32
}

func loadConfig() (config, error) {
	c := config{
		addr:        getenv("NEXORA_ADDR", ":8080"),
		metricsAddr: getenv("NEXORA_METRICS_ADDR", ":9091"),
		databaseURL: os.Getenv("NEXORA_DATABASE_URL"),
		paymentURL:  getenv("NEXORA_PAYMENT_URL", "http://payments:8090"),
	}
	if c.databaseURL == "" {
		return c, errors.New("NEXORA_DATABASE_URL is required")
	}
	maxConns, err := strconv.ParseInt(getenv("NEXORA_DB_MAX_CONNS", "10"), 10, 32)
	if err != nil || maxConns < 1 {
		return c, fmt.Errorf("NEXORA_DB_MAX_CONNS must be a positive integer")
	}
	c.dbMaxConns = int32(maxConns)
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("nexora stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	poolCfg.MaxConns = cfg.dbMaxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	m := metrics.New(pool)
	srv := api.NewServer(store.New(pool), pool, payment.NewClient(cfg.paymentURL, 10*time.Second), m, log)

	apiServer := &http.Server{Addr: cfg.addr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second}
	metricsServer := &http.Server{Addr: cfg.metricsAddr, Handler: m.Handler(), ReadHeaderTimeout: 5 * time.Second}

	errc := make(chan error, 2)
	go func() { errc <- apiServer.ListenAndServe() }()
	go func() { errc <- metricsServer.ListenAndServe() }()
	log.Info("nexora listening", "addr", cfg.addr, "metrics_addr", cfg.metricsAddr, "db_max_conns", cfg.dbMaxConns)

	select {
	case err := <-errc:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(apiServer.Shutdown(shutdownCtx), metricsServer.Shutdown(shutdownCtx))
}
