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
}
