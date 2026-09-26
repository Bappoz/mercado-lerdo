package observer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDuplicateChargesAreCounted(t *testing.T) {
	p := NewPayments(Latency{})
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()

	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/charge", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for _, body := range []string{
		`{"order_ref":"ref-1","amount_cents":100}`,
		`{"order_ref":"ref-1","amount_cents":100}`,
		`{"order_ref":"ref-2","amount_cents":50}`,
	} {
		if code := post(body); code != http.StatusCreated {
			t.Fatalf("charge %s: status %d", body, code)
		}
	}
	for _, bad := range []string{`{}`, `{"order_ref":"x","amount_cents":0}`, `not json`} {
		if code := post(bad); code != http.StatusBadRequest {
			t.Errorf("charge %q: status %d, want 400", bad, code)
		}
	}

	if got, want := p.Stats(), (Stats{Charges: 3, OrderRefs: 2, Duplicated: 1}); got != want {
		t.Errorf("Stats() = %+v, want %+v", got, want)
	}

	resp, err := http.Get(srv.URL + "/charges/ref-1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct{ Count int }
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Count != 2 {
		t.Errorf("GET /charges/ref-1 count = %d (err %v), want 2", body.Count, err)
	}
}

func TestLatencyDistribution(t *testing.T) {
	l := Latency{Median: 600 * time.Millisecond, P99: 2500 * time.Millisecond}
	if a, b := l.For("run-1"), l.For("run-1"); a != b {
		t.Fatalf("latency is not deterministic: %v vs %v", a, b)
	}
	var ds []time.Duration
	for i := range 20000 {
		ds = append(ds, l.For(fmt.Sprintf("r-%d", i)))
	}
	slices.Sort(ds)
	med, p99 := ds[len(ds)/2], ds[len(ds)*99/100]
	near := func(got, want time.Duration) bool { return math.Abs(float64(got-want)) < 0.05*float64(want) }
	if !near(med, l.Median) || !near(p99, l.P99) {
		t.Errorf("median %v, p99 %v; want ≈ %v, %v", med, p99, l.Median, l.P99)
	}
	if (Latency{}).For("x") != 0 {
		t.Error("zero latency must not delay")
	}
}

func TestParseLatency(t *testing.T) {
	if l, err := ParseLatency("600ms,2.5s"); err != nil || l.Median != 600*time.Millisecond || l.P99 != 2500*time.Millisecond {
		t.Errorf("ParseLatency = %+v, %v", l, err)
	}
	for _, bad := range []string{"600ms", "2s,1s", "x,y", "0s,1s"} {
		if _, err := ParseLatency(bad); err == nil {
			t.Errorf("ParseLatency(%q) accepted", bad)
		}
	}
}

func TestListByPrefix(t *testing.T) {
	p := NewPayments(Latency{})
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()
	for _, ref := range []string{"runA-1", "runA-1", "runA-2", "runB-1"} {
		resp, err := http.Post(srv.URL+"/charge", "application/json", strings.NewReader(`{"order_ref":"`+ref+`","amount_cents":10}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	got := p.List("runA-")
	for i := range got {
		if got[i].LastAt.IsZero() {
			t.Errorf("%s has no last charge time", got[i].OrderRef)
		}
		got[i].LastAt = time.Time{}
	}
	want := []RefCharges{{OrderRef: "runA-1", Count: 2, AmountCents: 20}, {OrderRef: "runA-2", Count: 1, AmountCents: 10}}
	if !slices.Equal(got, want) {
		t.Errorf("List = %+v, want %+v", got, want)
	}
	resp, err := http.Get(srv.URL + "/charges?prefix=r")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("short prefix status = %d, want 400", resp.StatusCode)
	}
}

func TestIdempotencyKeyChargesOnce(t *testing.T) {
	p := NewPayments(Latency{Median: 50 * time.Millisecond, P99: 50 * time.Millisecond})
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()
	post := func(key string) int {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/charge", strings.NewReader(`{"order_ref":"k-1","amount_cents":100}`))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return 0
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	// Two concurrent requests with the same key, then a late retry.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range 2 {
		wg.Go(func() { codes[i] = post("key-1") })
	}
	wg.Wait()
	slices.Sort(codes)
	if !slices.Equal(codes, []int{http.StatusOK, http.StatusCreated}) {
		t.Errorf("concurrent codes = %v, want one 201 and one replayed 200", codes)
	}
	if code := post("key-1"); code != http.StatusOK {
		t.Errorf("late retry = %d, want 200", code)
	}
	if s := p.Stats(); s.Charges != 1 || s.Duplicated != 0 {
		t.Errorf("stats = %+v, want exactly one charge", s)
	}
	post("") // without a key, a repeat is a new charge
	if s := p.Stats(); s.Charges != 2 || s.Duplicated != 1 {
		t.Errorf("stats without key = %+v, want a duplicate", s)
	}
}
