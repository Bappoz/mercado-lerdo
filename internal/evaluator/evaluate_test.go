package evaluator

import (
	"context"
	"encoding/json"
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
