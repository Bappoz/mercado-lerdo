package workload

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseK6Summary(t *testing.T) {
	f, err := os.Open("testdata/k6-normal-day.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := ParseK6Summary(f)
	if err != nil {
		t.Fatal(err)
	}

	if res.Iterations != 51 || res.DroppedIterations != 0 {
		t.Errorf("iterations=%d dropped=%d, want 51 and 0", res.Iterations, res.DroppedIterations)
	}
	if res.Duration < 10*time.Second || res.Duration > 12*time.Second {
		t.Errorf("duration = %s, want ~10.9s", res.Duration)
	}
	var names []string
	total := 0
	for _, e := range res.Endpoints {
		names = append(names, e.Name)
		total += e.Requests
		if e.P50 > e.P95 || e.P95 > e.P99 || e.P99 > e.Max {
			t.Errorf("%s: percentiles not monotonic: %+v", e.Name, e)
		}
	}
	if got := strings.Join(names, ","); got != "checkout,product,products,search" {
		t.Errorf("endpoints = %s", got)
	}
	if total != res.Iterations { // one request per iteration in these workloads
		t.Errorf("sum of endpoint requests = %d, want %d", total, res.Iterations)
	}
	search := res.Endpoints[3]
	if search.Requests != 4 || search.P50 < 300*time.Millisecond {
		t.Errorf("search = %+v, want 4 slow requests", search)
	}
}

func TestParseK6SummaryRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "not json", "{}"} {
		if _, err := ParseK6Summary(strings.NewReader(in)); err == nil {
			t.Errorf("ParseK6Summary(%q) = nil error", in)
		}
	}
}

func TestFailRate(t *testing.T) {
	if r := (Endpoint{}).FailRate(); r != 0 {
		t.Errorf("empty endpoint fail rate = %v", r)
	}
	if r := (Endpoint{Requests: 4, Failed: 1}).FailRate(); r != 0.25 {
		t.Errorf("fail rate = %v, want 0.25", r)
	}
}

func TestDropCause(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want Drops
	}{
		{"no drops", Result{MaxVUs: 10, VUCap: 70}, NoDrops},
		{"all VUs busy", Result{DroppedIterations: 5, MaxVUs: 70, VUCap: 70}, SystemSaturated},
		{"VUs to spare", Result{DroppedIterations: 5, MaxVUs: 41, VUCap: 70}, GeneratorLimited},
		{"unknown cap", Result{DroppedIterations: 5, MaxVUs: 41}, GeneratorLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.DropCause(); got != tt.want {
				t.Errorf("DropCause() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteTableExplainsDrops(t *testing.T) {
	var b strings.Builder
	Result{DroppedIterations: 3, MaxVUs: 70, VUCap: 70}.WriteTable(&b)
	if !strings.Contains(b.String(), "SATURATED") {
		t.Errorf("missing saturation note:\n%s", b.String())
	}
	b.Reset()
	Result{DroppedIterations: 3, MaxVUs: 10, VUCap: 70}.WriteTable(&b)
	if !strings.Contains(b.String(), "generator fell behind") {
		t.Errorf("missing generator warning:\n%s", b.String())
	}
}
