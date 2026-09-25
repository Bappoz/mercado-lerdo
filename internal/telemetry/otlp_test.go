package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
)

func TestFromResourceStats(t *testing.T) {
	samples := FromResourceStats([]sandbox.ResourceStats{{Component: "postgres", Replica: 1, CPUUsageSeconds: 3, CPUQuotaCores: 0.5}})
	got := map[string]Sample{}
	for _, s := range samples {
		got[s.Name] = s
	}
	if s := got["sesim_container_cpu_usage_seconds_total"]; s.Kind != Counter || s.Value != 3 || s.Labels["container"] != "postgres" {
		t.Errorf("cpu usage sample = %+v", s)
	}
	if s := got["sesim_container_cpu_quota_cores"]; s.Kind != Gauge || s.Value != 0.5 {
		t.Errorf("quota sample = %+v", s)
	}
	if len(got) != 10 {
		t.Errorf("got %d metrics, want 10", len(got))
	}
}

func TestEncodeOTLP(t *testing.T) {
	at := time.Unix(1_790_000_000, 5)
	body, err := EncodeOTLP("sesim", []Sample{
		{Name: "c_total", Kind: Counter, Value: 1, Labels: map[string]string{"container": "app", "replica": "1"}, At: at},
		{Name: "c_total", Kind: Counter, Value: 2, Labels: map[string]string{"container": "app", "replica": "2"}, At: at},
		{Name: "g", Kind: Gauge, Value: 0.5, Labels: map[string]string{"container": "db"}, At: at},
	})
	if err != nil {
		t.Fatal(err)
	}
	var req otlpRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	rm := req.ResourceMetrics[0]
	if rm.Resource.Attributes[0].Key != "service.name" || rm.Resource.Attributes[0].Value.StringValue != "sesim" {
		t.Errorf("resource = %+v", rm.Resource)
	}
	ms := rm.ScopeMetrics[0].Metrics
	if len(ms) != 2 || ms[0].Name != "c_total" || ms[0].Sum == nil || !ms[0].Sum.IsMonotonic ||
		ms[0].Sum.AggregationTemporality != 2 || len(ms[0].Sum.DataPoints) != 2 {
		t.Fatalf("counter metric = %+v", ms[0])
	}
	if ms[1].Gauge == nil || ms[1].Gauge.DataPoints[0].AsDouble != 0.5 {
		t.Errorf("gauge metric = %+v", ms[1])
	}
	if ts := ms[0].Sum.DataPoints[0].TimeUnixNano; ts != "1790000000000000005" {
		t.Errorf("timeUnixNano = %s", ts)
	}
	if k := ms[0].Sum.DataPoints[0].Attributes; k[0].Key != "container" || k[1].Key != "replica" {
		t.Errorf("attributes not sorted: %+v", k)
	}
}

func TestPush(t *testing.T) {
	var gotType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		if strings.Contains(string(gotBody), "bad") {
			http.Error(w, "rejected", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	p := Pusher{URL: srv.URL, Service: "sesim"}

	if err := p.Push(context.Background(), []Sample{{Name: "ok", Value: 1, At: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	if gotType != "application/json" || !strings.Contains(string(gotBody), `"name":"ok"`) {
		t.Errorf("content-type %q body %s", gotType, gotBody)
	}
	if err := p.Push(context.Background(), []Sample{{Name: "bad", Value: 1, At: time.Now()}}); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Errorf("want error with server message, got %v", err)
	}
	if err := p.Push(context.Background(), nil); err != nil {
		t.Errorf("empty push: %v", err)
	}
}
