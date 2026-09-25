package play

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

func TestSessionRoundTrip(t *testing.T) {
	ws := t.TempDir()
	if _, err := LoadSession(ws); !errors.Is(err, ErrNoSession) {
		t.Fatalf("empty workspace: err = %v, want ErrNoSession", err)
	}
	s := &Session{Workspace: ws, Phase: Incident, StartedAt: time.Now().UTC().Truncate(time.Second), HintsShown: 2}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(ws)
	if err != nil || got.Phase != Incident || got.HintsShown != 2 || !got.StartedAt.Equal(s.StartedAt) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestCreateWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "ws")
	ctx := context.Background()
	if err := CreateWorkspace(ctx, base, dst, "sesim/scenario-start/c1"); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", dst, "tag").Output()
	if err != nil || strings.TrimSpace(string(out)) != "sesim/scenario-start/c1" {
		t.Errorf("tags = %q, %v", out, err)
	}
	if err := CreateWorkspace(ctx, base, dst, "x"); err == nil {
		t.Error("overwrote an existing workspace")
	}
}

func TestNextHint(t *testing.T) {
	hints := []scenario.Hint{{Level: 1, Text: "a"}, {Level: 2, Text: "b"}}
	if h, ok := NextHint(hints, 0); !ok || h.Level != 1 {
		t.Errorf("first hint = %+v %v", h, ok)
	}
	if _, ok := NextHint(hints, 2); ok {
		t.Error("hint beyond the ladder")
	}
}

func TestSignalsWrite(t *testing.T) {
	now := Signals{WindowS: 30, Endpoints: []Endpoint{{Name: "search", RPS: 14, ErrorRate: 0.9, P95s: 4.8}},
		CPU: map[string]float64{"postgres": 1.0, "app": 0.1}}
	prev := &Signals{Endpoints: []Endpoint{{Name: "search", P95s: 0.045}}}
	var b strings.Builder
	now.Write(&b, prev)
	out := b.String()
	for _, want := range []string{"search", "90.0%", "4.80 s", "45 ms", "postgres", "100%", "[####################]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if !math.IsNaN(now.Endpoint("nope").P95s) {
		t.Error("absent endpoint should have NaN p95")
	}
}

func TestRenderBriefing(t *testing.T) {
	got := RenderBriefing("ws={{workspace}} api={{app_url}} prom={{prometheus_url}}",
		map[string]string{"workspace": "/w", "app_url": "http://10.0.0.2:8080", "prometheus_url": ""})
	if want := "ws=/w api=http://10.0.0.2:8080 prom=(indisponível)"; got != want {
		t.Errorf("RenderBriefing = %q, want %q", got, want)
	}
}
