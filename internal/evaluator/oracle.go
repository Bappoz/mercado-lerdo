package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/seed"
)

// Catalog is the ground truth for search: regenerated from the deterministic
// seed, so it cannot be influenced by anything the player does to the database.
type Catalog []CatalogEntry

// CatalogEntry is the part of a product the search contract talks about.
type CatalogEntry struct {
	Name      string
	CreatedAt time.Time
}

// CatalogFromSeed regenerates the seeded products.
// maxOracleBody bounds one search response (20 products are a few KiB).
const maxOracleBody = 1 << 20

func CatalogFromSeed(d scenario.Dataset) Catalog {
	cfg := seed.Config{Seed: d.Seed, Users: d.Users, Products: d.Products, Orders: d.Orders}
	cat := make(Catalog, 0, d.Products)
	for p := range cfg.ProductRows() {
		cat = append(cat, CatalogEntry{Name: p.Name, CreatedAt: p.CreatedAt})
	}
	return cat
}

// SearchResponse is the part of GET /search the oracle reads (nexora/API.md).
type SearchResponse struct {
	Products []struct {
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"products"`
}

// CheckSearch verifies one response against the contract: every result
// contains q (case-insensitive, literal), the count is min(limit, matches),
// and the created_at sequence equals the newest-first top-N of all matches.
// Comparing timestamps instead of ids tolerates ties in created_at.
func CheckSearch(cat Catalog, c scenario.OracleCase, got SearchResponse) error {
	q := strings.ToLower(strings.TrimSpace(c.Q))
	var want []time.Time
	for _, e := range cat {
		if strings.Contains(strings.ToLower(e.Name), q) {
			want = append(want, e.CreatedAt)
		}
	}
	slices.SortFunc(want, func(a, b time.Time) int { return b.Compare(a) })
	want = want[:min(len(want), c.Limit)]

	if len(got.Products) != len(want) {
		return fmt.Errorf("q=%q: veio %d resultados, esperado %d", c.Q, len(got.Products), len(want))
	}
	for i, p := range got.Products {
		if !strings.Contains(strings.ToLower(p.Name), q) {
			return fmt.Errorf("q=%q: o resultado %d %q não contém o termo", c.Q, i, p.Name)
		}
		if p.CreatedAt.Unix() != want[i].Unix() {
			return fmt.Errorf("q=%q: o resultado %d tem created_at %s, esperado %s (ordem errada ou faltou um resultado mais novo)",
				c.Q, i, p.CreatedAt.UTC().Format(time.RFC3339), want[i].UTC().Format(time.RFC3339))
		}
	}
	return nil
}

// OracleResult summarizes the correctness probes.
type OracleResult struct {
	Cases    int      `json:"cases"`
	Passed   int      `json:"passed"`
	Failures []string `json:"failures,omitempty"`
}

// PassRatio is Passed/Cases (0 without cases).
func (r OracleResult) PassRatio() float64 {
	if r.Cases == 0 {
		return 0
	}
	return float64(r.Passed) / float64(r.Cases)
}

// RunOracle sends every case to baseURL and checks the responses.
func RunOracle(ctx context.Context, hc *http.Client, baseURL string, cat Catalog, cases []scenario.OracleCase) OracleResult {
	res := OracleResult{Cases: len(cases)}
	for _, c := range cases {
		if err := probe(ctx, hc, baseURL, cat, c); err != nil {
			res.Failures = append(res.Failures, err.Error())
			continue
		}
		res.Passed++
	}
	return res
}

func probe(ctx context.Context, hc *http.Client, baseURL string, cat Catalog, c scenario.OracleCase) error {
	u := fmt.Sprintf("%s/search?q=%s&limit=%d", baseURL, url.QueryEscape(c.Q), c.Limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("q=%q: %w", c.Q, err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("q=%q: %w", c.Q, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("q=%q: status %s", c.Q, resp.Status)
	}
	var got SearchResponse
	// The player's app writes this body: a runaway one must not exhaust the host.
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOracleBody)).Decode(&got); err != nil {
		return fmt.Errorf("q=%q: decodificar: %w", c.Q, err)
	}
	return CheckSearch(cat, c, got)
}
