package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

func TestReconcile(t *testing.T) {
	charges := map[string]int{"r-1": 1, "r-2": 1, "r-3": 2, "r-5": 1}
	orders := map[string]int{"r-1": 1, "r-3": 1, "r-4": 1, "r-5": 2}
	got := Reconcile(charges, orders)
	want := Integrity{Lost: 1, Phantom: 1, DuplicateCharges: 1, DuplicateOrders: 1} // r-2, r-4, r-3, r-5
	if got != want {
		t.Errorf("Reconcile = %+v, want %+v", got, want)
	}
	if got.Mismatches() != 4 {
		t.Errorf("Mismatches = %d", got.Mismatches())
	}
	if (Reconcile(map[string]int{"a": 1}, map[string]int{"a": 1})).Mismatches() != 0 {
		t.Error("a clean ledger has mismatches")
	}
}

// ledgerEnv is a fakeEnv whose gateway loses one order in the last run.
type ledgerEnv struct {
	fakeEnv
	calls int
}

func (l *ledgerEnv) Charges(context.Context, string) (map[string]int, error) {
	l.calls++
	if l.calls == 3 {
		return map[string]int{"a": 1, "b": 1}, nil
	}
	return map[string]int{"a": 1}, nil
}

func (l *ledgerEnv) PaidOrders(context.Context, string) (map[string]int, error) {
	return map[string]int{"a": 1}, nil
}

func TestIntegrityIsJudgedOnTheWholeTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"products":[]}`))
	}))
	defer srv.Close()
	run := workload.Result{Endpoints: []workload.Endpoint{
		{Name: "products", Requests: 10, P95: 30 * time.Millisecond},
		{Name: "product", Requests: 10, P95: 80 * time.Millisecond},
		{Name: "search", Requests: 10, P95: 900 * time.Millisecond},
	}}
	env := &ledgerEnv{fakeEnv: fakeEnv{srv: srv, results: []workload.Result{run, run, run}}}
	pack := &scenario.Pack{Manifest: scenario.Manifest{
		Test: scenario.Test{Runs: 3},
		Objectives: []scenario.Objective{
			{ID: "i", Dimension: "integrity", Measure: "payment_mismatches", Below: ptr(1)},
			{ID: "b", Dimension: "latency", Measure: "browse_p95_ms", Below: ptr(400)},
		},
	}}
	rep, err := Evaluate(context.Background(), env, pack, "ws", Options{})
	if err != nil {
		t.Fatal(err)
	}
	// One lost order in one run out of three: the test as a whole lost money.
	if got := rep.Objectives[0]; got.Verdict != Fail || !slices.Equal(got.Values, []float64{1}) {
		t.Errorf("integrity = %s %v, want FAIL on the total [1]", got.Verdict, got.Values)
	}
	// Browsing is the slower of list and product; search is not part of it.
	if got := rep.Objectives[1]; got.Verdict != Pass || got.Median != 80 {
		t.Errorf("browse = %s %v, want PASS at 80 ms", got.Verdict, got.Values)
	}
}

type crashEnv struct {
	fakeEnv
	mu      sync.Mutex
	crashed []string
}

func (c *crashEnv) Crash(_ context.Context, component string, replica int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.crashed = append(c.crashed, fmt.Sprintf("%s#%d", component, replica))
	return nil
}

func TestInjectFaults(t *testing.T) {
	env := &crashEnv{}
	faults := []scenario.Fault{{After: 10 * time.Millisecond, Crash: "app"}, {After: 20 * time.Millisecond, Crash: "app"}}
	stop, errs := injectFaults(context.Background(), env, faults, slog.New(slog.DiscardHandler))
	time.Sleep(60 * time.Millisecond)
	stop()
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(env.crashed, []string{"app#1", "app#1"}) {
		t.Errorf("crashed = %v", env.crashed)
	}
	// A load that ends first cancels the faults not yet due.
	env.crashed = nil
	stop, errs = injectFaults(context.Background(), env, []scenario.Fault{{After: time.Hour, Crash: "app"}}, slog.New(slog.DiscardHandler))
	stop()
	if err := <-errs; err != nil || len(env.crashed) != 0 {
		t.Errorf("cancelled fault: err %v, crashed %v", err, env.crashed)
	}
	// Faults need a sandbox that can crash things.
	_, errs = injectFaults(context.Background(), &fakeEnv{}, faults, slog.New(slog.DiscardHandler))
	if err := <-errs; err == nil {
		t.Error("want an error from a sandbox without Crash")
	}
}
