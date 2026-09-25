package evaluator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

var testCatalog = Catalog{
	{"Modern Oak Lamp A10", t0.Add(-1 * time.Hour)},
	{"Rustic Steel Lamp B20", t0.Add(-3 * time.Hour)},
	{"Compact Oak Kettle C30", t0.Add(-2 * time.Hour)},
	{"Lamplighter Stool D40", t0.Add(-4 * time.Hour)}, // substring, not a word
	{"Studio Glass Mug E50", t0.Add(-5 * time.Hour)},
}

func resp(entries ...CatalogEntry) SearchResponse {
	var r SearchResponse
	for _, e := range entries {
		r.Products = append(r.Products, struct {
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}{e.Name, e.CreatedAt})
	}
	return r
}

func TestCheckSearch(t *testing.T) {
	c := scenario.OracleCase{Q: "lamp", Limit: 20}
	tests := []struct {
		name    string
		c       scenario.OracleCase
		got     SearchResponse
		wantErr string
	}{
		{"correct: substring matches, newest first", c, resp(testCatalog[0], testCatalog[1], testCatalog[3]), ""},
		{"whole-word search misses a substring match", c, resp(testCatalog[0], testCatalog[1]), "got 2 results, want 3"},
		{"wrong order", c, resp(testCatalog[1], testCatalog[0], testCatalog[3]), "result 0 created_at"},
		{"non-matching result", c, resp(testCatalog[0], testCatalog[2], testCatalog[3]), "does not contain the term"},
		{"limit applies to the newest", scenario.OracleCase{Q: "lamp", Limit: 1}, resp(testCatalog[0]), ""},
		{"case-insensitive", scenario.OracleCase{Q: "OAK", Limit: 20}, resp(testCatalog[0], testCatalog[2]), ""},
		{"no match", scenario.OracleCase{Q: "drone", Limit: 20}, resp(), ""},
		{"prefix search is not contains", scenario.OracleCase{Q: "oak", Limit: 20}, resp(), "got 0 results, want 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckSearch(testCatalog, tt.c, tt.got)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRunOracle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("q") {
		case "lamp":
			_ = json.NewEncoder(w).Encode(resp(testCatalog[0], testCatalog[1], testCatalog[3]))
		case "oak":
			_ = json.NewEncoder(w).Encode(resp(testCatalog[2])) // missing the newest
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	res := RunOracle(context.Background(), srv.Client(), srv.URL, testCatalog, []scenario.OracleCase{
		{Q: "lamp", Limit: 20}, {Q: "oak", Limit: 20}, {Q: "mug", Limit: 20},
	})
	if res.Cases != 3 || res.Passed != 1 || len(res.Failures) != 2 || res.PassRatio() != 1.0/3 {
		t.Errorf("result = %+v", res)
	}
}

func TestCatalogFromSeedIsDeterministic(t *testing.T) {
	d := scenario.Dataset{Seed: 9, Users: 1, Products: 50, Orders: 0}
	a, b := CatalogFromSeed(d), CatalogFromSeed(d)
	if len(a) != 50 || a[17] != b[17] {
		t.Fatalf("catalog not deterministic: %v vs %v", a[17], b[17])
	}
}
