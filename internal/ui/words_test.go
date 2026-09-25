package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

// The scenario's player texts name tools through placeholders; every view
// must fill all of them and none may leak a CLI command into the UI.
func TestPlayerTextsRenderInBothViews(t *testing.T) {
	files, err := filepath.Glob("../../scenarios/*/*/player/*")
	if err != nil || len(files) == 0 {
		t.Fatalf("no player files: %v", err)
	}
	live := map[string]string{"workspace": "/w", "app_url": "http://a", "prometheus_url": "http://p"}
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // G304: repository fixture
		if err != nil {
			t.Fatal(err)
		}
		for view, words := range map[string]map[string]string{"cli": play.CLIWords, "ui": Words} {
			out := play.RenderBriefing(play.RenderBriefing(string(b), words), live)
			if strings.Contains(out, "{{") {
				t.Errorf("%s (%s): unfilled placeholder in %q", f, view, out[strings.Index(out, "{{"):][:min(40, len(out)-strings.Index(out, "{{"))])
			}
			if view == "ui" && strings.Contains(out, "sesim ") {
				t.Errorf("%s: names a CLI command in the web UI", f)
			}
		}
	}
}
