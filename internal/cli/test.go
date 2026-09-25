package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/devstack"
	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
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

func (f *evalFlags) load(stderr io.Writer) (*scenario.Pack, *devstack.Stack, *slog.Logger, bool) {
	pack, err := scenario.Load(f.scenario)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, nil, nil, false
	}
	if f.runs > 0 {
		pack.Manifest.Test.Runs = f.runs
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	return pack, devstack.New(f.root, log), log, true
}

// runTest: sesim test — Production Test of one workspace.
func runTest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ef evalFlags
	ef.register(fs)
	ws := fs.String("workspace", "nexora", "the app to evaluate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	pack, stack, log, ok := ef.load(stderr)
	if !ok {
		return 1
	}
	defer stack.Close()
	rep, err := evaluator.Evaluate(ctx, stack, pack, *ws, evaluator.Options{Log: log, TelemetryLag: 5 * time.Second})
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	rep.WriteTable(stdout)
	if path, err := saveJSON(ef.root, "eval", rep); err == nil {
		fmt.Fprintln(stdout, "report:", path)
	}
	for _, v := range rep.Dimensions {
		if v != evaluator.Pass {
			return 3
		}
	}
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
	pack, stack, log, ok := ef.load(stderr)
	if !ok {
		return 1
	}
	defer stack.Close()

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
	for round := 1; round <= *rounds; round++ {
		for i, in := range inputs {
			log.Info("verify", "round", round, "case", in.name)
			dir, cleanup, err := workspace.Materialize(ctx, *base, in.patch)
			if err != nil {
				fmt.Fprintln(stderr, "sesim:", err)
				return 1
			}
			rep, err := evaluator.Evaluate(ctx, stack, pack, dir, evaluator.Options{Log: log, TelemetryLag: 5 * time.Second})
			cleanup()
			if err != nil {
				fmt.Fprintf(stderr, "sesim: %s: %v\n", in.name, err)
				return 1
			}
			cases[i].Rounds = append(cases[i].Rounds, rep.Dimensions)
			_, _ = saveJSON(ef.root, "eval-"+in.name, rep)
		}
	}

	mismatches, flips, cells := writeMatrix(stdout, cases)
	if path, err := saveJSON(ef.root, "verify", cases); err == nil {
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

func saveJSON(root, kind string, v any) (string, error) {
	dir := filepath.Join(root, ".runs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.json", time.Now().Format("20060102T150405"), kind))
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o600)
}
