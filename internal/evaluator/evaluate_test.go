package evaluator

import (
	"context"
	"encoding/json"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

type fakeEnv struct {
	srv     *httptest.Server
	results []workload.Result
	cpu     float64
	resets  int
}

func (f *fakeEnv) Prepare(context.Context, string) error { return nil }
func (f *fakeEnv) ResetData(context.Context) error       { f.resets++; return nil }
func (f *fakeEnv) Settle(context.Context) error          { return nil }
func (f *fakeEnv) URL() string                           { return f.srv.URL }
func (f *fakeEnv) HTTP() *http.Client                    { return f.srv.Client() }
func (f *fakeEnv) Query(context.Context, string, time.Time) (float64, error) {
	return f.cpu, nil
}
func (f *fakeEnv) RunLoad(context.Context, scenario.Test, string) (workload.Result, error) {
	r := f.results[0]
	f.results = f.results[1:]
	return r, nil
}

func TestLoadValues(t *testing.T) {
	r := workload.Result{
		DroppedIterations: 10, MaxVUs: 70, VUCap: 70, // saturated
		Endpoints: []workload.Endpoint{
			{Name: "search", Requests: 50, Failed: 40, P95: 5 * time.Second},
			{Name: "products", Requests: 40, Failed: 0},
		},
	}
	v := loadValues(r)
	if v["search_p95_ms"] != 5000 {
		t.Errorf("search_p95_ms = %v", v["search_p95_ms"])
	}
	if want := 50.0 / 100.0; v["error_rate"] != want { // (40 failed + 10 unserved) / (90 + 10)
		t.Errorf("error_rate = %v, want %v", v["error_rate"], want)
	}
}

func TestEvaluate(t *testing.T) {
	cat := CatalogFromSeed(scenario.Dataset{Seed: 3, Users: 1, Products: 200})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(r.URL.Query().Get("q"))
		var out SearchResponse
		for _, e := range cat { // a correct (if slow) implementation
			if strings.Contains(strings.ToLower(e.Name), q) {
				out.Products = append(out.Products, struct {
					Name      string    `json:"name"`
					CreatedAt time.Time `json:"created_at"`
				}{e.Name, e.CreatedAt})
			}
		}
		slices.SortFunc(out.Products, func(a, b struct {
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}) int {
			return b.CreatedAt.Compare(a.CreatedAt)
		})
		if len(out.Products) > 20 {
			out.Products = out.Products[:20]
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	fast := workload.Result{Endpoints: []workload.Endpoint{{Name: "search", Requests: 100, P95: 15 * time.Millisecond}}}
	env := &fakeEnv{srv: srv, results: []workload.Result{fast, fast}, cpu: 0.2}
	pack := &scenario.Pack{
		Manifest: scenario.Manifest{
			Metadata: scenario.Metadata{ID: "t", Version: "1"},
			Dataset:  scenario.Dataset{Seed: 3, Users: 1, Products: 200},
			Test:     scenario.Test{Runs: 2},
			Objectives: []scenario.Objective{
				{ID: "c", Dimension: "correctness", Measure: "oracle_pass_ratio", AtLeast: ptr(1)},
				{ID: "l", Dimension: "latency", Measure: "search_p95_ms", Below: ptr(300), Margin: 0.1},
				{ID: "e", Dimension: "availability", Measure: "error_rate", Below: ptr(0.005)},
				{ID: "r", Dimension: "resources", Measure: "db_cpu_p90", Below: ptr(0.7), Margin: 0.1},
			},
		},
		Oracle: []scenario.OracleCase{{Q: "oak", Limit: 20}, {Q: "ette", Limit: 20}},
	}
	rep, err := Evaluate(context.Background(), env, pack, "ws", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if env.resets != 2 || len(rep.Runs) != 2 {
		t.Errorf("resets=%d runs=%d, want 2 and 2", env.resets, len(rep.Runs))
	}
	for d, v := range rep.Dimensions {
		if v != Pass {
			t.Errorf("%s = %s, want PASS; report: %+v", d, v, rep.Objectives)
		}
	}
}

// probingEnv is a fakeEnv whose database answers the speed probes in order.
type probingEnv struct {
	*fakeEnv
	probes []time.Duration
}

func (p *probingEnv) DBProbe(context.Context) (time.Duration, error) {
	d := p.probes[0]
	p.probes = p.probes[1:]
	return d, nil
}

// A host slower than the floor cannot fail a solution on a near miss: the
// miss may be the machine's. A miss beyond what the machine's slowdown
// explains still fails, and what does not depend on speed keeps its verdict.
func TestSlowHostCannotFailOnSpeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(SearchResponse{}) // wrong: finds nothing
	}))
	defer srv.Close()
	load := func(p95 time.Duration) workload.Result {
		return workload.Result{Endpoints: []workload.Endpoint{{Name: "search", Requests: 100, P95: p95}}}
	}
	pack := &scenario.Pack{
		Manifest: scenario.Manifest{
			Metadata: scenario.Metadata{ID: "t", Version: "1"},
			Dataset:  scenario.Dataset{Seed: 3, Users: 1, Products: 200},
			Test:     scenario.Test{Runs: 3},
			Objectives: []scenario.Objective{
				{ID: "c", Dimension: "correctness", Measure: "oracle_pass_ratio", AtLeast: ptr(1)},
				{ID: "l", Dimension: "latency", Measure: "search_p95_ms", Below: ptr(300), Margin: 0.1},
				{ID: "e", Dimension: "availability", Measure: "error_rate", Below: ptr(0.005)},
				{ID: "r", Dimension: "resources", Measure: "db_cpu_p90", Below: ptr(0.7), Margin: 0.1},
			},
		},
		Oracle: []scenario.OracleCase{{Q: "oak", Limit: 20}},
	}
	// The reference machine probes 100 ms; the floor is 150 ms.
	opt := Options{ProbeReference: 100 * time.Millisecond, ProbeLimit: 150 * time.Millisecond}
	for _, tc := range []struct {
		name   string
		probes []time.Duration
		p95    time.Duration
		want   map[string]Verdict
	}{
		{"fast host, near miss", []time.Duration{100, 110, 90}, 400 * time.Millisecond,
			map[string]Verdict{"correctness": Fail, "latency": Fail, "availability": Pass, "resources": Fail}},
		{"one slow probe is noise", []time.Duration{100, 800, 90}, 400 * time.Millisecond,
			map[string]Verdict{"correctness": Fail, "latency": Fail, "availability": Pass, "resources": Fail}},
		{"2x slower host, near miss", []time.Duration{200, 210, 190}, 400 * time.Millisecond,
			map[string]Verdict{"correctness": Fail, "latency": Inconclusive, "availability": Pass, "resources": Inconclusive}},
		{"2x slower host, gross miss", []time.Duration{200, 210, 190}, 5 * time.Second,
			map[string]Verdict{"correctness": Fail, "latency": Fail, "availability": Pass, "resources": Inconclusive}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.probes {
				tc.probes[i] *= time.Millisecond
			}
			env := &probingEnv{fakeEnv: &fakeEnv{srv: srv, results: []workload.Result{load(tc.p95), load(tc.p95), load(tc.p95)}, cpu: 0.95}, probes: tc.probes}
			rep, err := Evaluate(context.Background(), env, pack, "ws", opt)
			if err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(rep.Dimensions, tc.want) {
				t.Errorf("dimensions = %v, want %v; objectives %+v", rep.Dimensions, tc.want, rep.Objectives)
			}
			for i, r := range rep.Runs {
				if r.ProbeMS != float64(tc.probes[i].Milliseconds()) {
					t.Errorf("run %d probe = %v ms", i, r.ProbeMS)
				}
			}
			excused := slices.Contains(slices.Collect(maps.Values(tc.want)), Inconclusive)
			if rep.Host == nil || rep.Host.Excused != excused {
				t.Errorf("host = %+v, want excused %v", rep.Host, excused)
			}
			for _, o := range rep.Objectives {
				if o.Verdict == Inconclusive && !strings.Contains(o.Reason, "slower") {
					t.Errorf("%s: reason %q does not blame the machine", o.ID, o.Reason)
				}
			}
		})
	}
}

func TestSlowdownTolerance(t *testing.T) {
	objs := []scenario.Objective{
		{ID: "lat", Dimension: "latency", Measure: "search_p95_ms", Below: ptr(300), Margin: 0.1},
		{ID: "cpu", Dimension: "resources", Measure: "db_cpu_p90", Below: ptr(0.8), Margin: 0.1},
		{ID: "cost", Dimension: "cost", Measure: "monthly_cost", Below: ptr(100)},
	}
	rep := Report{Objectives: []ObjectiveResult{
		{ID: "lat", Measure: "search_p95_ms", Values: []float64{20, 33}, Verdict: Pass},
		{ID: "cpu", Measure: "db_cpu_p90", Values: []float64{0.5, 0.55}, Verdict: Pass},
		{ID: "cost", Measure: "monthly_cost", Values: []float64{99}, Verdict: Pass}, // not about speed
	}}
	tol, ok := SlowdownTolerance(objs, rep)
	if !ok || tol.Objective != "cpu" || math.Abs(tol.Slowdown-0.88/0.55) > 1e-9 {
		t.Errorf("tolerance = %+v, %v; want cpu at %.3f", tol, ok, 0.88/0.55)
	}
	if f := FloorFor(tol, 267); math.Abs(f-267*1.6/1.10) > 1e-6 {
		t.Errorf("floor = %v", f)
	}
	rep.Objectives[0].Verdict, rep.Objectives[1].Verdict = Fail, Inconclusive
	if _, ok := SlowdownTolerance(objs, rep); ok {
		t.Error("a report with no speed objective passing has no tolerance")
	}
}
