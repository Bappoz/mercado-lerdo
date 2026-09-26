// Package game is one player's game: the scenario, the workspace session and
// the sandbox it runs on. The CLI and the web UI are two views of it; neither
// holds game logic.
package game

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
	"github.com/Bappoz/software-engineering-simulator/internal/events"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
)

// Errors a view turns into player-facing text.
var (
	ErrNoSession  = play.ErrNoSession
	ErrNotRunning = errors.New("a Nexora desta partida não está rodando")
)

// Options locate a game.
type Options struct {
	Root      string // repository root
	Scenario  string // scenario pack directory
	Workspace string // the player's copy of the app
	// Log receives sandbox and evaluator progress at Info; nil discards it.
	Log *slog.Logger
}

// Say narrates a long operation to the player, one line at a time.
type Say func(string)

// Game is safe for concurrent use: the session and the sandbox handle are
// guarded, and the long parts of an operation run without the lock. Views
// must still run one long operation (Start, Check, Test) at a time.
type Game struct {
	Root string
	Pack *scenario.Pack

	d      *docker.Client
	reg    stack.Registry
	events events.Log
	log    *slog.Logger

	mu       sync.Mutex
	stack    *stack.Stack // nil while the sandbox is down
	session  *play.Session
	saved    bool // the session exists on disk
	lastTest *evaluator.Report
}

// Open loads the scenario and the workspace's session (a new one in memory
// when there is none; HasSession tells) and attaches the sandbox when alive.
func Open(ctx context.Context, o Options) (*Game, error) {
	p, err := scenario.Load(o.Scenario)
	if err != nil {
		return nil, err
	}
	if p.Manifest.Player == nil {
		return nil, fmt.Errorf("o cenário %s não é jogável (sem seção player)", p.Manifest.Metadata.ID)
	}
	ws, err := filepath.Abs(o.Workspace)
	if err != nil {
		return nil, err
	}
	d, err := docker.New()
	if err != nil {
		return nil, err
	}
	reg, err := stack.DefaultRegistry()
	if err != nil {
		_ = d.Close()
		return nil, err
	}
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	g := &Game{
		Root: o.Root, Pack: p, d: d, reg: reg, log: log,
		events: events.Log{Path: play.EventsPath(ws), Scenario: p.Manifest.Metadata.ID},
	}
	g.session, err = play.LoadSession(ws)
	switch {
	case errors.Is(err, play.ErrNoSession):
		g.session = &play.Session{Scenario: o.Scenario, Workspace: ws}
	case err != nil:
		g.Close()
		return nil, err
	default:
		g.saved = true
		g.lastTest = loadLastTest(ws)
	}
	if g.session.Sandbox != "" {
		rec, err := reg.Get(sandbox.RunID(g.session.Sandbox))
		if err == nil && rec != nil && rec.State == stack.Ready && g.sandboxRunning(ctx, rec.Run) {
			if g.stack, err = stack.Attach(d, reg, *rec, o.Root, log); err != nil {
				g.Close()
				return nil, err
			}
		}
	}
	return g, nil
}

// Close releases the Docker client and stops the stats pusher. The sandbox
// keeps running.
func (g *Game) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stack != nil {
		g.stack.Close()
	}
	_ = g.d.Close()
}

// sandboxRunning reports whether every container of run is up (a reboot or
// `docker stop` leaves them stopped, and the game then starts over).
func (g *Game) sandboxRunning(ctx context.Context, run sandbox.RunID) bool {
	runs, err := g.d.Runs(ctx)
	if err != nil || runs[run] == nil {
		return false
	}
	return runs[run].Containers > 0 && runs[run].Running == runs[run].Containers
}

// HasSession reports whether this workspace has a game in progress.
func (g *Game) HasSession() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.saved
}

// Session returns a copy of the session.
func (g *Game) Session() play.Session {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := *g.session
	return s
}

// Running reports whether the game's sandbox is up.
func (g *Game) Running() bool { return g.running() != nil }

// LastTest is the last Production Test of this workspace, if any.
func (g *Game) LastTest() *evaluator.Report {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastTest
}

func (g *Game) running() *stack.Stack {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stack
}

func (g *Game) requireStack() (*stack.Stack, error) {
	if st := g.running(); st != nil {
		return st, nil
	}
	return nil, ErrNotRunning
}

// update changes the session under the lock and saves it.
func (g *Game) update(f func(s *play.Session)) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	f(g.session)
	if err := g.session.Save(); err != nil {
		return fmt.Errorf("não consegui salvar o progresso: %w", err)
	}
	g.saved = true
	return nil
}

// Event records a product event; failures never break the game.
func (g *Game) Event(name string, props map[string]any) { _ = g.events.Append(name, props) }

// Phase is the scenario phase the story is in (the growth phase once the
// incident started).
func (g *Game) Phase() scenario.Phase {
	if g.Session().Phase == play.Calm {
		return g.Pack.Manifest.Player.Calm
	}
	return g.Pack.Manifest.Player.Growth
}

// Document renders a player file of the pack (briefing, debrief) with words,
// the view's names for the game's tools, plus the live addresses.
func (g *Game) Document(ctx context.Context, rel string, words map[string]string) (string, error) {
	b, err := os.ReadFile(filepath.Join(g.Pack.Dir, rel)) //nolint:gosec // G304: file named by the scenario pack
	if err != nil {
		return "", err
	}
	vars := map[string]string{"workspace": g.Session().Workspace}
	for k, v := range words {
		vars[k] = v
	}
	if st := g.running(); st != nil {
		vars["app_url"], _ = st.NginxURL(ctx)
		vars["prometheus_url"], _ = st.PrometheusURL(ctx)
	}
	return play.RenderBriefing(string(b), vars), nil
}

// Start starts a new game or resumes this one: workspace, sandbox, traffic.
// fresh is true for a new game.
func (g *Game) Start(ctx context.Context, say Say) (fresh bool, err error) {
	s := g.Session()
	fresh = s.Phase == ""
	if fresh {
		base := filepath.Join(g.Root, g.Pack.Manifest.Player.WorkspaceBase)
		tag := "sesim/scenario-start/" + filepath.Base(g.Pack.Dir)
		if err := play.CreateWorkspace(ctx, base, s.Workspace, tag, g.Pack.StartPatches()...); err != nil {
			return false, err
		}
		if err := g.update(func(s *play.Session) { s.Phase, s.StartedAt = play.Calm, time.Now().UTC() }); err != nil {
			return false, err
		}
		say("Workspace criado em " + s.Workspace)
	}
	if g.running() == nil {
		if !fresh && s.Sandbox != "" {
			say("A Nexora desta partida não está mais rodando; subindo de novo (os dados voltam ao início do cenário, o seu código continua).")
		}
		if gone, err := stack.GC(ctx, g.d, g.reg, false); err == nil && len(gone) > 0 {
			g.log.Info("collected orphaned sandboxes", "runs", gone)
		}
		if old := sandbox.RunID(s.Sandbox); old != "" {
			_ = stack.Down(ctx, g.d, g.reg, old) // a stopped sandbox of this game; start clean
		}
		say("Subindo a Nexora (a primeira vez leva alguns minutos)…")
		st, err := stack.Up(ctx, g.d, g.reg, stack.Options{Root: g.Root, Workspace: s.Workspace, PaymentLatency: g.Pack.Manifest.World.PaymentLatency}, g.log)
		if err != nil {
			return fresh, err
		}
		g.mu.Lock()
		g.stack = st
		g.mu.Unlock()
		if err := g.update(func(s *play.Session) { s.Sandbox = string(st.Run) }); err != nil {
			return fresh, err
		}
	}
	if err := g.startBackground(ctx); err != nil {
		return fresh, err
	}
	if fresh {
		g.Event("scenario_started", map[string]any{"seed": g.Pack.Manifest.Dataset.Seed})
	} else {
		g.Event("scenario_resumed", map[string]any{"phase": string(g.Session().Phase)})
	}
	return fresh, nil
}

func (g *Game) startBackground(ctx context.Context) error {
	st, err := g.requireStack()
	if err != nil {
		return err
	}
	ph := g.Phase()
	id := fmt.Sprintf("%s%s-%d", backgroundPrefix, g.Session().Phase, time.Now().Unix())
	if err := st.StartBackground(ctx, ph.Profile, ph.Rate, id); err != nil {
		return err
	}
	return g.update(func(s *play.Session) { s.Background = id })
}

// backgroundPrefix starts the testid of the story's traffic (never of a
// Production Test run).
const backgroundPrefix = "bg-"

// Next ends the calm phase: the growth event starts the incident. It returns
// false when the incident had already started.
func (g *Game) Next(ctx context.Context) (bool, error) {
	if _, err := g.requireStack(); err != nil {
		return false, err
	}
	s := g.Session()
	if s.Phase != play.Calm {
		return false, nil
	}
	now := time.Now().UTC()
	if err := g.update(func(s *play.Session) { s.Phase, s.IncidentAt = play.Incident, now }); err != nil {
		return false, err
	}
	if err := g.startBackground(ctx); err != nil {
		return true, err
	}
	g.Event("growth_event", map[string]any{"calm_seconds": int(now.Sub(s.StartedAt).Seconds())})
	return true, nil
}

// Hint reveals the next rung of the hint ladder; false when none is left.
func (g *Game) Hint() (scenario.Hint, bool, error) {
	s := g.Session()
	h, ok := play.NextHint(g.Pack.Hints, s.HintsShown)
	if !ok {
		return h, false, nil
	}
	if err := g.update(func(s *play.Session) { s.HintsShown++ }); err != nil {
		return h, true, err
	}
	props := map[string]any{"level": h.Level}
	if !s.IncidentAt.IsZero() {
		props["incident_minutes"] = int(time.Since(s.IncidentAt).Minutes())
	}
	g.Event("hint_requested", props)
	return h, true, nil
}

// Hints returns the hints revealed so far.
func (g *Game) Hints() []scenario.Hint {
	return g.Pack.Hints[:min(g.Session().HintsShown, len(g.Pack.Hints))]
}

// CheckResult is one Quick Check.
type CheckResult struct {
	Now     play.Signals  `json:"now"`
	Before  *play.Signals `json:"before,omitempty"` // the previous check
	Elapsed time.Duration `json:"elapsed"`
}

// Deploy ships the workspace (code, migrations, topology, nginx config) to
// the live sandbox, keeping its data.
func (g *Game) Deploy(ctx context.Context, say Say) ([]stack.Change, error) {
	st, err := g.requireStack()
	if err != nil {
		return nil, err
	}
	start := time.Now()
	changes, err := st.Deploy(ctx, say)
	if err != nil {
		g.Event("deploy_failed", nil)
		return nil, err
	}
	comps := make([]string, 0, len(changes))
	for _, c := range changes {
		comps = append(comps, c.Component+":"+string(c.Action))
	}
	g.Event("deploy_run", map[string]any{"ms": time.Since(start).Milliseconds(), "changes": comps})
	return changes, nil
}

// Check is the Quick Check: deploy the workspace to the live sandbox and
// measure the story's traffic (indicative; no data reset).
func (g *Game) Check(ctx context.Context, say Say) (CheckResult, error) {
	st, err := g.requireStack()
	if err != nil {
		return CheckResult{}, err
	}
	start := time.Now()
	if _, err := st.Deploy(ctx, say); err != nil {
		g.Event("quick_check_failed", map[string]any{"stage": "deploy"})
		return CheckResult{}, err
	}
	applied := time.Now()
	say("Aquecendo 15 s e medindo 30 s do tráfego real…")
	select {
	case <-ctx.Done():
		return CheckResult{}, ctx.Err()
	case <-time.After(15 * time.Second):
	}
	sig, err := g.Signals(ctx, 30*time.Second)
	if err != nil {
		return CheckResult{}, err
	}
	res := CheckResult{Now: sig, Before: g.Session().LastCheck, Elapsed: time.Since(start)}
	if err := g.update(func(s *play.Session) { s.LastCheck = &sig; s.Checks++ }); err != nil {
		return res, err
	}
	g.Event("quick_check_run", map[string]any{
		"deploy_ms": applied.Sub(start).Milliseconds(), "total_ms": res.Elapsed.Milliseconds(),
	})
	return res, nil
}

// TestResult is one Production Test inside the game.
type TestResult struct {
	Report  evaluator.Report `json:"report"`
	Passed  bool             `json:"passed"`
	Path    string           `json:"-"` // the saved report
	Elapsed time.Duration    `json:"elapsed"`
}

// Test runs the graded Production Test (runs > 0 overrides the scenario's
// count). The story's traffic pauses meanwhile. A PASS completes the scenario.
func (g *Game) Test(ctx context.Context, runs int, say Say) (TestResult, error) {
	st, err := g.requireStack()
	if err != nil {
		return TestResult{}, err
	}
	pack := *g.Pack
	if runs > 0 {
		m := pack.Manifest
		m.Test.Runs = runs
		pack.Manifest = m
	}
	m := pack.Manifest.Test
	say(fmt.Sprintf("Production Test: %d execuções de %s até %d req/s, cada uma a partir do banco original + as suas migrações.", m.Runs, m.Profile, m.Rate))
	say("O tráfego do dia fica pausado enquanto isso. Leva alguns minutos.")
	if err := st.StopBackground(ctx); err != nil {
		return TestResult{}, err
	}
	start := time.Now()
	s := g.Session()
	rep, err := evaluator.Evaluate(ctx, st, &pack, s.Workspace, evaluator.Options{Log: g.log, TelemetryLag: 5 * time.Second, IntegritySettle: 12 * time.Second})
	if bgErr := g.startBackground(context.WithoutCancel(ctx)); bgErr != nil {
		say("Não consegui retomar o tráfego: " + bgErr.Error())
	}
	if uerr := g.update(func(s *play.Session) { s.Tests++ }); uerr != nil && err == nil {
		err = uerr
	}
	if err != nil {
		g.Event("production_test_failed_to_run", nil)
		return TestResult{}, err
	}
	res := TestResult{Report: rep, Passed: AllPass(rep), Elapsed: time.Since(start)}
	dims := map[string]string{}
	for d, v := range rep.Dimensions {
		dims[d] = string(v)
	}
	g.Event("production_test_run", map[string]any{"seconds": int(res.Elapsed.Seconds()), "dimensions": dims})
	res.Path, _ = SaveReport(g.Root, "eval", rep)
	g.mu.Lock()
	g.lastTest = &rep
	g.mu.Unlock()
	saveLastTest(s.Workspace, rep)
	if !res.Passed || s.Phase == play.Completed {
		return res, nil
	}
	done := time.Now().UTC()
	if err := g.update(func(s *play.Session) { s.Phase, s.DoneAt = play.Completed, done }); err != nil {
		return res, err
	}
	s = g.Session()
	props := map[string]any{"tests": s.Tests, "checks": s.Checks, "hints": s.HintsShown,
		"total_minutes": int(done.Sub(s.StartedAt).Minutes())}
	if !s.IncidentAt.IsZero() {
		props["incident_minutes"] = int(done.Sub(s.IncidentAt).Minutes())
	}
	g.Event("scenario_completed", props)
	return res, nil
}

// AllPass reports whether every dimension passed.
func AllPass(rep evaluator.Report) bool {
	for _, v := range rep.Dimensions {
		if v != evaluator.Pass {
			return false
		}
	}
	return len(rep.Dimensions) > 0
}

// Stop stops the story's traffic; with down it also destroys the sandbox
// (the workspace and the progress stay).
func (g *Game) Stop(ctx context.Context, down bool) error {
	st, err := g.requireStack()
	if err != nil {
		return err
	}
	if err := st.StopBackground(ctx); err != nil {
		return err
	}
	g.Event("session_paused", map[string]any{"phase": string(g.Session().Phase), "down": down})
	if !down {
		return nil
	}
	st.Close()
	if err := stack.Down(ctx, g.d, g.reg, st.Run); err != nil {
		return err
	}
	g.mu.Lock()
	g.stack = nil
	g.mu.Unlock()
	return g.update(func(s *play.Session) { s.Sandbox = "" })
}

// Shell attaches the player's own terminal to a shell inside the Nexora network.
func (g *Game) Shell(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	st, err := g.requireStack()
	if err != nil {
		return err
	}
	return st.Shell(ctx, stdin, stdout, stderr)
}

// Terminal opens an interactive shell inside the Nexora network.
func (g *Game) Terminal(ctx context.Context, cols, rows uint) (*docker.TTY, error) {
	st, err := g.requireStack()
	if err != nil {
		return nil, err
	}
	return st.Terminal(ctx, cols, rows)
}

// WatchStats pushes container stats to the sandbox's Prometheus until ctx
// ends or the game closes (the live dashboard reads them).
func (g *Game) WatchStats(ctx context.Context) error {
	st, err := g.requireStack()
	if err != nil {
		return err
	}
	return st.StartStats(ctx)
}

// isPlayerComponent reports whether a container is part of the player's
// system. The rest (load generator, telemetry, the emulated payment gateway) is
// the world around it and stays out of the player's view.
func isPlayerComponent(c string) bool {
	switch c {
	case "nginx", "app", "postgres", "pgbouncer":
		return true
	}
	return false
}

// Signals reads what the clients experienced and each container's CPU over window.
func (g *Game) Signals(ctx context.Context, window time.Duration) (play.Signals, error) {
	st, err := g.requireStack()
	if err != nil {
		return play.Signals{}, err
	}
	cpu, err := st.CPU(ctx, window)
	if err != nil {
		return play.Signals{}, err
	}
	at := time.Now()
	sel := fmt.Sprintf(`testid=%q,name!="setup_login",name!=""`, g.Session().Background)
	w := fmt.Sprintf("%ds", int(window.Seconds()))
	byName := map[string]*play.Endpoint{}
	for kind, q := range signalQueries(sel, w) {
		vec, err := st.QueryVector(ctx, q, at)
		if err != nil {
			return play.Signals{}, err
		}
		for _, s := range vec {
			n := s.Labels["name"]
			e, ok := byName[n]
			if !ok {
				e = &play.Endpoint{Name: n}
				byName[n] = e
			}
			switch kind {
			case "rps":
				e.RPS = s.Value
			case "err":
				e.ErrorRate = s.Value
			case "p95":
				e.P95s = s.Value
			}
		}
	}
	out := play.Signals{WindowS: int(window.Seconds()), CPU: map[string]float64{}}
	for c, v := range cpu {
		if isPlayerComponent(c) {
			out.CPU[c] = v
		}
	}
	for _, e := range byName {
		out.Endpoints = append(out.Endpoints, e.Finite())
	}
	slices.SortFunc(out.Endpoints, func(a, b play.Endpoint) int { return cmp.Compare(b.RPS, a.RPS) })
	return out, nil
}

// signalQueries are the client-side signals of the traffic selected by sel,
// per endpoint, over the range w.
func signalQueries(sel, w string) map[string]string {
	return map[string]string{
		"rps": fmt.Sprintf(`sum by (name) (rate(k6_http_reqs_total{%s}[%s]))`, sel, w),
		// k6 splits series by HTTP status, so a ratio of counters is the only
		// correct error rate (averaging the per-series failed_rate gauges is not).
		"err": fmt.Sprintf(`sum by (name) (rate(k6_http_reqs_total{%s,expected_response="false"}[%s])) / sum by (name) (rate(k6_http_reqs_total{%s}[%s]))`, sel, w, sel, w),
		// Native histogram (K6_FEATURES=native-histograms): a true p95 over the
		// window, across every status.
		"p95": fmt.Sprintf(`histogram_quantile(0.95, sum by (name) (rate(k6_http_req_duration_seconds{%s}[%s])))`, sel, w),
	}
}

// SaveReport writes v as JSON into <root>/.runs and returns the path.
func SaveReport(root, kind string, v any) (string, error) {
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

func lastTestPath(ws string) string { return filepath.Join(play.StateDir(ws), "last-test.json") }

func saveLastTest(ws string, rep evaluator.Report) {
	if b, err := json.Marshal(rep); err == nil {
		_ = os.WriteFile(lastTestPath(ws), b, 0o600)
	}
}

func loadLastTest(ws string) *evaluator.Report {
	b, err := os.ReadFile(lastTestPath(ws)) //nolint:gosec // G304: the player's own workspace
	if err != nil {
		return nil
	}
	var rep evaluator.Report
	if json.Unmarshal(b, &rep) != nil {
		return nil
	}
	return &rep
}
