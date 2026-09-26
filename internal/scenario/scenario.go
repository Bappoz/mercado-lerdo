// Package scenario loads scenario packs: a directory with scenario.yaml,
// workload/oracle files and reference solutions (docs/architecture/simulation-engine.md).
// Format v0alpha1 is unstable until the second playtest.
package scenario

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/Bappoz/software-engineering-simulator/internal/observer"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// APIVersion is the only manifest version this build reads.
const APIVersion = "sesim.dev/v0alpha1"

// Pack is a loaded scenario pack.
type Pack struct {
	Dir      string
	Manifest Manifest
	Oracle   []OracleCase
	Hints    []Hint // empty when the pack has no player section
}

// Manifest is scenario.yaml.
type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Dataset    Dataset  `yaml:"dataset"`
	Test       Test     `yaml:"test"`
	OracleFile string   `yaml:"oracle"`
	// Objectives are properties of the running system. They never mention a
	// technology: the loader rejects any measure it does not know.
	Objectives []Objective `yaml:"objectives"`
	// Player is what the player sees (sesim play); optional for harness-only packs.
	Player *Player `yaml:"player"`
	// Start lists patches (relative to the pack) that turn the workspace base
	// into this scenario's start, in order: e.g. C2 starts where C1 ended.
	// References are patches against the start.
	Start []string `yaml:"start"`
	// World is what happens outside the player's system.
	World World `yaml:"world"`
	// Faults happen during every Production Test run.
	Faults []Fault `yaml:"faults"`
	// Broken is the expected verdict of the untouched scenario start.
	Broken     Expect      `yaml:"broken"`
	References []Reference `yaml:"references"`
}

// World sets the conditions outside the player's reach.
type World struct {
	// PaymentLatency is the emulated gateway's "median,p99" answer time
	// (e.g. "600ms,2.5s"); empty answers at once.
	PaymentLatency string `yaml:"paymentLatency"`
}

// Fault is something the platform does to the player's system on purpose.
type Fault struct {
	// After is when, counted from the start of the run's load.
	After time.Duration `yaml:"after"`
	// Crash names the component whose first replica's process crashes
	// (like a panic or an out-of-memory kill: the runtime sees it die).
	Crash string `yaml:"crash"`
}

// Crashable are the components a fault may crash.
var Crashable = map[string]bool{"app": true}

// Metadata identifies the scenario.
type Metadata struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
	Title   string `yaml:"title"`
}

// Dataset must match what the seed tool loaded (the oracle regenerates it).
type Dataset struct {
	Seed     uint64 `yaml:"seed"`
	Users    int    `yaml:"users"`
	Products int    `yaml:"products"`
	Orders   int    `yaml:"orders"`
}

// Test is the Production Test load profile.
type Test struct {
	Profile  string        `yaml:"profile"` // workloads/nexora/<profile>.js
	Rate     int           `yaml:"rate"`
	Ramp     time.Duration `yaml:"ramp"`
	Duration time.Duration `yaml:"duration"`
	Runs     int           `yaml:"runs"`
}

// Objective is one pass/fail property. Exactly one of Below or AtLeast is set.
type Objective struct {
	ID        string   `yaml:"id"`
	Dimension string   `yaml:"dimension"`
	Measure   string   `yaml:"measure"`
	Below     *float64 `yaml:"below"`
	AtLeast   *float64 `yaml:"atLeast"`
	// Margin is a relative band around the threshold where the verdict is
	// INCONCLUSIVE (docs/architecture/evaluator.md §4).
	Margin float64 `yaml:"margin"`
}

// Expect maps dimension -> expected verdict (PASS | FAIL). Unlisted dimensions are not checked.
type Expect map[string]string

// Reference is a known solution, stored as a patch against the scenario start.
type Reference struct {
	Name   string `yaml:"name"`
	Patch  string `yaml:"patch"`
	Why    string `yaml:"why"`
	Expect Expect `yaml:"expect"`
}

// Player describes the playable side of a scenario.
type Player struct {
	WorkspaceBase string `yaml:"workspaceBase"` // repo-relative app copied into the player's workspace
	Calm          Phase  `yaml:"calm"`
	Growth        Phase  `yaml:"growth"`
	Hints         string `yaml:"hints"`
	Debrief       string `yaml:"debrief"`
}

// Phase is one step of the story: a briefing and the background traffic.
type Phase struct {
	Profile  string `yaml:"profile"`
	Rate     int    `yaml:"rate"`
	Briefing string `yaml:"briefing"`
	// Crash names a component whose first replica crashes when the phase starts.
	Crash string `yaml:"crash"`
}

// Hint is one rung of the hint ladder.
type Hint struct {
	Level int    `yaml:"level"`
	Kind  string `yaml:"kind"`
	Text  string `yaml:"text"`
}

// OracleCase is one correctness probe of the search contract.
type OracleCase struct {
	Q     string `yaml:"q"`
	Limit int    `yaml:"limit"`
}

// Measures the evaluator knows how to produce.
var Measures = map[string]bool{
	"search_p95_ms":     true,
	"error_rate":        true,
	"db_cpu_p90":        true,
	"oracle_pass_ratio": true,
	"browse_p95_ms":     true, // the slower of the catalog list and product pages
	// The gateway's charges reconciled with the stored paid orders (evaluator.Reconcile).
	"payment_mismatches": true,
	topology.CostMeasure: true, // modeled from topology.yaml; cost dimension only
}

// Dimensions a scenario may grade.
var Dimensions = map[string]bool{
	"correctness": true, "latency": true, "availability": true, "integrity": true, "resources": true, "cost": true,
}

// Load reads and validates the pack in dir.
func Load(dir string) (*Pack, error) {
	var m Manifest
	if err := readYAML(filepath.Join(dir, "scenario.yaml"), &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("scenario %s: %w", dir, err)
	}
	p := &Pack{Dir: dir, Manifest: m}
	if err := readYAML(filepath.Join(dir, m.OracleFile), &p.Oracle); err != nil {
		return nil, err
	}
	for _, r := range m.References {
		if _, err := os.Stat(filepath.Join(dir, r.Patch)); err != nil {
			return nil, fmt.Errorf("scenario %s: reference %s: %w", dir, r.Name, err)
		}
	}
	for _, s := range m.Start {
		if _, err := os.Stat(filepath.Join(dir, s)); err != nil {
			return nil, fmt.Errorf("scenario %s: start: %w", dir, err)
		}
	}
	if pl := m.Player; pl != nil {
		for _, f := range []string{pl.Calm.Briefing, pl.Growth.Briefing, pl.Debrief} {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				return nil, fmt.Errorf("scenario %s: player: %w", dir, err)
			}
		}
		if err := readYAML(filepath.Join(dir, pl.Hints), &p.Hints); err != nil {
			return nil, err
		}
		for i, h := range p.Hints {
			if h.Level != i+1 || h.Text == "" {
				return nil, fmt.Errorf("scenario %s: hints must be levels 1..n in order, each with text", dir)
			}
		}
	}
	return p, nil
}

func readYAML(path string, v any) error {
	b, err := os.ReadFile(path) //nolint:gosec // G304: reading the scenario pack the local user pointed at is the purpose
	if err != nil {
		return fmt.Errorf("scenario: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // typos in a manifest must fail loudly
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("scenario: %s: %w", path, err)
	}
	return nil
}

// Validate enforces the format invariants (simulation-engine.md §2).
func (m Manifest) Validate() error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if m.APIVersion != APIVersion || m.Kind != "Scenario" {
		fail("apiVersion/kind must be %s/Scenario", APIVersion)
	}
	if m.Metadata.ID == "" || m.Metadata.Version == "" {
		fail("metadata.id and metadata.version are required")
	}
	if m.Test.Profile == "" || m.Test.Rate <= 0 || m.Test.Duration <= 0 || m.Test.Runs < 1 {
		fail("test needs profile, rate > 0, duration > 0 and runs >= 1")
	}
	if m.OracleFile == "" {
		fail("oracle file is required")
	}
	if len(m.Objectives) == 0 {
		fail("at least one objective is required")
	}
	seen := map[string]bool{}
	for _, o := range m.Objectives {
		switch {
		case seen[o.ID] || o.ID == "":
			fail("objective id %q is empty or duplicated", o.ID)
		case !Dimensions[o.Dimension]:
			fail("objective %s: unknown dimension %q", o.ID, o.Dimension)
		case !Measures[o.Measure]:
			fail("objective %s: unknown measure %q (objectives grade properties, not technologies)", o.ID, o.Measure)
		case (o.Below == nil) == (o.AtLeast == nil):
			fail("objective %s: set exactly one of below/atLeast", o.ID)
		case o.Margin < 0 || o.Margin >= 1:
			fail("objective %s: margin must be in [0, 1)", o.ID)
		case (o.Measure == topology.CostMeasure) != (o.Dimension == "cost"):
			// The topology may say what the system costs, never whether it
			// works: that would grade the presence of a technology.
			fail("objective %s: %s belongs to the cost dimension, and only it", o.ID, topology.CostMeasure)
		}
		seen[o.ID] = true
	}
	passing := 0
	for _, r := range m.References {
		if r.Name == "" || r.Patch == "" {
			fail("reference needs name and patch")
		}
		if r.Expect.AllPass() {
			passing++
		}
	}
	// "Many valid solutions" is enforced by the format.
	if passing < 2 {
		fail("need at least 2 references expected to PASS every dimension, got %d", passing)
	}
	if pl := m.Player; pl != nil {
		if pl.WorkspaceBase == "" || pl.Hints == "" || pl.Debrief == "" {
			fail("player needs workspaceBase, hints and debrief")
		}
		for name, ph := range map[string]Phase{"calm": pl.Calm, "growth": pl.Growth} {
			if ph.Profile == "" || ph.Rate <= 0 || ph.Briefing == "" {
				fail("player.%s needs profile, rate > 0 and briefing", name)
			}
		}
	}
	for i, f := range m.Faults {
		if f.After <= 0 || !Crashable[f.Crash] {
			fail("faults[%d]: needs after > 0 and a crashable component (app)", i)
		}
	}
	if pl := m.Player; pl != nil && pl.Growth.Crash != "" && !Crashable[pl.Growth.Crash] {
		fail("player.growth.crash: %q cannot crash", pl.Growth.Crash)
	}
	if _, err := observer.ParseLatency(m.World.PaymentLatency); err != nil {
		fail("world.paymentLatency: %v", err)
	}
	if len(m.Broken) == 0 || m.Broken.AllPass() {
		fail("broken: the scenario start must be expected to FAIL at least one dimension")
	}
	return errors.Join(errs...)
}

// StartPatches returns the Start patches as paths.
func (p *Pack) StartPatches() []string {
	out := make([]string, len(p.Manifest.Start))
	for i, s := range p.Manifest.Start {
		out[i] = filepath.Join(p.Dir, s)
	}
	return out
}

// AllPass reports whether every listed dimension is expected to PASS.
func (e Expect) AllPass() bool {
	if len(e) == 0 {
		return false
	}
	for _, v := range e {
		if v != "PASS" {
			return false
		}
	}
	return true
}
