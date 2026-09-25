package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ErrNoData is returned when a query matches no series.
var ErrNoData = errors.New("telemetry: query returned no data")

// QueryScalar runs an instant query against the Prometheus HTTP API at time
// at and returns the value of the first sample.
func QueryScalar(ctx context.Context, hc *http.Client, baseURL, promql string, at time.Time) (float64, error) {
	vec, err := QueryVector(ctx, hc, baseURL, promql, at)
	if err != nil {
		return 0, err
	}
	if len(vec) == 0 {
		return 0, fmt.Errorf("%w: %s", ErrNoData, promql)
	}
	return vec[0].Value, nil
}

// Series is one element of an instant vector.
type Series struct {
	Labels map[string]string
	Value  float64
}

// QueryVector runs an instant query and returns every sample.
func QueryVector(ctx context.Context, hc *http.Client, baseURL, promql string, at time.Time) ([]Series, error) {
	form := url.Values{"query": {promql}, "time": {strconv.FormatInt(at.Unix(), 10)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/query?"+form.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("telemetry: decode query response: %w", err)
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("telemetry: query %q: %s", promql, body.Error)
	}
	out := make([]Series, 0, len(body.Data.Result))
	for _, r := range body.Data.Result {
		s, _ := r.Value[1].(string)
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("telemetry: parse sample %q: %w", s, err)
		}
		out = append(out, Series{Labels: r.Metric, Value: v})
	}
	return out, nil
}

// RangeSeries is one element of a range-query matrix. Points are [unix
// seconds, value]; NaN and infinite samples (e.g. a quantile over no
// requests) are dropped, since they mean "no data" and JSON cannot carry them.
type RangeSeries struct {
	Labels map[string]string
	Points [][2]float64
}

// QueryRange runs a range query over [start, end] every step.
func QueryRange(ctx context.Context, hc *http.Client, baseURL, promql string, start, end time.Time, step time.Duration) ([]RangeSeries, error) {
	form := url.Values{
		"query": {promql},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {strconv.FormatFloat(step.Seconds(), 'f', -1, 64)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/query_range?"+form.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query_range: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query_range: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][2]any          `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("telemetry: decode query_range response: %w", err)
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("telemetry: query_range %q: %s", promql, body.Error)
	}
	out := make([]RangeSeries, 0, len(body.Data.Result))
	for _, r := range body.Data.Result {
		rs := RangeSeries{Labels: r.Metric, Points: make([][2]float64, 0, len(r.Values))}
		for _, p := range r.Values {
			ts, _ := p[0].(float64)
			s, _ := p[1].(string)
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("telemetry: parse sample %q: %w", s, err)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			rs.Points = append(rs.Points, [2]float64{ts, v})
		}
		out = append(out, rs)
	}
	return out, nil
}
