package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
