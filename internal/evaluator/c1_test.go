package evaluator

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// TestC1OracleDiscriminates checks the shipped C1 oracle against the real
// dataset: a correct "contains" search passes every case, while the wrong
// semantics of the failing references (prefix, whole word) fail at least one.
func TestC1OracleDiscriminates(t *testing.T) {
	pack, err := scenario.Load("../../scenarios/stage1/c1-slow-search")
	if err != nil {
		t.Fatal(err)
	}
	cat := CatalogFromSeed(pack.Manifest.Dataset)

	contains := func(name, q string) bool { return strings.Contains(strings.ToLower(name), q) }
	prefix := func(name, q string) bool { return strings.HasPrefix(strings.ToLower(name), q) }
	words := func(name, q string) bool { // every query word is a whole word of the name
		nameWords := strings.Fields(strings.ToLower(name))
		for _, w := range strings.Fields(q) {
			if !slices.Contains(nameWords, w) {
				return false
			}
		}
		return true
	}

	failures := func(match func(name, q string) bool) int {
		n := 0
		for _, c := range pack.Oracle {
			if CheckSearch(cat, c, simulate(cat, c, match)) != nil {
				n++
			}
		}
		return n
	}
	if n := failures(contains); n != 0 {
		t.Errorf("correct semantics failed %d case(s)", n)
	}
	if n := failures(prefix); n == 0 {
		t.Error("prefix semantics passed every case: the oracle cannot catch reference c")
	}
	if n := failures(words); n == 0 {
		t.Error("whole-word semantics passed every case: the oracle cannot catch reference d")
	}
}

func simulate(cat Catalog, c scenario.OracleCase, match func(name, q string) bool) SearchResponse {
	q := strings.ToLower(strings.TrimSpace(c.Q))
	var hits Catalog
	for _, e := range cat {
		if match(e.Name, q) {
			hits = append(hits, e)
		}
	}
	slices.SortStableFunc(hits, func(a, b CatalogEntry) int { return b.CreatedAt.Compare(a.CreatedAt) })
	var r SearchResponse
	for _, e := range hits[:min(len(hits), c.Limit)] {
		r.Products = append(r.Products, struct {
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}{e.Name, e.CreatedAt})
	}
	return r
}
