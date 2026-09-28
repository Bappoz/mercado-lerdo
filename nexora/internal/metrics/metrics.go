// Package metrics expõe métricas de HTTP e do pool do banco no formato de texto
// do Prometheus (https://prometheus.io/docs/instrumenting/exposition_formats/).
// É escrito à mão de propósito: pequeno, sem dependências e fácil de ler.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

type requestKey struct {
	route string
	code  int
}

type histogram struct {
	counts []uint64 // por bucket, não cumulativo
	sum    float64
	count  uint64
}

// Metrics coleta as métricas das requisições e lê as estatísticas do pool a cada coleta.
type Metrics struct {
	pool      *pgxpool.Pool
	mu        sync.Mutex
	requests  map[requestKey]uint64
	durations map[string]*histogram
}

// New devolve um coletor vazio para pool.
func New(pool *pgxpool.Pool) *Metrics {
	return &Metrics{pool: pool, requests: map[requestKey]uint64{}, durations: map[string]*histogram{}}
}

// Observe registra uma requisição terminada.
func (m *Metrics) Observe(route string, code int, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[requestKey{route, code}]++
	h := m.durations[route]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(buckets))}
		m.durations[route] = h
	}
	secs := elapsed.Seconds()
	if i, _ := slices.BinarySearch(buckets, secs); i < len(buckets) {
		h.counts[i]++
	}
	h.sum += secs
	h.count++
}

// Handler serve o endpoint de métricas.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.Render(w)
	})
}

// Render escreve todas as métricas no formato de exposição.
func (m *Metrics) Render(w io.Writer) {
	m.mu.Lock()
	reqKeys := make([]requestKey, 0, len(m.requests))
	for k := range m.requests {
		reqKeys = append(reqKeys, k)
	}
	slices.SortFunc(reqKeys, func(a, b requestKey) int {
		return strings.Compare(fmt.Sprint(a.route, a.code), fmt.Sprint(b.route, b.code))
	})
	header(w, "nexora_http_requests_total", "counter", "Requisições HTTP atendidas, por rota e status.")
	for _, k := range reqKeys {
		fmt.Fprintf(w, "nexora_http_requests_total{route=\"%s\",code=\"%d\"} %d\n", escape(k.route), k.code, m.requests[k])
	}
	routes := make([]string, 0, len(m.durations))
	for r := range m.durations {
		routes = append(routes, r)
	}
	slices.Sort(routes)
	header(w, "nexora_http_request_duration_seconds", "histogram", "Latência das requisições HTTP, por rota.")
	for _, r := range routes {
		h := m.durations[r]
		var cumulative uint64
		for i, le := range buckets {
			cumulative += h.counts[i]
			fmt.Fprintf(w, "nexora_http_request_duration_seconds_bucket{route=\"%s\",le=\"%g\"} %d\n", escape(r), le, cumulative)
		}
		fmt.Fprintf(w, "nexora_http_request_duration_seconds_bucket{route=\"%s\",le=\"+Inf\"} %d\n", escape(r), h.count)
		fmt.Fprintf(w, "nexora_http_request_duration_seconds_sum{route=\"%s\"} %g\n", escape(r), h.sum)
		fmt.Fprintf(w, "nexora_http_request_duration_seconds_count{route=\"%s\"} %d\n", escape(r), h.count)
	}
	m.mu.Unlock()

	if m.pool != nil {
		s := m.pool.Stat()
		gauge(w, "nexora_db_pool_acquired_conns", "Conexões emprestadas do pool agora.", float64(s.AcquiredConns()))
		gauge(w, "nexora_db_pool_idle_conns", "Conexões ociosas no pool.", float64(s.IdleConns()))
		gauge(w, "nexora_db_pool_total_conns", "Conexões abertas (ociosas + emprestadas + sendo criadas).", float64(s.TotalConns()))
		gauge(w, "nexora_db_pool_max_conns", "Tamanho máximo do pool.", float64(s.MaxConns()))
		counter(w, "nexora_db_pool_acquire_total", "Conexões obtidas do pool com sucesso.", float64(s.AcquireCount()))
		counter(w, "nexora_db_pool_acquire_duration_seconds_total", "Tempo total gasto obtendo conexões.", s.AcquireDuration().Seconds())
		counter(w, "nexora_db_pool_empty_acquire_total", "Pedidos de conexão que esperaram por falta de conexão ociosa.", float64(s.EmptyAcquireCount()))
		counter(w, "nexora_db_pool_empty_acquire_wait_seconds_total", "Tempo total de espera por falta de conexão ociosa.", s.EmptyAcquireWaitTime().Seconds())
		counter(w, "nexora_db_pool_canceled_acquire_total", "Pedidos de conexão cancelados pelo contexto.", float64(s.CanceledAcquireCount()))
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	gauge(w, "go_goroutines", "Número de goroutines.", float64(runtime.NumGoroutine()))
	gauge(w, "go_memstats_heap_alloc_bytes", "Bytes de objetos alocados no heap.", float64(ms.HeapAlloc))
}

func header(w io.Writer, name, kind, help string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
}

func gauge(w io.Writer, name, help string, v float64) {
	header(w, name, "gauge", help)
	fmt.Fprintf(w, "%s %g\n", name, v)
}

func counter(w io.Writer, name, help string, v float64) {
	header(w, name, "counter", help)
	fmt.Fprintf(w, "%s %g\n", name, v)
}

// escape aplica o escape de valores de label do formato de exposição.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`).Replace(s)
}
