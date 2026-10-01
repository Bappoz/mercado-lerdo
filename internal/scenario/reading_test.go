package scenario

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReading(t *testing.T) {
	ok := Reading{ID: "explain", Title: "Como ler um plano", Source: "Documentação do PostgreSQL",
		URL: "https://www.postgresql.org/docs/current/using-explain.html", Lang: "en", Minutes: 20, Why: "Para ler o plano."}
	other := ok
	other.ID = "indices"
	if err := ValidateReading([]Reading{ok, other}); err != nil {
		t.Fatalf("valid reading rejected: %v", err)
	}
	with := func(f func(*Reading)) Reading { r := ok; f(&r); return r }
	for _, tt := range []struct {
		name string
		list []Reading
		want string
	}{
		{"duplicate id", []Reading{ok, ok}, "unique id"},
		{"no title", []Reading{with(func(r *Reading) { r.Title = "" })}, "title, a source and why"},
		{"no why", []Reading{with(func(r *Reading) { r.Why = " " })}, "title, a source and why"},
		{"plain http", []Reading{with(func(r *Reading) { r.URL = "http://example.com/x" })}, "https"},
		{"not a link", []Reading{with(func(r *Reading) { r.URL = "postgresql.org/docs" })}, "https"},
		{"a script", []Reading{with(func(r *Reading) { r.URL = "javascript:alert(1)" })}, "https"},
		{"unknown language", []Reading{with(func(r *Reading) { r.Lang = "fr" })}, "lang"},
		{"no reading time", []Reading{with(func(r *Reading) { r.Minutes = 0 })}, "minutes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateReading(tt.list); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

// Tabby recommends reading at the start of every mission (ADR-0026).
func TestStage1PacksRecommendReading(t *testing.T) {
	dirs, err := filepath.Glob("../../scenarios/stage1/c*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no packs: %v", err)
	}
	for _, d := range dirs {
		p, err := Load(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Reading) < 3 {
			t.Errorf("%s recommends %d readings; a mission has a few", d, len(p.Reading))
		}
	}
}
