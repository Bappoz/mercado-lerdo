// Package telemetry moves host-side measurements into the metrics store and
// queries it back. Samples are pushed to Prometheus' OTLP receiver as
// OTLP/HTTP JSON, so no protobuf dependency is needed (ADR-0014).
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
)

// Kind is the OTLP data type of a sample.
type Kind int

// Sample kinds.
const (
	Gauge   Kind = iota
	Counter      // cumulative, monotonic sum
)

// Sample is one data point.
type Sample struct {
	Name   string
	Kind   Kind
	Value  float64
	Labels map[string]string
	At     time.Time
}

// FromResourceStats turns container stats into samples named sesim_container_*.
func FromResourceStats(stats []sandbox.ResourceStats) []Sample {
	var out []Sample
	for _, s := range stats {
		l := map[string]string{"container": s.Component, "replica": strconv.Itoa(s.Replica)}
		add := func(name string, k Kind, v float64) {
			out = append(out, Sample{Name: "sesim_container_" + name, Kind: k, Value: v, Labels: l, At: s.At})
		}
		add("cpu_usage_seconds_total", Counter, s.CPUUsageSeconds)
		add("cpu_quota_cores", Gauge, s.CPUQuotaCores)
		add("cpu_periods_total", Counter, float64(s.CPUPeriods))
		add("cpu_throttled_periods_total", Counter, float64(s.ThrottledPeriods))
		add("cpu_throttled_seconds_total", Counter, s.ThrottledSeconds)
		add("memory_working_set_bytes", Gauge, float64(s.MemoryBytes))
		add("memory_limit_bytes", Gauge, float64(s.MemoryLimitBytes))
		add("pids", Gauge, float64(s.Pids))
		add("network_receive_bytes_total", Counter, float64(s.NetRxBytes))
		add("network_transmit_bytes_total", Counter, float64(s.NetTxBytes))
	}
	return out
}

// OTLP wire types (subset of opentelemetry-proto, JSON mapping).
type (
	otlpRequest struct {
		ResourceMetrics []otlpResourceMetrics `json:"resourceMetrics"`
	}
	otlpResourceMetrics struct {
		Resource     otlpResource       `json:"resource"`
		ScopeMetrics []otlpScopeMetrics `json:"scopeMetrics"`
	}
	otlpResource struct {
		Attributes []otlpKV `json:"attributes"`
	}
	otlpScopeMetrics struct {
		Metrics []otlpMetric `json:"metrics"`
	}
	otlpMetric struct {
		Name  string     `json:"name"`
		Gauge *otlpGauge `json:"gauge,omitempty"`
		Sum   *otlpSum   `json:"sum,omitempty"`
	}
	otlpGauge struct {
		DataPoints []otlpPoint `json:"dataPoints"`
	}
	otlpSum struct {
		DataPoints             []otlpPoint `json:"dataPoints"`
		AggregationTemporality int         `json:"aggregationTemporality"` // 2 = cumulative
		IsMonotonic            bool        `json:"isMonotonic"`
	}
	otlpPoint struct {
		Attributes   []otlpKV `json:"attributes"`
		TimeUnixNano string   `json:"timeUnixNano"` // uint64 encoded as string in OTLP JSON
		AsDouble     float64  `json:"asDouble"`
	}
	otlpKV struct {
		Key   string       `json:"key"`
		Value otlpAnyValue `json:"value"`
	}
	otlpAnyValue struct {
		StringValue string `json:"stringValue"`
	}
)

// EncodeOTLP groups samples by metric name into one OTLP/JSON request.
// service becomes the Prometheus "job" label.
func EncodeOTLP(service string, samples []Sample) ([]byte, error) {
	byName := map[string]*otlpMetric{}
	var names []string
	for _, s := range samples {
		m, ok := byName[s.Name]
		if !ok {
			m = &otlpMetric{Name: s.Name}
			if s.Kind == Counter {
				m.Sum = &otlpSum{AggregationTemporality: 2, IsMonotonic: true}
			} else {
				m.Gauge = &otlpGauge{}
			}
			byName[s.Name] = m
			names = append(names, s.Name)
		}
		p := otlpPoint{Attributes: attrs(s.Labels), TimeUnixNano: strconv.FormatInt(s.At.UnixNano(), 10), AsDouble: s.Value}
		if m.Sum != nil {
			m.Sum.DataPoints = append(m.Sum.DataPoints, p)
		} else {
			m.Gauge.DataPoints = append(m.Gauge.DataPoints, p)
		}
	}
	metrics := make([]otlpMetric, 0, len(names))
	for _, n := range names {
		metrics = append(metrics, *byName[n])
	}
	req := otlpRequest{ResourceMetrics: []otlpResourceMetrics{{
		Resource:     otlpResource{Attributes: []otlpKV{{Key: "service.name", Value: otlpAnyValue{service}}}},
		ScopeMetrics: []otlpScopeMetrics{{Metrics: metrics}},
	}}}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("telemetry: encode otlp: %w", err)
	}
	return b, nil
}

func attrs(labels map[string]string) []otlpKV {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	slices.Sort(keys) // stable output
	out := make([]otlpKV, 0, len(keys))
	for _, k := range keys {
		out = append(out, otlpKV{Key: k, Value: otlpAnyValue{labels[k]}})
	}
	return out
}

// Pusher sends samples to an OTLP/HTTP metrics endpoint.
type Pusher struct {
	URL     string // e.g. http://127.0.0.1:9090/api/v1/otlp/v1/metrics
	Service string
	HTTP    *http.Client
}

// Push encodes and sends samples in one request.
func (p Pusher) Push(ctx context.Context, samples []Sample) error {
	if len(samples) == 0 {
		return nil
	}
	body, err := EncodeOTLP(p.Service, samples)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telemetry: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	hc := p.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("telemetry: push: %w", err)
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telemetry: push: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	return nil
}
