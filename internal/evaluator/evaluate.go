package evaluator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

// Env is what the evaluator needs from a running sandbox (implemented by
// internal/stack on the sandbox driver).
type Env interface {
	Prepare(ctx context.Context, workspace string) error
	ResetData(ctx context.Context) error
	Settle(ctx context.Context) error
	RunLoad(ctx context.Context, t scenario.Test, runID string) (workload.Result, error)
	Query(ctx context.Context, promql string, at time.Time) (float64, error)
	URL() string
	HTTP() *http.Client
}

// Run is what one Production Test run measured.
type Run struct {
	ID       string             `json:"id"`
	Values   map[string]float64 `json:"values"`
	Drops    string             `json:"drops"`
	Note     string             `json:"note,omitempty"`
	Duration time.Duration      `json:"duration"`
}

// Report is the outcome of evaluating one workspace against one scenario.
type Report struct {
	Scenario   string             `json:"scenario"`
	Version    string             `json:"version"`
	Workspace  string             `json:"workspace"`
	Started    time.Time          `json:"started"`
	Finished   time.Time          `json:"finished"`
	Runs       []Run              `json:"runs"`
	Oracle     OracleResult       `json:"oracle"`
	Objectives []ObjectiveResult  `json:"objectives"`
	Dimensions map[string]Verdict `json:"dimensions"`
}

// dbCPUQuery is the p90 of Postgres CPU use relative to its quota over a window.
const dbCPUQuery = `quantile_over_time(0.9, (rate(sesim_container_cpu_usage_seconds_total{container="postgres"}[10s]) / on(container) sesim_container_cpu_quota_cores{container="postgres"})[%ds:2s])`

// Options tunes an evaluation.
type Options struct {
	Log *slog.Logger
	// TelemetryLag is how long to wait after a run for the last remote-write
	// and OTLP batches to land before querying the window.
	TelemetryLag time.Duration
}

// Evaluate runs the scenario's Production Test against workspace:
// runs × (reset → load → measure), then the correctness oracle once.
func Evaluate(ctx context.Context, env Env, pack *scenario.Pack, workspace string, opt Options) (Report, error) {
	log := opt.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	m := pack.Manifest
	rep := Report{Scenario: m.Metadata.ID, Version: m.Metadata.Version, Workspace: workspace, Started: time.Now().UTC()}
	if err := env.Prepare(ctx, workspace); err != nil {
		return rep, err
	}
	for i := 1; i <= m.Test.Runs; i++ {
		id := fmt.Sprintf("%s-run%d", rep.Started.Format("20060102T150405"), i)
		log.Info("production test run", "run", i, "of", m.Test.Runs, "id", id)
		if err := env.ResetData(ctx); err != nil {
			return rep, fmt.Errorf("run %d: reset: %w", i, err)
		}
		res, err := env.RunLoad(ctx, m.Test, id)
		if err != nil {
			return rep, fmt.Errorf("run %d: load: %w", i, err)
		}
		end := time.Now()
		run := Run{ID: id, Values: loadValues(res), Drops: dropName(res.DropCause()), Duration: res.Duration}
		if res.DropCause() == workload.GeneratorLimited {
			// The generator fell behind: client-side numbers are not trustworthy.
			run.Values = map[string]float64{}
			run.Note = "generator could not sustain the rate; load measurements discarded"
		}
		select {
		case <-ctx.Done():
			return rep, ctx.Err()
		case <-time.After(opt.TelemetryLag):
		}
		window := int(math.Ceil(res.Duration.Seconds()))
		if v, err := env.Query(ctx, fmt.Sprintf(dbCPUQuery, window), end); err == nil {
			run.Values["db_cpu_p90"] = v
		} else {
			log.Warn("db cpu query failed", "run", i, "err", err)
		}
		rep.Runs = append(rep.Runs, run)
	}

	if err := env.Settle(ctx); err != nil {
		return rep, fmt.Errorf("oracle: %w", err)
	}
	rep.Oracle = RunOracle(ctx, env.HTTP(), env.URL(), CatalogFromSeed(m.Dataset), pack.Oracle)

	for _, o := range m.Objectives {
		var values []float64
		if o.Measure == "oracle_pass_ratio" {
			values = []float64{rep.Oracle.PassRatio()}
		} else {
			for _, r := range rep.Runs {
				if v, ok := r.Values[o.Measure]; ok {
					values = append(values, v)
				}
			}
		}
		rep.Objectives = append(rep.Objectives, Judge(o, values))
	}
	rep.Dimensions = Dimensions(rep.Objectives)
	rep.Finished = time.Now().UTC()
	return rep, nil
}

// loadValues derives the client-side measures from one k6 run. Arrivals the
// saturated system never served count as errors: those users got nothing.
func loadValues(r workload.Result) map[string]float64 {
	v := map[string]float64{}
	var reqs, failed int
	for _, e := range r.Endpoints {
		reqs += e.Requests
		failed += e.Failed
		if e.Name == "search" {
			v["search_p95_ms"] = float64(e.P95) / float64(time.Millisecond)
		}
	}
	if r.DropCause() == workload.SystemSaturated {
		reqs += r.DroppedIterations
		failed += r.DroppedIterations
	}
	if reqs > 0 {
		v["error_rate"] = float64(failed) / float64(reqs)
	}
	return v
}

func dropName(d workload.Drops) string {
	switch d {
	case workload.SystemSaturated:
		return "system-saturated"
	case workload.GeneratorLimited:
		return "generator-limited"
	default:
		return "none"
	}
}

// WriteTable prints a human-readable report.
func (r Report) WriteTable(w io.Writer) {
	fmt.Fprintf(w, "scenario %s@%s · workspace %s · %s\n\n", r.Scenario, r.Version, r.Workspace, r.Finished.Sub(r.Started).Round(time.Second))
	fmt.Fprintf(w, "%-8s %14s %12s %12s  %s\n", "run", "search p95", "errors", "db cpu p90", "drops")
	for i, run := range r.Runs {
		fmt.Fprintf(w, "%-8d %14s %12s %12s  %s %s\n", i+1, fmtVal(run.Values, "search_p95_ms", "%.1f ms"),
			fmtVal(run.Values, "error_rate", "%.2f%%", 100), fmtVal(run.Values, "db_cpu_p90", "%.0f%%", 100), run.Drops, run.Note)
	}
	fmt.Fprintf(w, "\noracle: %d/%d cases passed\n", r.Oracle.Passed, r.Oracle.Cases)
	for _, f := range r.Oracle.Failures {
		fmt.Fprintf(w, "  - %s\n", f)
	}
	fmt.Fprintf(w, "\n%-14s %-13s %-30s %s\n", "dimension", "verdict", "rule", "median")
	for _, o := range r.Objectives {
		fmt.Fprintf(w, "%-14s %-13s %-30s %.4g\n", o.Dimension, o.Verdict, o.Rule, o.Median)
		if o.Reason != "" {
			fmt.Fprintf(w, "  %s\n", o.Reason)
		}
	}
	dims := make([]string, 0, len(r.Dimensions))
	for d := range r.Dimensions {
		dims = append(dims, d)
	}
	slices.Sort(dims)
	var parts []string
	for _, d := range dims {
		parts = append(parts, d+"="+string(r.Dimensions[d]))
	}
	fmt.Fprintf(w, "\nresult: %s\n", strings.Join(parts, " "))
}

func fmtVal(vals map[string]float64, key, format string, scale ...float64) string {
	v, ok := vals[key]
	if !ok {
		return "—"
	}
	if len(scale) > 0 {
		v *= scale[0]
	}
	return fmt.Sprintf(format, v)
}
