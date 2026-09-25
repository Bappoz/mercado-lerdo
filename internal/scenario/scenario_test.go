package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = `apiVersion: sesim.dev/v0alpha1
kind: Scenario
metadata: {id: test/demo, version: 1.0.0, title: Demo}
dataset: {seed: 1, users: 10, products: 100, orders: 5}
test: {profile: flash-sale, rate: 60, ramp: 10s, duration: 30s, runs: 3}
oracle: oracle.yaml
objectives:
  - {id: correct, dimension: correctness, measure: oracle_pass_ratio, atLeast: 1}
  - {id: fast, dimension: latency, measure: search_p95_ms, below: 300, margin: 0.1}
broken: {correctness: PASS, latency: FAIL}
references:
  - {name: a, patch: a.patch, expect: {correctness: PASS, latency: PASS}}
  - {name: b, patch: b.patch, expect: {correctness: PASS, latency: PASS}}
`

func writePack(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"scenario.yaml": manifest,
		"oracle.yaml":   "- {q: lamp, limit: 20}\n",
		"a.patch":       "",
		"b.patch":       "",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadValid(t *testing.T) {
	p, err := Load(writePack(t, validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if p.Manifest.Test.Ramp.Seconds() != 10 || len(p.Oracle) != 1 || p.Oracle[0].Q != "lamp" {
		t.Errorf("unexpected pack: %+v", p)
	}
	if *p.Manifest.Objectives[1].Below != 300 {
		t.Errorf("below = %v", *p.Manifest.Objectives[1].Below)
	}
}

func TestLoadShippedC1(t *testing.T) {
	p, err := Load("../../scenarios/stage1/c1-slow-search")
	if err != nil {
		t.Fatal(err)
	}
	if p.Manifest.Player == nil || len(p.Hints) != 5 || p.Hints[4].Kind != "Implementação" {
		t.Fatalf("player section not loaded: %+v, %d hints", p.Manifest.Player, len(p.Hints))
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name, from, to, wantErr string
	}{
		{"technology as measure", "measure: search_p95_ms", "measure: redis_present", "unknown measure"},
		{"both thresholds", "below: 300, margin", "below: 300, atLeast: 1, margin", "exactly one of below/atLeast"},
		{"single passing reference", "{name: b, patch: b.patch, expect: {correctness: PASS, latency: PASS}}",
			"{name: b, patch: b.patch, expect: {correctness: FAIL}}", "at least 2 references"},
		{"broken passes everything", "broken: {correctness: PASS, latency: FAIL}", "broken: {latency: PASS}", "scenario start must be expected to FAIL"},
		{"unknown field (typo)", "oracle: oracle.yaml", "oracle: oracle.yaml\norcale: x", "field orcale not found"},
		{"missing patch file", "patch: b.patch", "patch: nope.patch", "nope.patch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := strings.Replace(validManifest, tt.from, tt.to, 1)
			if m == validManifest {
				t.Fatalf("test setup: %q not found", tt.from)
			}
			_, err := Load(writePack(t, m))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
