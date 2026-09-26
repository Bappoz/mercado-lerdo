package stack

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// State of a registered run.
type State string

// Run states.
const (
	Creating State = "creating" // Up in progress (owned by PID)
	Ready    State = "ready"    // Up finished; lives until Down
)

// Record is what the host remembers about a run, so GC can tell a live
// sandbox from the leftovers of a crashed or killed sesim.
type Record struct {
	Run       sandbox.RunID `json:"run"`
	PID       int           `json:"pid"`
	State     State         `json:"state"`
	Workspace string        `json:"workspace"`
	Secrets   Secrets       `json:"secrets"` // file is 0600 in the user's state dir
	Created   time.Time     `json:"created"`
	// Topology is what the sandbox runs now (last deploy). Records written
	// before topologies existed have none: they ran the default.
	Topology *topology.Topology `json:"topology,omitempty"`
	// NginxConfHash identifies the nginx configuration the sandbox runs.
	NginxConfHash string `json:"nginx_conf_hash,omitempty"`
}

// Registry stores one JSON file per run.
type Registry struct{ Dir string }

// DefaultRegistry lives in $XDG_STATE_HOME/sesim/runs (~/.local/state/sesim/runs).
func DefaultRegistry() (Registry, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Registry{}, fmt.Errorf("stack: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return Registry{Dir: filepath.Join(base, "sesim", "runs")}, nil
}

func (r Registry) path(run sandbox.RunID) string { return filepath.Join(r.Dir, string(run)+".json") }

// Put writes rec.
func (r Registry) Put(rec Record) error {
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return fmt.Errorf("stack: %w", err)
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("stack: %w", err)
	}
	tmp := r.path(rec.Run) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("stack: %w", err)
	}
	return os.Rename(tmp, r.path(rec.Run))
}

// Get reads the record of run (nil when unknown).
func (r Registry) Get(run sandbox.RunID) (*Record, error) {
	b, err := os.ReadFile(r.path(run))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stack: %w", err)
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, fmt.Errorf("stack: corrupt run record %s: %w", r.path(run), err)
	}
	return &rec, nil
}

// Delete forgets run.
func (r Registry) Delete(run sandbox.RunID) error {
	if err := os.Remove(r.path(run)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stack: %w", err)
	}
	return nil
}

// Orphan reports whether the resources of run can be collected: nobody
// registered them, or their Up never finished and its process is gone.
func Orphan(rec *Record) bool {
	return rec == nil || (rec.State == Creating && !processAlive(rec.PID))
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
