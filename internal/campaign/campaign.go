// Package campaign strings scenarios into a stage (Stage 1: C1 → C2 → C3).
// A player's workspace carries their own code from one scenario to the next;
// only the story changes it (e.g. "we added instances"), as edits to
// topology.yaml. Alternatively the player adopts the next scenario's
// canonical start (docs/architecture/simulation-engine.md §4).
package campaign

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// File is where a stage is described, next to its scenarios.
const File = "stage.yaml"

// Stage is a stage.yaml.
type Stage struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		ID    string `yaml:"id"`
		Title string `yaml:"title"`
	} `yaml:"metadata"`
	Scenarios []Step `yaml:"scenarios"`
	Dir       string `yaml:"-"`
}

// Step is one scenario of the stage.
type Step struct {
	Dir string `yaml:"dir"` // relative to the stage directory
	// Story sets topology.yaml fields ("app.replicas": 3) when a workspace
	// carrying the player's code enters this scenario.
	Story map[string]int `yaml:"story"`
}

// Load reads <dir>/stage.yaml.
func Load(dir string) (*Stage, error) {
	b, err := os.ReadFile(filepath.Join(dir, File)) //nolint:gosec // G304: the stage the local user pointed at
	if err != nil {
		return nil, fmt.Errorf("campaign: %w", err)
	}
	var s Stage
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("campaign: %s: %w", dir, err)
	}
	if s.Kind != "Stage" || len(s.Scenarios) == 0 {
		return nil, fmt.Errorf("campaign: %s: want kind Stage with scenarios", dir)
	}
	for _, st := range s.Scenarios {
		for field := range st.Story {
			if !storyFields[field] {
				return nil, fmt.Errorf("campaign: %s: %s: story cannot set %q", dir, st.Dir, field)
			}
		}
	}
	s.Dir = dir
	return &s, nil
}

// storyFields are the topology fields a story may change.
var storyFields = map[string]bool{"app.replicas": true, "app.dbMaxConns": true}

// ErrNotInStage means a scenario is not part of the stage.
var ErrNotInStage = errors.New("campaign: scenario is not in this stage")

// Path is the scenario directory of step i.
func (s *Stage) Path(i int) string { return filepath.Join(s.Dir, s.Scenarios[i].Dir) }

// Index finds the step whose scenario is at dir.
func (s *Stage) Index(dir string) (int, error) {
	want, err := filepath.Abs(dir)
	if err != nil {
		return 0, err
	}
	for i := range s.Scenarios {
		if got, err := filepath.Abs(s.Path(i)); err == nil && got == want {
			return i, nil
		}
	}
	return 0, ErrNotInStage
}

// StageOf finds the stage a scenario belongs to: the stage.yaml next to it.
func StageOf(scenarioDir string) (*Stage, error) {
	st, err := Load(filepath.Dir(filepath.Clean(scenarioDir)))
	if err != nil {
		return nil, err
	}
	if _, err := st.Index(scenarioDir); err != nil {
		return nil, err
	}
	return st, nil
}
