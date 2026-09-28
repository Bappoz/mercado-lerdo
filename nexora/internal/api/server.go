// Package api implementa a API HTTP do Mercado Lerdo (contrato: API.md).
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"nexora/internal/metrics"
	"nexora/internal/store"
)

// Charger cobra um cliente pelo gateway de pagamento.
type Charger interface {
	Charge(ctx context.Context, orderRef string, amountCents int) error
}

// Pinger verifica se o banco está acessível.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server liga os handlers às suas dependências.
type Server struct {
	store   *store.Store
	db      Pinger
	payment Charger
	metrics *metrics.Metrics
	log     *slog.Logger
}

// NewServer devolve o servidor da API.
func NewServer(st *store.Store, db Pinger, payment Charger, m *metrics.Metrics, log *slog.Logger) *Server {
	return &Server{store: st, db: db, payment: payment, metrics: m, log: log}
}

// Handler devolve o handler HTTP com as rotas e a instrumentação.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /products", s.listProducts)
	mux.HandleFunc("GET /products/{id}", s.getProduct)
	mux.HandleFunc("GET /search", s.search)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /checkout", s.requireUser(s.checkout))
	mux.HandleFunc("GET /orders", s.requireUser(s.listOrders))
	return s.instrument(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "banco indisponível")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// statusRecorder guarda o status escrito por um handler.
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)
		route := r.Pattern // preenchido pelo ServeMux depois do roteamento; vazio quando nenhuma rota casou
		if route == "" {
			route = "unmatched"
		}
		s.metrics.Observe(route, rec.code, elapsed)
		s.log.Info("requisição",
			"method", r.Method, "route", route, "path", r.URL.Path,
			"status", rec.code, "duration_ms", float64(elapsed.Microseconds())/1000)
	})
}

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorBody{Error: msg})
}
