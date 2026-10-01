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
	"github.com/Bappoz/software-engineering-simulator/internal/power"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
)

// Errors a view turns into player-facing text.
var (
	ErrNoSession  = play.ErrNoSession
	ErrNotRunning = errors.New("a loja desta partida não está rodando")
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

	d      *docker.Client
	reg    stack.Registry
	events events.Log
	log    *slog.Logger

	mu       sync.Mutex
	pack     *scenario.Pack // changes when a campaign advances
	stack    *stack.Stack   // nil while the sandbox is down
	session  *play.Session
	saved    bool        // the session exists on disk
	file     os.FileInfo // the session file this process last read or wrote
	lastTest *evaluator.Report
}

// SlowHost explains a report judged on a machine slower than the scenarios
// were calibrated for (ADR-0021); "" when the machine was fast enough.
func SlowHost(rep evaluator.Report) string {
	if rep.Host == nil || !rep.Host.Excused {
		return ""
	}
	return fmt.Sprintf("Esta máquina rodou %.1fx mais devagar do que a máquina em que os cenários foram calibrados (teste de velocidade do banco: %.0f ms; limite: %.0f ms). "+
		"Um resultado perto do limite ficou INCONCLUSIVE; o que reprova mesmo descontando a lentidão continua FAIL. "+
		"Ligue o carregador ou saia do modo de economia de energia e teste de novo.",
		rep.Host.Slowdown, rep.Host.ProbeMS, rep.Host.LimitMS)
}

// DefaultScenario is where a new game starts: the first scenario of Stage 1.
const DefaultScenario = "scenarios/stage1/c1-slow-search"

// Open loads the scenario and the workspace's session (a new one in memory
// when there is none; HasSession tells) and attaches the sandbox when alive.
// An empty o.Scenario means the workspace's current scenario (a campaign
// moves it forward), or DefaultScenario for a new game.
func Open(ctx context.Context, o Options) (*Game, error) {
	ws0, err := filepath.Abs(o.Workspace)
	if err != nil {
		return nil, err
	}
	if prev, err := play.LoadSession(ws0); err == nil && prev.Scenario != "" {
		switch {
		case o.Scenario == "":
			o.Scenario = prev.Scenario
		case !samePath(o.Scenario, prev.Scenario):
			return nil, fmt.Errorf("este workspace está no cenário %s; rode sem --scenario, ou use outro --workspace", prev.Scenario)
		}
	}
	if o.Scenario == "" {
		o.Scenario = filepath.Join(o.Root, DefaultScenario)
	}
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
		Root: o.Root, pack: p, d: d, reg: reg, log: log,
		events: events.Log{Path: play.EventsPath(ws), Scenario: p.Manifest.Metadata.ID},
	}
	g.session, g.file, err = play.LoadSessionInfo(ws)
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
		if err == nil && rec != nil && rec.State == stack.Ready && g.sandboxRunning(ctx, *rec) {
			if g.stack, err = stack.Attach(d, reg, *rec, o.Root, log); err != nil {
				g.Close()
				return nil, err
			}
			if err := g.ensureBackground(ctx); err != nil {
				log.Warn("could not restart the story's traffic", "err", err)
			}
		}
	}
	return g, nil
}

func samePath(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && filepath.Clean(x) == filepath.Clean(y)
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

// sandboxRunning reports whether the sandbox of rec is up. A crashed app
// replica is part of the story (C3), and a stopped story traffic generator
// is started again by the game: neither means the game must start over.
// Anything else stopped (a reboot, a `docker stop`) does.
func (g *Game) sandboxRunning(ctx context.Context, rec stack.Record) bool {
	runs, err := g.d.Runs(ctx)
	if err != nil || runs[rec.Run] == nil {
		return false
	}
	r := runs[rec.Run]
	for _, c := range r.Stopped {
		if c != "app" && c != stack.BackgroundComponent {
			return false
		}
	}
	return r.Running > 0
}

// Pack is the scenario being played.
func (g *Game) Pack() *scenario.Pack {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.follow()
	return g.pack
}

// HasSession reports whether this workspace has a game in progress.
func (g *Game) HasSession() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.follow()
	return g.saved
}

// Session returns a copy of the session.
func (g *Game) Session() play.Session {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.follow()
	s := *g.session
	return s
}

// follow adopts the session another process saved since this one last read
// or wrote it: the UI and a CLI command can hold the same workspace, and
// neither may act on (or save over) a stale copy. When the other process
// advanced the campaign, the scenario follows. Callers hold g.mu.
func (g *Game) follow() {
	if err := g.syncSession(); err != nil {
		g.log.Warn("could not reload the session saved by another process", "err", err)
	}
}

func (g *Game) syncSession() error {
	ws := g.session.Workspace
	fi, err := play.StateInfo(ws)
	if errors.Is(err, os.ErrNotExist) || (err == nil && g.file != nil && os.SameFile(fi, g.file)) {
		return nil
	}
	if err != nil {
		return err
	}
	s, fi, err := play.LoadSessionInfo(ws)
	if err != nil {
		return err
	}
	if !samePath(s.Scenario, g.pack.Dir) {
		p, err := scenario.Load(s.Scenario)
		if err != nil {
			return err
		}
		g.pack = p
		g.events = events.Log{Path: play.EventsPath(ws), Scenario: p.Manifest.Metadata.ID}
	}
	if g.stack != nil && string(g.stack.Run) != s.Sandbox { // the other process brought it down
		g.stack.Close()
		g.stack = nil
	}
	g.session, g.file, g.saved = s, fi, true
	g.lastTest = loadLastTest(ws)
	return nil
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
	if err := g.syncSession(); err != nil {
		return fmt.Errorf("não consegui ler o progresso salvo: %w", err)
	}
	f(g.session)
	fi, err := g.session.SaveInfo()
	if err != nil {
		return fmt.Errorf("não consegui salvar o progresso: %w", err)
	}
	g.file, g.saved = fi, true
	return nil
}

// Event records a product event; failures never break the game.
func (g *Game) Event(name string, props map[string]any) {
	g.mu.Lock()
	l := g.events
	g.mu.Unlock()
	_ = l.Append(name, props)
}

// Phase is the scenario phase the story is in (the growth phase once the
// incident started).
func (g *Game) Phase() scenario.Phase {
	if g.Session().Phase == play.Calm {
		return g.Pack().Manifest.Player.Calm
	}
	return g.Pack().Manifest.Player.Growth
}

// Document renders a player file of the pack (briefing, debrief) with words,
// the view's names for the game's tools, plus the live addresses.
func (g *Game) Document(ctx context.Context, rel string, words map[string]string) (string, error) {
	b, err := os.ReadFile(filepath.Join(g.Pack().Dir, rel)) //nolint:gosec // G304: file named by the scenario pack
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
		base := filepath.Join(g.Root, g.Pack().Manifest.Player.WorkspaceBase)
		tag := "sesim/scenario-start/" + filepath.Base(g.Pack().Dir)
		if err := play.CreateWorkspace(ctx, base, s.Workspace, tag, g.Pack().StartPatches()...); err != nil {
			return false, err
		}
		if err := g.update(func(s *play.Session) { s.Phase, s.StartedAt = play.Calm, time.Now().UTC() }); err != nil {
			return false, err
		}
		say("Workspace criado em " + s.Workspace)
	}
	if g.running() == nil {
		if !fresh && s.Sandbox != "" {
			say("A loja desta partida não está mais rodando; subindo de novo (os dados voltam ao início do cenário, o seu código continua).")
		}
		if gone, err := stack.GC(ctx, g.d, g.reg, false); err == nil && len(gone) > 0 {
			g.log.Info("collected orphaned sandboxes", "runs", gone)
		}
		if old := sandbox.RunID(s.Sandbox); old != "" {
			_ = stack.Down(ctx, g.d, g.reg, old) // a stopped sandbox of this game; start clean
		}
		say("Subindo o Mercado Lerdo (a primeira vez leva alguns minutos)…")
		st, err := stack.Up(ctx, g.d, g.reg, stack.Options{Root: g.Root, Workspace: s.Workspace, PaymentLatency: g.Pack().Manifest.World.PaymentLatency}, g.log)
		if err != nil {
			return fresh, err
		}
		// Session first: a process following it drops a handle the session does not name.
		if err := g.update(func(s *play.Session) { s.Sandbox = string(st.Run) }); err != nil {
			st.Close()
			return fresh, err
		}
		g.mu.Lock()
		g.stack = st
		g.mu.Unlock()
	}
	if err := g.startBackground(ctx); err != nil {
		return fresh, err
	}
	if fresh {
		g.Event("scenario_started", map[string]any{"seed": g.Pack().Manifest.Dataset.Seed})
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
	if err := g.ResumeClock(); err != nil {
		return false, err
	}
	now := time.Now().UTC()
	if err := g.update(func(s *play.Session) { s.Phase, s.IncidentAt = play.Incident, now }); err != nil {
		return false, err
	}
	if err := g.startBackground(ctx); err != nil {
		return true, err
	}
	g.Event("growth_event", map[string]any{"calm_seconds": int(now.Sub(s.StartedAt).Seconds())})
	if c := g.Pack().Manifest.Player.Growth.Crash; c != "" {
		st, err := g.requireStack()
		if err != nil {
			return true, err
		}
		// The customers must be in the store when it goes down: a load
		// generator that cannot log its users in at start gives up.
		g.waitForTraffic(ctx, st, 30*time.Second)
		if err := st.Crash(ctx, c, 1); err != nil {
			return true, err
		}
		if err := g.update(func(s *play.Session) { s.CrashedAt = time.Now().UTC() }); err != nil {
			return true, err
		}
		g.Event("fault_injected", map[string]any{"crash": c})
	}
	return true, nil
}

// waitForTraffic waits until the story's traffic reaches the store, or timeout.
func (g *Game) waitForTraffic(ctx context.Context, st *stack.Stack, timeout time.Duration) {
	q := fmt.Sprintf(`sum(k6_http_reqs_total{testid=%q,name!="setup_login",name!=""})`, g.Session().Background)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v, err := st.Query(ctx, q, time.Now()); err == nil && v > 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// ensureBackground starts the story's traffic again if its generator stopped
// (e.g. it could not log its users in while the app was down).
func (g *Game) ensureBackground(ctx context.Context) error {
	s := g.Session()
	if s.Background == "" || s.Phase == "" {
		return nil
	}
	runs, err := g.d.Runs(ctx)
	if err != nil {
		return err
	}
	if r := runs[sandbox.RunID(s.Sandbox)]; r != nil && slices.Contains(r.Stopped, stack.BackgroundComponent) {
		return g.startBackground(ctx)
	}
	return nil
}

// MarkViewed records that the player looked at a view ("pulse", "support"):
// the story's requests can ask for it.
func (g *Game) MarkViewed(what string) error {
	if !g.HasSession() {
		return nil
	}
	return g.update(func(s *play.Session) {
		if s.Viewed == nil {
			s.Viewed = map[string]time.Time{}
		}
		s.Viewed[what] = time.Now().UTC()
	})
}

// RecordBaseline keeps the first look at production in the calm phase that
// had traffic in it: the "normal" the story's replies can quote. sig is what
// the player saw; nil measures it now.
func (g *Game) RecordBaseline(ctx context.Context, sig *play.Signals) error {
	if !g.HasSession() {
		return nil
	}
	if s := g.Session(); s.Phase != play.Calm || s.Baseline != nil {
		return nil
	}
	if sig == nil {
		got, err := g.Signals(ctx, 10*time.Second) // the window of `sesim status`
		if err != nil {
			return err
		}
		sig = &got
	}
	if len(sig.Endpoints) == 0 {
		return nil
	}
	b := &play.Baseline{At: time.Now().UTC(), Signals: *sig}
	return g.update(func(s *play.Session) {
		if s.Phase == play.Calm && s.Baseline == nil {
			s.Baseline = b
		}
	})
}

// Hint reveals the next rung of the hint ladder; false when none is left.
func (g *Game) Hint() (scenario.Hint, bool, error) {
	s := g.Session()
	h, ok := play.NextHint(g.Pack().Hints, s.HintsShown)
	if !ok {
		return h, false, nil
	}
	price := g.hintPrice(s.HintsShown + 1)
	item := fmt.Sprintf("Dica %d · %s", h.Level, g.Pack().Manifest.Metadata.Title) // outside update: it holds g.mu
	if err := g.update(func(s *play.Session) {
		s.HintsShown++
		buy(s, item, price)
	}); err != nil {
		return h, true, err
	}
	props := map[string]any{"level": h.Level, "price": price}
	if !s.IncidentAt.IsZero() {
		props["incident_minutes"] = int(time.Since(s.IncidentAt).Minutes())
	}
	g.Event("hint_requested", props)
	return h, true, nil
}

// Hints returns the hints revealed so far.
func (g *Game) Hints() []scenario.Hint {
	return g.Pack().Hints[:min(g.Session().HintsShown, len(g.Pack().Hints))]
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
	if err := g.ResumeClock(); err != nil {
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
	if err := g.update(func(s *play.Session) { s.DeployedAt = time.Now().UTC() }); err != nil {
		return changes, err
	}
	if err := g.ensureBackground(ctx); err != nil {
		say("Não consegui religar o tráfego do dia: " + err.Error())
	}
	return changes, nil
}

// Check is the Quick Check: deploy the workspace to the live sandbox and
// measure the story's traffic (indicative; no data reset).
func (g *Game) Check(ctx context.Context, say Say) (CheckResult, error) {
	st, err := g.requireStack()
	if err != nil {
		return CheckResult{}, err
	}
	if err := g.ResumeClock(); err != nil {
		return CheckResult{}, err
	}
	start := time.Now()
	if _, err := st.Deploy(ctx, say); err != nil {
		g.Event("quick_check_failed", map[string]any{"stage": "deploy"})
		return CheckResult{}, err
	}
	if err := g.ensureBackground(ctx); err != nil {
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
	if err := g.ResumeClock(); err != nil {
		return TestResult{}, err
	}
	pack := *g.Pack()
	if runs > 0 {
		m := pack.Manifest
		m.Test.Runs = runs
		pack.Manifest = m
	}
	m := pack.Manifest.Test
	say(fmt.Sprintf("Production Test: %d execuções de %s até %d req/s, cada uma a partir do banco original + as suas migrações.", m.Runs, m.Profile, m.Rate))
	say("O tráfego do dia fica pausado enquanto isso. Leva alguns minutos.")
	if why := power.Saving(); why != "" {
		say("Atenção: " + why + ". A loja roda mais devagar assim, e o que depende de velocidade pode ficar inconclusivo. " +
			"Para um resultado que vale, ligue o carregador ou use um perfil de energia equilibrado.")
	}
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
	run := map[string]any{"seconds": int(res.Elapsed.Seconds()), "dimensions": dims}
	if h := rep.Host; h != nil {
		run["db_probe_ms"], run["slow_host"] = int(h.ProbeMS), h.Slow
	}
	g.Event("production_test_run", run)
	res.Path, _ = SaveReport(g.Root, "eval", rep)
	g.mu.Lock()
	g.lastTest = &rep
	g.mu.Unlock()
	saveLastTest(s.Workspace, rep)
	if !res.Passed {
		if err := g.update(func(s *play.Session) { s.LastFailAt = time.Now().UTC() }); err != nil {
			return res, err
		}
	}
	if !res.Passed || s.Phase == play.Completed {
		return res, nil
	}
	done := time.Now().UTC()
	cost, budget := costAtPass(g.Pack(), rep)
	if err := g.update(func(s *play.Session) { s.Phase, s.DoneAt, s.PassCost, s.Budget = play.Completed, done, cost, budget }); err != nil {
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
