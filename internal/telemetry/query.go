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
	form := url.Values{"query": {promql}, "time": {strconv.FormatInt(at.Unix(), 10)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/query?"+form.Encode(), nil)
	if err != nil {
		return 0, fmt.Errorf("telemetry: query: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("telemetry: query: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []struct {
				Value [2]any `json:"value"` // [unix time, "value as string"]
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("telemetry: decode query response: %w", err)
	}
	if body.Status != "success" {
		return 0, fmt.Errorf("telemetry: query %q: %s", promql, body.Error)
	}
	if len(body.Data.Result) == 0 {
		return 0, fmt.Errorf("%w: %s", ErrNoData, promql)
	}
	s, ok := body.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("telemetry: unexpected sample value %v", body.Data.Result[0].Value[1])
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("telemetry: parse sample %q: %w", s, err)
	}
	return v, nil
}
