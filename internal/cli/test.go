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
	"github.com/Bappoz/software-engineering-simulator/internal/power"
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
	fs.StringVar(&f.root, "root", ".", "raiz do repositório do jogo")
	fs.StringVar(&f.scenario, "scenario", defaultScenario, "diretório do pacote do cenário")
	fs.IntVar(&f.runs, "runs", 0, "número de execuções (0 = o do cenário)")
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
func withSandbox(ctx context.Context, root, workspace string, pack *scenario.Pack, log *slog.Logger, f func(*stack.Stack) error) error {
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
	s, err := stack.Up(ctx, d, reg, stack.Options{Root: root, Workspace: workspace, PaymentLatency: pack.Manifest.World.PaymentLatency}, log)
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
	ptFlags(fs, stderr)
	var ef evalFlags
	ef.register(fs)
	ws := fs.String("workspace", "nexora", "o app a avaliar")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	explicit, scenarioSet := false, false
	fs.Visit(func(f *flag.Flag) {
		explicit = explicit || f.Name == "workspace"
		scenarioSet = scenarioSet || f.Name == "scenario"
	})
	// A workspace with a game in it (the default ./workspace, or the one
	// named) is tested inside that game: it pauses the story's traffic,
	// records the result and completes the scenario on a PASS.
	gameWS := defaultWorkspace
	if explicit {
		gameWS = *ws
	}
	if _, err := play.LoadSession(gameWS); err == nil {
		if !scenarioSet {
			ef.scenario = "" // the game's current scenario
		}
		return runPlayerTest(ctx, ef, gameWS, stdout, stderr)
	}
	pack, log, ok := ef.load(stderr)
	if !ok {
		return 1
	}
	var rep evaluator.Report
	err := withSandbox(ctx, ef.root, *ws, pack, log, func(s *stack.Stack) error {
		var err error
		rep, err = evaluator.Evaluate(ctx, s, pack, *ws, evaluator.Options{Log: log, TelemetryLag: 5 * time.Second, IntegritySettle: 12 * time.Second})
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
	if msg := game.SlowHost(rep); msg != "" {
		fmt.Fprintln(stdout, "\n"+msg)
	}
	if path, err := game.SaveReport(root, "eval", rep); err == nil {
		fmt.Fprintln(stdout, "relatório:", path)
	}
	if !game.AllPass(rep) {
		return 3
	}
	return 0
}

func runPlayerTest(ctx context.Context, ef evalFlags, ws string, stdout, stderr io.Writer) int {
	args := []string{"--root", ef.root, "--workspace", ws}
	if ef.scenario != "" {
		args = append(args, "--scenario", ef.scenario)
	}
	t, code := openGame(ctx, "test", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	defer t.tellStory(ctx) // after the command's own output
	res, err := t.g.Test(ctx, ef.runs, t.say)
	if err != nil {
		return t.fail(err)
	}
	res.Report.WriteTable(stdout)
	if msg := game.SlowHost(res.Report); msg != "" {
		fmt.Fprintln(stdout, "\n"+msg)
	}
	if res.Path != "" {
		fmt.Fprintln(stdout, "relatório:", res.Path)
	}
	if !res.Passed {
		fmt.Fprintln(stdout, "\nAinda não. As dimensões acima dizem o que falta. `sesim hint` se precisar.")
		return 3
	}
	fmt.Fprintln(stdout, "\nPASS em todas as dimensões.")
	fmt.Fprintln(stdout)
	if err := t.printDoc(ctx, t.g.Pack().Manifest.Player.Debrief); err != nil {
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
	ptFlags(fs, stderr)
	var ef evalFlags
	ef.register(fs)
	rounds := fs.Int("rounds", 1, "quantas vezes avaliar cada caso (a taxa de oscilação precisa de 2 ou mais)")
	only := fs.String("only", "", "casos a rodar, separados por vírgula (broken ou nomes de referências)")
	base := fs.String("base", "nexora", "início do cenário sobre o qual as referências são aplicadas")
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

	if why := power.Saving(); why != "" {
		fmt.Fprintf(stderr, "sesim: atenção: %s; os vereditos de velocidade não vão bater com a calibração (ADR-0021)\n", why)
	}
	cases := make([]*verifyCase, len(inputs))
	for i, in := range inputs {
		cases[i] = &verifyCase{Name: in.name, Expect: expect[in.name]}
	}
	// Calibration of the slow-host floor (ADR-0021): how close the passing
	// references come to failing, and how fast this machine probes.
	var (
		tol     evaluator.Tolerance
		tolOK   bool
		tolCase string
		probes  []float64
	)
	// One sandbox for the whole matrix: every case rebuilds the app from its
	// own workspace and every run restores the pristine dataset.
	err := withSandbox(ctx, ef.root, *base, pack, log, func(sb *stack.Stack) error {
		for round := 1; round <= *rounds; round++ {
			for i, in := range inputs {
				log.Info("verify", "round", round, "case", in.name)
				dir, cleanup, err := workspace.Materialize(ctx, *base, append(pack.StartPatches(), in.patch)...)
				if err != nil {
					return err
				}
				rep, err := evaluator.Evaluate(ctx, sb, pack, dir, evaluator.Options{Log: log, TelemetryLag: 5 * time.Second, IntegritySettle: 12 * time.Second})
				cleanup()
				if err != nil {
					return fmt.Errorf("%s: %w", in.name, err)
				}
				cases[i].Rounds = append(cases[i].Rounds, rep.Dimensions)
				if t, ok := evaluator.SlowdownTolerance(pack.Manifest.Objectives, rep); ok && (!tolOK || t.Slowdown < tol.Slowdown) {
					tol, tolOK, tolCase = t, true, in.name
				}
				if rep.Host != nil {
					probes = append(probes, rep.Host.ProbeMS)
				}
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
		fmt.Fprintln(stdout, "relatório:", path)
	}
	fmt.Fprintf(stdout, "\nvereditos inesperados: %d · células instáveis (o veredito mudou entre rodadas): %d de %d", mismatches, flips, cells)
	if *rounds >= 2 && cells > 0 {
		fmt.Fprintf(stdout, " (taxa de oscilação %.1f%%)", 100*float64(flips)/float64(cells))
	}
	fmt.Fprintln(stdout)
	if tolOK && len(probes) > 0 {
		slices.Sort(probes)
		ref := probes[len(probes)/2]
		fmt.Fprintf(stdout, "tolerância a lentidão: %.2fx (%s, %s) · teste do banco mediano %.0f ms → piso %.0f ms (evaluator.DefaultProbeLimit: %v, ADR-0021)\n",
			tol.Slowdown, tolCase, tol.Objective, ref, evaluator.FloorFor(tol, ref), evaluator.DefaultProbeLimit)
	}
	if mismatches > 0 || flips > 0 {
		return 3
	}
	return 0
}

// writeMatrix prints one line per case and dimension and counts
// unexpected verdicts and cells whose verdict changed across rounds.
func writeMatrix(w io.Writer, cases []*verifyCase) (mismatches, flips, cells int) {
	fmt.Fprintf(w, "\n%-18s %-13s %-9s %s\n", "caso", "dimensão", "esperado", "vereditos por rodada")
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
				mark = "INESPERADO"
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
