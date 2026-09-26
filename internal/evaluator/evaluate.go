package evaluator

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
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
	ProbeMS  float64            `json:"db_probe_ms,omitempty"` // Prober, before the load
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
	Host       *HostCheck         `json:"host,omitempty"` // nil when the env cannot probe
}

// dbCPUQuery is the p90 of Postgres CPU use relative to its quota over a window.
const dbCPUQuery = `quantile_over_time(0.9, (rate(sesim_container_cpu_usage_seconds_total{container="postgres"}[10s]) / on(container) sesim_container_cpu_quota_cores{container="postgres"})[%ds:2s])`

// Options tunes an evaluation.
type Options struct {
	Log *slog.Logger
	// TelemetryLag is how long to wait after a run for the last remote-write
	// and OTLP batches to land before querying the window.
	TelemetryLag time.Duration
	// IntegritySettle is how long to wait after a run before reconciling
	// payments: checkouts the clients gave up on may still be running on the
	// server (up to the app's payment timeout).
	IntegritySettle time.Duration
	// ProbeReference and ProbeLimit override DefaultProbeReference and
	// DefaultProbeLimit.
	ProbeReference, ProbeLimit time.Duration
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
		probeMS := 0.0
		if p, ok := env.(Prober); ok {
			// On the fresh database, before the load: how fast this host computes now.
			if d, err := p.DBProbe(ctx); err == nil {
				probeMS = float64(d.Milliseconds())
			} else {
				log.Warn("db probe failed", "run", i, "err", err)
			}
		}
		stopFaults, faultErr := injectFaults(ctx, env, m.Faults, log)
		res, err := env.RunLoad(ctx, m.Test, id)
		stopFaults()
		if err != nil {
			return rep, fmt.Errorf("run %d: load: %w", i, err)
		}
		if err := <-faultErr; err != nil {
			return rep, fmt.Errorf("run %d: fault: %w", i, err)
		}
		end := time.Now()
		run := Run{ID: id, Values: loadValues(res), Drops: dropName(res.DropCause()), Duration: res.Duration, ProbeMS: probeMS}
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
		if needsLedger(m.Objectives) {
			in, err := reconcileRun(ctx, env, id, opt.IntegritySettle)
			if err != nil {
				return rep, fmt.Errorf("run %d: integrity: %w", i, err)
			}
			run.Values["payment_mismatches"] = float64(in.Mismatches())
			run.Values["lost_orders"] = float64(in.Lost)
			run.Values["phantom_orders"] = float64(in.Phantom)
			run.Values["duplicate_charges"] = float64(in.DuplicateCharges)
		}
		window := int(math.Ceil(res.Duration.Seconds()))
		if v, err := env.Query(ctx, fmt.Sprintf(dbCPUQuery, window), end); err == nil {
			run.Values["db_cpu_p90"] = v
		} else {
			log.Warn("db cpu query failed", "run", i, "err", err)
		}
		rep.Runs = append(rep.Runs, run)
	}

	// Correctness is about the code: bring back whatever the faults took down
	// (availability was already measured by the runs).
	settle := env.Settle
	if len(m.Faults) > 0 {
		settle = env.ResetData
	}
	if err := settle(ctx); err != nil {
		return rep, fmt.Errorf("oracle: %w", err)
	}
	rep.Oracle = RunOracle(ctx, env.HTTP(), env.URL(), CatalogFromSeed(m.Dataset), pack.Oracle)

	reference, limit := cmp.Or(opt.ProbeReference, DefaultProbeReference), cmp.Or(opt.ProbeLimit, DefaultProbeLimit)
	rep.Host = hostCheck(rep.Runs, reference, limit)
	for _, o := range m.Objectives {
		var values []float64
		switch {
		case o.Measure == "oracle_pass_ratio":
			values = []float64{rep.Oracle.PassRatio()}
		case summed[o.Measure]:
			// Counts of rare events are a property of the whole test ("no
			// order lost in the Production Test"): judged on the total.
			total := 0.0
			for _, r := range rep.Runs {
				total += r.Values[o.Measure]
			}
			values = []float64{total}
		case o.Measure == topology.CostMeasure:
			// Modeled, not measured: the same for every run.
			t, err := topology.Load(workspace)
			if err != nil {
				return rep, err
			}
			values = []float64{t.MonthlyCost()}
		default:
			for _, r := range rep.Runs {
				if v, ok := r.Values[o.Measure]; ok {
					values = append(values, v)
				}
			}
		}
		rep.Objectives = append(rep.Objectives, excuseSlowHost(o, Judge(o, values), rep.Host))
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
		ms := float64(e.P95) / float64(time.Millisecond)
		switch e.Name {
		case "search":
			v["search_p95_ms"] = ms
		case "products", "product":
			// Browsing: the catalog list and product pages, the slower of the
			// two. Not search (C1's property, noisy under writes because of
			// its GIN index) and not checkout (it includes the gateway's time).
			v["browse_p95_ms"] = max(v["browse_p95_ms"], ms)
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
	fmt.Fprintf(w, "%-8s %14s %12s %12s %10s  %s\n", "run", "search p95", "errors", "db cpu p90", "db probe", "drops")
	for i, run := range r.Runs {
		probe := "—"
		if run.ProbeMS > 0 {
			probe = fmt.Sprintf("%.0f ms", run.ProbeMS)
		}
		fmt.Fprintf(w, "%-8d %14s %12s %12s %10s  %s %s\n", i+1, fmtVal(run.Values, "search_p95_ms", "%.1f ms"),
			fmtVal(run.Values, "error_rate", "%.2f%%", 100), fmtVal(run.Values, "db_cpu_p90", "%.0f%%", 100), probe, run.Drops, run.Note)
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

func needsLedger(objectives []scenario.Objective) bool {
	for _, o := range objectives {
		if integrityMeasures[o.Measure] {
			return true
		}
	}
	return false
}

// reconcileRun compares what the gateway charged with what the system stored
// for one run (the load generator prefixes every order_ref with the run id).
func reconcileRun(ctx context.Context, env Env, runID string, settle time.Duration) (Integrity, error) {
	l, ok := env.(Ledger)
	if !ok {
		return Integrity{}, errors.New("this sandbox cannot read the payment ledger")
	}
	select {
	case <-ctx.Done():
		return Integrity{}, ctx.Err()
	case <-time.After(settle):
	}
	prefix := runID + "-"
	charges, err := l.Charges(ctx, prefix)
	if err != nil {
		return Integrity{}, err
	}
	orders, err := l.PaidOrders(ctx, prefix)
	if err != nil {
		return Integrity{}, err
	}
	return Reconcile(charges, orders), nil
}

// Crasher makes a component's process crash (implemented by internal/stack).
type Crasher interface {
	Crash(ctx context.Context, component string, replica int) error
}

// injectFaults schedules the scenario's faults against the running load. stop
// cancels those not yet due; the channel yields the first error, or nil, once
// every fault has fired or been cancelled.
func injectFaults(ctx context.Context, env Env, faults []scenario.Fault, log *slog.Logger) (stop func(), errs <-chan error) {
	out := make(chan error, 1)
	if len(faults) == 0 {
		out <- nil
		return func() {}, out
	}
	c, ok := env.(Crasher)
	if !ok {
		out <- errors.New("this sandbox cannot inject faults")
		return func() {}, out
	}
	ctx, cancel := context.WithCancel(ctx)
	start := time.Now()
	go func() {
		var first error
		for _, f := range faults {
			select {
			case <-ctx.Done():
				out <- first
				return
			case <-time.After(time.Until(start.Add(f.After))):
			}
			log.Info("fault", "crash", f.Crash, "after", f.After)
			if err := c.Crash(ctx, f.Crash, 1); err != nil && first == nil {
				first = err
			}
		}
		out <- first
	}()
	return cancel, out
}
