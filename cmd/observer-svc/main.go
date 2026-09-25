// Command observer-svc runs the observer-zone services (currently: the
// emulated payment gateway). See docs/architecture/evaluator.md §2.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/observer"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := os.Getenv("OBSERVER_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	srv := &http.Server{Addr: addr, Handler: observer.NewPayments().Handler(), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Info("observer-svc listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("observer-svc stopped", "err", err)
		os.Exit(1)
	}
}
