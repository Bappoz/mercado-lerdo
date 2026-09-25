// Package workload runs the load generator (k6, as a separate process — it is
// AGPL, see ADR-0006/0009) and converts its end-of-test summary into our own
// Result type, so nothing downstream depends on k6's format.
package workload

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// Endpoint holds client-observed stats for requests tagged with one name.
type Endpoint struct {
	Name     string
	Requests int
	Failed   int
	P50      time.Duration
	P90      time.Duration
	P95      time.Duration
	P99      time.Duration
	Max      time.Duration
}

// FailRate is Failed/Requests (0 when there were no requests).
func (e Endpoint) FailRate() float64 {
	if e.Requests == 0 {
		return 0
	}
	return float64(e.Failed) / float64(e.Requests)
}

// Result is the load generator's view of one test run.
type Result struct {
	Duration          time.Duration
	Iterations        int
	DroppedIterations int
	MaxVUs            int        // highest number of VUs in use
	VUCap             int        // VUs available to the scenario (0 if unknown)
	PeakRate          int        // target iterations per second at peak (0 if unknown)
	Endpoints         []Endpoint // sorted by name
}

// Drops classifies dropped iterations.
type Drops int

// Drop causes.
const (
	NoDrops Drops = iota
	// SystemSaturated: every VU was busy waiting on the system (up to the
	// client timeout), so new arrivals had no VU. These are unserved users.
	SystemSaturated
	// GeneratorLimited: iterations dropped while VUs were still free, so the
	// generator itself fell behind. The run is not trustworthy.
	GeneratorLimited
)

// DropCause tells a saturated system from a generator that could not keep up.
func (r Result) DropCause() Drops {
	switch {
	case r.DroppedIterations == 0:
		return NoDrops
	case r.VUCap > 0 && r.MaxVUs >= r.VUCap:
		return SystemSaturated
	default:
		return GeneratorLimited
	}
}

// k6Summary mirrors the parts of k6's handleSummary data we read (verified
// against k6 2.3.0; see testdata/).
type k6Summary struct {
	Metrics map[string]struct {
		Values map[string]float64 `json:"values"`
	} `json:"metrics"`
	State struct {
		TestRunDurationMs float64 `json:"testRunDurationMs"`
	} `json:"state"`
	// Sesim is added by the workloads' handleSummary.
	Sesim struct {
		MaxVUs   int `json:"maxVUs"`
		PeakRate int `json:"peakRate"`
	} `json:"sesim"`
}

// ParseK6Summary reads the JSON printed by the workloads' handleSummary.
// Per-endpoint stats come from sub-metrics tagged {name:<endpoint>}; the
// scripts reference them in thresholds so k6 includes them.
func ParseK6Summary(r io.Reader) (Result, error) {
	var s k6Summary
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return Result{}, fmt.Errorf("workload: decode k6 summary: %w", err)
	}
	if s.Metrics == nil {
		return Result{}, fmt.Errorf("workload: k6 summary has no metrics")
	}
	val := func(metric, key string) float64 { return s.Metrics[metric].Values[key] }
	ms := func(v float64) time.Duration { return time.Duration(v * float64(time.Millisecond)) }

	res := Result{
		Duration:          ms(s.State.TestRunDurationMs),
		Iterations:        int(val("iterations", "count")),
		DroppedIterations: int(val("dropped_iterations", "count")),
		MaxVUs:            int(val("vus_max", "max")),
		VUCap:             s.Sesim.MaxVUs,
		PeakRate:          s.Sesim.PeakRate,
	}
	for key := range s.Metrics {
		name, ok := strings.CutPrefix(key, "http_req_duration{name:")
		if !ok {
			continue
		}
		name = strings.TrimSuffix(name, "}")
		dur := "http_req_duration{name:" + name + "}"
		res.Endpoints = append(res.Endpoints, Endpoint{
			Name:     name,
			Requests: int(val("http_reqs{name:"+name+"}", "count")),
			// For k6 Rate metrics, "passes" counts true values: failed requests.
			Failed: int(val("http_req_failed{name:"+name+"}", "passes")),
			P50:    ms(val(dur, "med")),
			P90:    ms(val(dur, "p(90)")),
			P95:    ms(val(dur, "p(95)")),
			P99:    ms(val(dur, "p(99)")),
			Max:    ms(val(dur, "max")),
		})
	}
	slices.SortFunc(res.Endpoints, func(a, b Endpoint) int { return strings.Compare(a.Name, b.Name) })
	return res, nil
}

// WriteTable prints a human-readable summary.
func (r Result) WriteTable(w io.Writer) {
	fmt.Fprintf(w, "duration %s · peak rate %d/s · iterations %d · dropped %d · VUs %d/%d\n\n",
		r.Duration.Round(time.Second), r.PeakRate, r.Iterations, r.DroppedIterations, r.MaxVUs, r.VUCap)
	fmt.Fprintf(w, "%-10s %8s %8s %9s %9s %9s %9s %9s\n", "endpoint", "reqs", "fail%", "p50", "p90", "p95", "p99", "max")
	for _, e := range r.Endpoints {
		fmt.Fprintf(w, "%-10s %8d %7.2f%% %9s %9s %9s %9s %9s\n", e.Name, e.Requests, e.FailRate()*100,
			fmtDur(e.P50), fmtDur(e.P90), fmtDur(e.P95), fmtDur(e.P99), fmtDur(e.Max))
	}
	switch r.DropCause() {
	case SystemSaturated:
		fmt.Fprintf(w, "\nSATURATED: all %d VUs were waiting on the system; %d arrivals were never served.\n",
			r.VUCap, r.DroppedIterations)
	case GeneratorLimited:
		fmt.Fprintf(w, "\nWARNING: %d iterations dropped while VUs were free — the generator fell behind; results are not trustworthy.\n",
			r.DroppedIterations)
	}
}

func fmtDur(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
}
