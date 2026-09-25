package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQueryScalar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch q := r.URL.Query().Get("query"); q {
		case "up":
			if r.URL.Query().Get("time") != "1790000000" {
				http.Error(w, "missing time", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1790000000,"0.93"]}]}}`))
		case "by_name":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"name":"search"},"value":[1,"4.5"]},{"metric":{"name":"products"},"value":[1,"0.002"]}]}}`))
		case "empty":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"status":"error","error":"parse error"}`))
		}
	}))
	defer srv.Close()
	ctx, at := context.Background(), time.Unix(1_790_000_000, 0)

	if v, err := QueryScalar(ctx, srv.Client(), srv.URL, "up", at); err != nil || v != 0.93 {
		t.Errorf("up = %v, %v", v, err)
	}
	if _, err := QueryScalar(ctx, srv.Client(), srv.URL, "empty", at); !errors.Is(err, ErrNoData) {
		t.Errorf("empty err = %v, want ErrNoData", err)
	}
	if _, err := QueryScalar(ctx, srv.Client(), srv.URL, "bad(", at); err == nil || !strings.Contains(err.Error(), "parse error") {
		t.Errorf("bad query err = %v", err)
	}

	vec, err := QueryVector(ctx, srv.Client(), srv.URL, "by_name", at)
	if err != nil || len(vec) != 2 || vec[0].Labels["name"] != "search" || vec[0].Value != 4.5 {
		t.Errorf("QueryVector = %+v, %v", vec, err)
	}
	if vec, err := QueryVector(ctx, srv.Client(), srv.URL, "empty", at); err != nil || len(vec) != 0 {
		t.Errorf("empty vector = %+v, %v", vec, err)
	}
}

func TestQueryRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/v1/query_range" || q.Get("start") != "100" || q.Get("end") != "160" || q.Get("step") != "30" {
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		switch q.Get("query") {
		case "p95":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[` +
				`{"metric":{"name":"search"},"values":[[100,"4.5"],[130,"NaN"],[160,"+Inf"]]},` +
				`{"metric":{"name":"products"},"values":[[100,"0.002"]]}]}}`))
		default:
			_, _ = w.Write([]byte(`{"status":"error","error":"parse error"}`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	start, end := time.Unix(100, 0), time.Unix(160, 0)

	got, err := QueryRange(ctx, srv.Client(), srv.URL, "p95", start, end, 30*time.Second)
	if err != nil || len(got) != 2 {
		t.Fatalf("QueryRange = %+v, %v", got, err)
	}
	if got[0].Labels["name"] != "search" || len(got[0].Points) != 1 || got[0].Points[0] != [2]float64{100, 4.5} {
		t.Errorf("search series = %+v, want only the finite point", got[0])
	}
	if _, err := QueryRange(ctx, srv.Client(), srv.URL, "bad(", start, end, 30*time.Second); err == nil {
		t.Error("bad query: want error")
	}
}
