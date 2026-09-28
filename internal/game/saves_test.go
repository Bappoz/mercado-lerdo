package game

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/career"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

func TestSummarize(t *testing.T) {
	root := "../.."
	empty, err := Summarize(root, filepath.Join(t.TempDir(), "workspace-2"))
	if err != nil || !empty.Empty {
		t.Fatalf("a slot with no game: %+v, %v", empty, err)
	}
	ws := t.TempDir()
	s := play.Session{Scenario: filepath.Join(root, DefaultScenario), Workspace: ws, Phase: play.Incident, StartedAt: time.Now().UTC()}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Summarize(root, ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.Empty || got.Scenario != "Busca Lenta" || got.Phase != "incident" || got.Missions != 0 ||
		got.Level != "Júnior I" || got.Balance != career.SigningBonus || got.PlayedAt.IsZero() {
		t.Errorf("summary = %+v", got)
	}
}
