// Package metrics exposes HTTP and DB pool metrics in the Prometheus text
// exposition format (https://prometheus.io/docs/instrumenting/exposition_formats/).
// It is hand-written on purpose: small, dependency-free and easy to read.
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
	counts []uint64 // per bucket, non-cumulative
	sum    float64
	count  uint64
}

// Metrics collects request metrics and reads pool stats on scrape.
type Metrics struct {
	pool      *pgxpool.Pool
	mu        sync.Mutex
	requests  map[requestKey]uint64
	durations map[string]*histogram
}

// New returns an empty collector for pool.
func New(pool *pgxpool.Pool) *Metrics {
	return &Metrics{pool: pool, requests: map[requestKey]uint64{}, durations: map[string]*histogram{}}
}

// Observe records one finished request.
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

// Handler serves the metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.Render(w)
	})
}

// Render writes every metric in exposition format.
func (m *Metrics) Render(w io.Writer) {
	m.mu.Lock()
	reqKeys := make([]requestKey, 0, len(m.requests))
	for k := range m.requests {
		reqKeys = append(reqKeys, k)
	}
	slices.SortFunc(reqKeys, func(a, b requestKey) int {
		return strings.Compare(fmt.Sprint(a.route, a.code), fmt.Sprint(b.route, b.code))
	})
	header(w, "nexora_http_requests_total", "counter", "HTTP requests handled, by route pattern and status code.")
	for _, k := range reqKeys {
		fmt.Fprintf(w, "nexora_http_requests_total{route=\"%s\",code=\"%d\"} %d\n", escape(k.route), k.code, m.requests[k])
	}
	routes := make([]string, 0, len(m.durations))
	for r := range m.durations {
		routes = append(routes, r)
	}
	slices.Sort(routes)
	header(w, "nexora_http_request_duration_seconds", "histogram", "HTTP request latency, by route pattern.")
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
		gauge(w, "nexora_db_pool_acquired_conns", "Connections currently checked out of the pool.", float64(s.AcquiredConns()))
		gauge(w, "nexora_db_pool_idle_conns", "Idle connections in the pool.", float64(s.IdleConns()))
		gauge(w, "nexora_db_pool_total_conns", "Open connections (idle + acquired + constructing).", float64(s.TotalConns()))
		gauge(w, "nexora_db_pool_max_conns", "Maximum pool size.", float64(s.MaxConns()))
		counter(w, "nexora_db_pool_acquire_total", "Successful connection acquisitions.", float64(s.AcquireCount()))
		counter(w, "nexora_db_pool_acquire_duration_seconds_total", "Total time spent acquiring connections.", s.AcquireDuration().Seconds())
		counter(w, "nexora_db_pool_empty_acquire_total", "Acquisitions that waited because no idle connection was available.", float64(s.EmptyAcquireCount()))
		counter(w, "nexora_db_pool_empty_acquire_wait_seconds_total", "Total time spent waiting when no idle connection was available.", s.EmptyAcquireWaitTime().Seconds())
		counter(w, "nexora_db_pool_canceled_acquire_total", "Acquisitions canceled by their context.", float64(s.CanceledAcquireCount()))
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	gauge(w, "go_goroutines", "Number of goroutines.", float64(runtime.NumGoroutine()))
	gauge(w, "go_memstats_heap_alloc_bytes", "Bytes of allocated heap objects.", float64(ms.HeapAlloc))
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

// escape applies the exposition-format label value escaping.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`).Replace(s)
}
