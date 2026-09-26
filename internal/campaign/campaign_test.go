package campaign

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStage1(t *testing.T) {
	s, err := Load("../../scenarios/stage1")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Scenarios) != 3 || s.Scenarios[1].Story["app.replicas"] != 3 {
		t.Fatalf("stage1 = %+v", s.Scenarios)
	}
	for i := range s.Scenarios {
		if _, err := os.Stat(filepath.Join(s.Path(i), "scenario.yaml")); err != nil {
			t.Errorf("step %d: %v", i, err)
		}
	}
	if i, err := s.Index("../../scenarios/stage1/c2-connection-pressure/"); err != nil || i != 1 {
		t.Errorf("Index(c2) = %d, %v", i, err)
	}
	if _, err := StageOf("../../scenarios/stage1/c3-instance-failure"); err != nil {
		t.Errorf("StageOf(c3): %v", err)
	}
	if _, err := s.Index("../../nexora"); !errors.Is(err, ErrNotInStage) {
		t.Errorf("Index(outside) err = %v", err)
	}
}

func TestLoadRejectsUnknownStoryFields(t *testing.T) {
	dir := t.TempDir()
	stage := "apiVersion: sesim.dev/v0alpha1\nkind: Stage\nmetadata: {id: x, title: X}\nscenarios:\n  - dir: a\n    story: {postgres.size: 2}\n"
	if err := os.WriteFile(filepath.Join(dir, File), []byte(stage), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("a story may only change the fields it is allowed to")
	}
}
