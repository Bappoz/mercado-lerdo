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

	"github.com/Bappoz/software-engineering-simulator/internal/workspace"
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
	Sandbox    string    `json:"sandbox"`    // run id of this game's sandbox
	LastCheck  *Signals  `json:"last_check,omitempty"`
	// Canonical is true when this scenario started from its canonical state
	// instead of the player's code from the previous one.
	Canonical bool `json:"canonical,omitempty"`
	// History is every scenario of the campaign this workspace completed.
	History []Finished `json:"history,omitempty"`
	// Story is what happened to each of the scenario's story events (ADR-0022).
	Story map[string]StoryMark `json:"story,omitempty"`
	// What the story's triggers and goals are checked against.
	LastFailAt time.Time            `json:"last_fail_at,omitzero"` // last failed Production Test
	DeployedAt time.Time            `json:"deployed_at,omitzero"`  // last deploy
	CrashedAt  time.Time            `json:"crashed_at,omitzero"`   // when the incident took the store down
	Viewed     map[string]time.Time `json:"viewed,omitempty"`      // last look at "pulse", "support"
}

// StoryMark is one story event's fate: fired, then met or missed (or neither).
type StoryMark struct {
	FiredAt  time.Time `json:"fired_at"`
	MetAt    time.Time `json:"met_at,omitzero"`
	MissedAt time.Time `json:"missed_at,omitzero"`
}

// Finished is one completed scenario of the campaign.
type Finished struct {
	Scenario  string    `json:"scenario"` // pack directory
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartedAt time.Time `json:"started_at"`
	DoneAt    time.Time `json:"done_at"`
	Checks    int       `json:"checks"`
	Tests     int       `json:"tests"`
	Hints     int       `json:"hints"`
	Canonical bool      `json:"canonical,omitempty"`
	// Story is what happened to the scenario's story events: the bonuses of
	// the career (docs/product/universe.md §4) come from it.
	Story map[string]StoryMark `json:"story,omitempty"`
}

// StateDirName is the directory sesim keeps inside a workspace (git-ignored).
const StateDirName = ".sesim"

// StateDir is where sesim keeps its files inside a workspace.
func StateDir(workspace string) string { return filepath.Join(workspace, StateDirName) }

// EventsPath is the product-event log of a workspace.
func EventsPath(workspace string) string { return filepath.Join(StateDir(workspace), "events.jsonl") }

func statePath(workspace string) string { return filepath.Join(StateDir(workspace), "state.json") }

// LoadSession reads the session of workspace.
func LoadSession(workspace string) (*Session, error) {
	s, _, err := LoadSessionInfo(workspace)
	return s, err
}

// LoadSessionInfo is LoadSession plus the identity of the file it read. Save
// always replaces the file, so a file that is not os.SameFile as this one
// holds a session some process saved later.
func LoadSessionInfo(workspace string) (*Session, os.FileInfo, error) {
	f, err := os.Open(statePath(workspace)) //nolint:gosec // G304: the player's own workspace
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNoSession
	}
	if err != nil {
		return nil, nil, fmt.Errorf("play: %w", err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("play: %w", err)
	}
	var s Session
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, nil, fmt.Errorf("play: corrupt session file %s: %w", statePath(workspace), err)
	}
	return &s, fi, nil
}

// StateInfo is the identity of the saved session file (see LoadSessionInfo).
func StateInfo(workspace string) (os.FileInfo, error) {
	return os.Stat(statePath(workspace))
}

// Save writes the session atomically.
func (s *Session) Save() error {
	_, err := s.SaveInfo()
	return err
}

// SaveInfo is Save plus the identity of the file it wrote.
func (s *Session) SaveInfo() (os.FileInfo, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("play: encode session: %w", err)
	}
	if err := os.MkdirAll(StateDir(s.Workspace), 0o750); err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	// A temp file of its own: another process may be saving at the same time.
	f, err := os.CreateTemp(StateDir(s.Workspace), "state-*.json")
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }() // a no-op once renamed
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	fi, err := os.Stat(f.Name()) // the same file once renamed
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	if err := os.Rename(f.Name(), statePath(s.Workspace)); err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	return fi, nil
}

// CreateWorkspace copies base into dst (which must not exist), applies the
// scenario's start patches, makes it a git repository and tags the scenario
// start, so the player's changes are a diff.
func CreateWorkspace(ctx context.Context, base, dst, tag string, start ...string) error {
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("play: %s already exists", dst)
	}
	if err := os.CopyFS(dst, os.DirFS(base)); err != nil {
		return fmt.Errorf("play: copy %s: %w", base, err)
	}
	if err := workspace.Apply(ctx, dst, start...); err != nil {
		return fmt.Errorf("play: scenario start: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dst, ".gitignore"), []byte(".sesim/\n"), 0o600); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	git := func(args ...string) error {
		// The player's global hooks and signing must not run on (or break) the game's own commits.
		full := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "user.name=sesim",
			"-c", "user.email=sesim@localhost", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)
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
