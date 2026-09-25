// Package play holds the player's side of a scenario: the session state kept
// in the workspace, the workspace itself, and what the terminal shows.
package play

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Phase of the story.
type Phase string

// Phases, in order.
const (
	Calm      Phase = "calm"
	Incident  Phase = "incident"
	Completed Phase = "completed"
)

// ErrNoSession means the workspace has no session yet.
var ErrNoSession = errors.New("play: no session in this workspace")

// Session is the player's progress, stored in <workspace>/.sesim/state.json.
type Session struct {
	Scenario   string    `json:"scenario"` // pack directory
	Workspace  string    `json:"workspace"`
	Phase      Phase     `json:"phase"`
	StartedAt  time.Time `json:"started_at"`
	IncidentAt time.Time `json:"incident_at,omitzero"`
	DoneAt     time.Time `json:"done_at,omitzero"`
	HintsShown int       `json:"hints_shown"`
	Checks     int       `json:"checks"`
	Tests      int       `json:"tests"`
	Background string    `json:"background"` // testid of the running background traffic
	LastCheck  *Signals  `json:"last_check,omitempty"`
}

// StateDir is where sesim keeps its files inside a workspace (git-ignored).
func StateDir(workspace string) string { return filepath.Join(workspace, ".sesim") }

// EventsPath is the product-event log of a workspace.
func EventsPath(workspace string) string { return filepath.Join(StateDir(workspace), "events.jsonl") }

func statePath(workspace string) string { return filepath.Join(StateDir(workspace), "state.json") }

// LoadSession reads the session of workspace.
func LoadSession(workspace string) (*Session, error) {
	b, err := os.ReadFile(statePath(workspace)) //nolint:gosec // G304: the player's own workspace
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("play: corrupt session file %s: %w", statePath(workspace), err)
	}
	return &s, nil
}

// Save writes the session atomically.
func (s *Session) Save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("play: encode session: %w", err)
	}
	if err := os.MkdirAll(StateDir(s.Workspace), 0o750); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	tmp := statePath(s.Workspace) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	return os.Rename(tmp, statePath(s.Workspace))
}

// CreateWorkspace copies base into dst (which must not exist), makes it a git
// repository and tags the scenario start, so the player's changes are a diff.
func CreateWorkspace(ctx context.Context, base, dst, tag string) error {
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("play: %s already exists", dst)
	}
	if err := os.CopyFS(dst, os.DirFS(base)); err != nil {
		return fmt.Errorf("play: copy %s: %w", base, err)
	}
	if err := os.WriteFile(filepath.Join(dst, ".gitignore"), []byte(".sesim/\n"), 0o600); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	git := func(args ...string) error {
		full := append([]string{"-c", "user.name=sesim", "-c", "user.email=sesim@localhost", "-c", "commit.gpgsign=false"}, args...)
		cmd := exec.CommandContext(ctx, "git", full...) //nolint:gosec // G204: fixed binary and arguments built here
		cmd.Dir = dst
		var errb bytes.Buffer
		cmd.Stderr = &errb
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("play: git %s: %w: %s", args[0], err, errb.String())
		}
		return nil
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-q", "-m", "scenario start"},
		{"tag", tag},
	} {
		if err := git(args...); err != nil {
			return err
		}
	}
	return nil
}
