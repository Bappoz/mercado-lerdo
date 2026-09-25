package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
	"github.com/Bappoz/software-engineering-simulator/internal/workspace"
)

const defaultScenario = "scenarios/stage1/c1-slow-search"

type evalFlags struct {
	root, scenario string
	runs           int
}

func (f *evalFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.root, "root", ".", "repository root (holds deploy/dev/compose.yml)")
	fs.StringVar(&f.scenario, "scenario", defaultScenario, "scenario pack directory")
	fs.IntVar(&f.runs, "runs", 0, "override the scenario's number of runs (0 = use the scenario's)")
}

func (f *evalFlags) load(stderr io.Writer) (*scenario.Pack, *slog.Logger, bool) {
	pack, err := scenario.Load(f.scenario)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, nil, false
	}
	if f.runs > 0 {
		pack.Manifest.Test.Runs = f.runs
	}
	return pack, slog.New(slog.NewTextHandler(stderr, nil)), true
}

// withSandbox brings up a throwaway sandbox for workspace, runs f and always
// destroys it (and verifies the cleanup) afterwards.
func withSandbox(ctx context.Context, root, workspace string, log *slog.Logger, f func(*stack.Stack) error) error {
	d, err := docker.New()
	if err != nil {
		return err
	}
	defer d.Close()
	reg, err := stack.DefaultRegistry()
	if err != nil {
		return err
	}
	if _, err := stack.GC(ctx, d, reg, false); err != nil {
		log.Warn("gc of orphaned sandboxes failed", "err", err)
	}
	log.Info("creating sandbox", "workspace", workspace)
	s, err := stack.Up(ctx, d, reg, stack.Options{Root: root, Workspace: workspace, PGCPUs: 0.5}, log)
	if s != nil {
		defer func() {
			s.Close()
			if err := stack.Down(context.WithoutCancel(ctx), d, reg, s.Run); err != nil {
				log.Error("destroying the sandbox failed; `sesim sandbox gc --all` cleans up", "run", s.Run, "err", err)
			}
		}()
	}
	if err != nil {
		return err
	}
	return f(s)
}

// runTest: sesim test — Production Test of one workspace. Inside a game (a
// session in ./workspace and no explicit --workspace) it also pauses the
// story's traffic, records events and shows the debrief on PASS.
func runTest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ef evalFlags
	ef.register(fs)
	ws := fs.String("workspace", "nexora", "the app to evaluate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "workspace" })
	if !explicit {
		if _, err := play.LoadSession(defaultWorkspace); err == nil {
			return runPlayerTest(ctx, ef, stdout, stderr)
		}
	}
	pack, log, ok := ef.load(stderr)
	if !ok {
		return 1
	}
	var rep evaluator.Report
	err := withSandbox(ctx, ef.root, *ws, log, func(s *stack.Stack) error {
		var err error
		rep, err = evaluator.Evaluate(ctx, s, pack, *ws, evaluator.Options{Log: log, TelemetryLag: 5 * time.Second})
		return err
	})
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	return reportTest(rep, ef.root, stdout)
}

func reportTest(rep evaluator.Report, root string, stdout io.Writer) int {
	rep.WriteTable(stdout)
	if path, err := game.SaveReport(root, "eval", rep); err == nil {
		fmt.Fprintln(stdout, "report:", path)
	}
	if !game.AllPass(rep) {
		return 3
	}
	return 0
}

func runPlayerTest(ctx context.Context, ef evalFlags, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "test", []string{"--root", ef.root, "--scenario", ef.scenario}, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	res, err := t.g.Test(ctx, ef.runs, t.say)
	if err != nil {
		return t.fail(err)
	}
	res.Report.WriteTable(stdout)
	if res.Path != "" {
		fmt.Fprintln(stdout, "report:", res.Path)
	}
	if !res.Passed {
		fmt.Fprintln(stdout, "\nAinda não. As dimensões acima dizem o que falta. `sesim hint` se precisar.")
		return 3
	}
	fmt.Fprintln(stdout, "\nPASS em todas as dimensões.")
	fmt.Fprintln(stdout)
	if err := t.printDoc(ctx, t.g.Pack.Manifest.Player.Debrief); err != nil {
		return t.fail(err)
	}
	t.g.Event("debrief_viewed", nil)
	return 0
}

// verifyCase is one row of the reference matrix.
type verifyCase struct {
	Name   string                         `json:"name"`
	Expect scenario.Expect                `json:"expect"`
	Rounds []map[string]evaluator.Verdict `json:"rounds"`
}

// runScenarioVerify: sesim scenario verify — the scenario harness
// (simulation-engine.md §8): the broken start and every reference, K rounds.
func runScenarioVerify(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scenario verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ef evalFlags
	ef.register(fs)
	rounds := fs.Int("rounds", 1, "how many times to evaluate each case (flip rate needs >= 2)")
	only := fs.String("only", "", "comma-separated case names to run (broken, or reference names)")
	base := fs.String("base", "nexora", "scenario start the reference patches apply to")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	pack, log, ok := ef.load(stderr)
	if !ok {
		return 1
	}

	type input struct{ name, patch string }
	inputs := []input{{"broken", ""}}
	expect := map[string]scenario.Expect{"broken": pack.Manifest.Broken}
	for _, r := range pack.Manifest.References {
		inputs = append(inputs, input{r.Name, filepath.Join(pack.Dir, r.Patch)})
		expect[r.Name] = r.Expect
	}
	if *only != "" {
		keep := strings.Split(*only, ",")
		inputs = slices.DeleteFunc(inputs, func(in input) bool { return !slices.Contains(keep, in.name) })
	}

	cases := make([]*verifyCase, len(inputs))
	for i, in := range inputs {
		cases[i] = &verifyCase{Name: in.name, Expect: expect[in.name]}
	}
	// One sandbox for the whole matrix: every case rebuilds the app from its
	// own workspace and every run restores the pristine dataset.
	err := withSandbox(ctx, ef.root, *base, log, func(sb *stack.Stack) error {
		for round := 1; round <= *rounds; round++ {
			for i, in := range inputs {
				log.Info("verify", "round", round, "case", in.name)
				dir, cleanup, err := workspace.Materialize(ctx, *base, in.patch)
				if err != nil {
					return err
				}
				rep, err := evaluator.Evaluate(ctx, sb, pack, dir, evaluator.Options{Log: log, TelemetryLag: 5 * time.Second})
				cleanup()
				if err != nil {
					return fmt.Errorf("%s: %w", in.name, err)
				}
				cases[i].Rounds = append(cases[i].Rounds, rep.Dimensions)
				_, _ = game.SaveReport(ef.root, "eval-"+in.name, rep)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}

	mismatches, flips, cells := writeMatrix(stdout, cases)
	if path, err := game.SaveReport(ef.root, "verify", cases); err == nil {
		fmt.Fprintln(stdout, "report:", path)
	}
	fmt.Fprintf(stdout, "\nunexpected verdicts: %d · unstable cells (verdict changed between rounds): %d of %d", mismatches, flips, cells)
	if *rounds >= 2 && cells > 0 {
		fmt.Fprintf(stdout, " (flip rate %.1f%%)", 100*float64(flips)/float64(cells))
	}
	fmt.Fprintln(stdout)
	if mismatches > 0 || flips > 0 {
		return 3
	}
	return 0
}

// writeMatrix prints one line per case and dimension and counts
// unexpected verdicts and cells whose verdict changed across rounds.
func writeMatrix(w io.Writer, cases []*verifyCase) (mismatches, flips, cells int) {
	fmt.Fprintf(w, "\n%-18s %-13s %-9s %s\n", "case", "dimension", "expected", "verdicts by round")
	for _, c := range cases {
		dims := make([]string, 0, len(c.Expect))
		for d := range c.Expect {
			dims = append(dims, d)
		}
		slices.Sort(dims)
		for _, d := range dims {
			var got []string
			for _, r := range c.Rounds {
				got = append(got, string(r[d]))
			}
			mark := "ok"
			if slices.ContainsFunc(got, func(v string) bool { return v != c.Expect[d] }) {
				mark = "UNEXPECTED"
				mismatches++
			}
			if len(slices.Compact(slices.Sorted(slices.Values(got)))) > 1 {
				flips++
			}
			cells++
			fmt.Fprintf(w, "%-18s %-13s %-9s %-40s %s\n", c.Name, d, c.Expect[d], strings.Join(got, " "), mark)
		}
	}
	return mismatches, flips, cells
}
