package cli

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/events"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
)

const defaultWorkspace = "workspace"

// game bundles what every player command needs.
type game struct {
	root    string
	pack    *scenario.Pack
	d       *docker.Client
	reg     stack.Registry
	stack   *stack.Stack // nil until the session's sandbox is attached
	session *play.Session
	events  events.Log
	log     *slog.Logger
	stdout  io.Writer
	stderr  io.Writer
}

// openGame parses the common flags and loads the session. With needSession,
// a missing session is an error. The sandbox is attached when it is alive.
func openGame(ctx context.Context, name string, args []string, stdout, stderr io.Writer, needSession bool) (*game, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	pack := fs.String("scenario", defaultScenario, "scenario pack directory")
	ws := fs.String("workspace", defaultWorkspace, "your workspace (a copy of the app you edit)")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	p, err := scenario.Load(*pack)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, 1
	}
	if p.Manifest.Player == nil {
		fmt.Fprintf(stderr, "sesim: o cenário %s não é jogável (sem seção player)\n", p.Manifest.Metadata.ID)
		return nil, 1
	}
	abs, err := filepath.Abs(*ws)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, 1
	}
	d, err := docker.New()
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, 1
	}
	reg, err := stack.DefaultRegistry()
	if err != nil {
		_ = d.Close()
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, 1
	}
	g := &game{
		root: *root, pack: p, d: d, reg: reg, stdout: stdout, stderr: stderr,
		log:    slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		events: events.Log{Path: play.EventsPath(abs), Scenario: p.Manifest.Metadata.ID},
	}
	g.session, err = play.LoadSession(abs)
	switch {
	case errors.Is(err, play.ErrNoSession) && needSession:
		g.close()
		fmt.Fprintln(stderr, "sesim: nenhuma partida neste workspace. Comece com: sesim play")
		return nil, 1
	case errors.Is(err, play.ErrNoSession):
		g.session = &play.Session{Scenario: *pack, Workspace: abs}
	case err != nil:
		g.close()
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, 1
	}
	if g.session.Sandbox != "" {
		if rec, err := reg.Get(sandbox.RunID(g.session.Sandbox)); err == nil && rec != nil && rec.State == stack.Ready && g.sandboxRunning(ctx, rec.Run) {
			g.stack, err = stack.Attach(d, *rec, *root, g.log)
			if err != nil {
				g.close()
				fmt.Fprintln(stderr, "sesim:", err)
				return nil, 1
			}
		}
	}
	return g, 0
}

// sandboxRunning reports whether every container of run is up (a reboot or
// `docker stop` leaves them stopped, and the game then starts over).
func (g *game) sandboxRunning(ctx context.Context, run sandbox.RunID) bool {
	runs, err := g.d.Runs(ctx)
	if err != nil || runs[run] == nil {
		return false
	}
	return runs[run].Containers > 0 && runs[run].Running == runs[run].Containers
}

// requireSandbox is for commands that need the game's sandbox.
func (g *game) requireSandbox() error {
	if g.stack == nil {
		return errors.New("a Nexora desta partida não está rodando. Rode `sesim play` para subir de novo")
	}
	return nil
}

func (g *game) close() {
	if g.stack != nil {
		g.stack.Close()
	}
	_ = g.d.Close()
}

func (g *game) fail(err error) int {
	fmt.Fprintln(g.stderr, "sesim:", err)
	return 1
}

func (g *game) save() {
	if err := g.session.Save(); err != nil {
		fmt.Fprintln(g.stderr, "sesim: não consegui salvar o progresso:", err)
	}
}

func (g *game) event(name string, props map[string]any) { _ = g.events.Append(name, props) }

func (g *game) phase() scenario.Phase {
	if g.session.Phase == play.Calm {
		return g.pack.Manifest.Player.Calm
	}
	return g.pack.Manifest.Player.Growth
}

func (g *game) startBackground(ctx context.Context) error {
	ph := g.phase()
	id := fmt.Sprintf("bg-%s-%d", g.session.Phase, time.Now().Unix())
	if err := g.stack.StartBackground(ctx, ph.Profile, ph.Rate, id); err != nil {
		return err
	}
	g.session.Background = id
	return nil
}

func (g *game) printFile(ctx context.Context, rel string) error {
	b, err := os.ReadFile(filepath.Join(g.pack.Dir, rel)) //nolint:gosec // G304: file named by the scenario pack
	if err != nil {
		return err
	}
	vars := map[string]string{"workspace": g.session.Workspace}
	if g.stack != nil {
		vars["app_url"], _ = g.stack.NginxURL(ctx)
		vars["prometheus_url"], _ = g.stack.PrometheusURL(ctx)
	}
	fmt.Fprintln(g.stdout, play.RenderBriefing(string(b), vars))
	return nil
}

// runPlay: start a new game, or resume the one in the workspace.
func runPlay(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	g, code := openGame(ctx, "play", args, stdout, stderr, false)
	if g == nil {
		return code
	}
	defer g.close()
	s := g.session
	fresh := s.Phase == ""
	if fresh {
		base := filepath.Join(g.root, g.pack.Manifest.Player.WorkspaceBase)
		tag := "sesim/scenario-start/" + filepath.Base(g.pack.Dir)
		if err := play.CreateWorkspace(ctx, base, s.Workspace, tag); err != nil {
			return g.fail(err)
		}
		s.Phase, s.StartedAt = play.Calm, time.Now().UTC()
		fmt.Fprintf(stdout, "Workspace criado em %s\n", s.Workspace)
	}
	if g.stack == nil {
		if !fresh && s.Sandbox != "" {
			fmt.Fprintln(stdout, "A Nexora desta partida não está mais rodando; subindo de novo (os dados voltam ao início do cenário, o seu código continua).")
		}
		if gone, err := stack.GC(ctx, g.d, g.reg, false); err == nil && len(gone) > 0 {
			g.log.Info("collected orphaned sandboxes", "runs", gone)
		}
		if old := sandbox.RunID(s.Sandbox); old != "" {
			_ = stack.Down(ctx, g.d, g.reg, old) // a stopped sandbox of this game; start clean
		}
		fmt.Fprintln(stdout, "Subindo a Nexora (a primeira vez leva alguns minutos)…")
		st, err := stack.Up(ctx, g.d, g.reg, stack.Options{Root: g.root, Workspace: s.Workspace, PGCPUs: 0.5}, g.log)
		if err != nil {
			return g.fail(err)
		}
		g.stack, s.Sandbox = st, string(st.Run)
	}
	if s.Phase == play.Completed {
		fmt.Fprintln(stdout, "Este cenário já foi concluído. O debrief está em:", filepath.Join(g.pack.Dir, g.pack.Manifest.Player.Debrief))
	}
	if err := g.startBackground(ctx); err != nil {
		return g.fail(err)
	}
	g.save()
	if fresh {
		g.event("scenario_started", map[string]any{"seed": g.pack.Manifest.Dataset.Seed})
	} else {
		g.event("scenario_resumed", map[string]any{"phase": string(s.Phase)})
	}
	fmt.Fprintln(stdout)
	if err := g.printFile(ctx, g.phase().Briefing); err != nil {
		return g.fail(err)
	}
	return 0
}

// runNext: end the calm phase; the growth event starts the incident.
func runNext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	g, code := openGame(ctx, "next", args, stdout, stderr, true)
	if g == nil {
		return code
	}
	defer g.close()
	if err := g.requireSandbox(); err != nil {
		return g.fail(err)
	}
	if g.session.Phase != play.Calm {
		fmt.Fprintln(stdout, "Nada a avançar: o incidente já começou. `sesim status` mostra como está a produção.")
		return 0
	}
	g.session.Phase, g.session.IncidentAt = play.Incident, time.Now().UTC()
	if err := g.startBackground(ctx); err != nil {
		return g.fail(err)
	}
	g.save()
	g.event("growth_event", map[string]any{"calm_seconds": int(g.session.IncidentAt.Sub(g.session.StartedAt).Seconds())})
	if err := g.printFile(ctx, g.phase().Briefing); err != nil {
		return g.fail(err)
	}
	return 0
}

// isPlayerComponent reports whether a container is part of the player's
// system. The rest (load generator, telemetry, the emulated payment gateway) is
// the world around it and stays out of the player's view.
func isPlayerComponent(c string) bool {
	switch c {
	case "nginx", "app", "postgres":
		return true
	}
	return false
}

// signals reads what the clients experienced and each container's CPU over window.
func (g *game) signals(ctx context.Context, window time.Duration) (play.Signals, error) {
	cpu, err := g.stack.CPU(ctx, window)
	if err != nil {
		return play.Signals{}, err
	}
	at := time.Now()
	sel := fmt.Sprintf(`testid=%q,name!="setup_login",name!=""`, g.session.Background)
	w := fmt.Sprintf("%ds", int(window.Seconds()))
	queries := map[string]string{
		"rps": fmt.Sprintf(`sum by (name) (rate(k6_http_reqs_total{%s}[%s]))`, sel, w),
		// k6 splits series by HTTP status, so a ratio of counters is the only
		// correct error rate (averaging the per-series failed_rate gauges is not).
		"err": fmt.Sprintf(`sum by (name) (rate(k6_http_reqs_total{%s,expected_response="false"}[%s])) / sum by (name) (rate(k6_http_reqs_total{%s}[%s]))`, sel, w, sel, w),
		// Native histogram (K6_FEATURES=native-histograms): a true p95 over the
		// window, across every status.
		"p95": fmt.Sprintf(`histogram_quantile(0.95, sum by (name) (rate(k6_http_req_duration_seconds{%s}[%s])))`, sel, w),
	}
	byName := map[string]*play.Endpoint{}
	for kind, q := range queries {
		vec, err := g.stack.QueryVector(ctx, q, at)
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
		out.Endpoints = append(out.Endpoints, *e)
	}
	slices.SortFunc(out.Endpoints, func(a, b play.Endpoint) int { return cmp.Compare(b.RPS, a.RPS) })
	return out, nil
}

// runStatus: production signals right now.
func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	g, code := openGame(ctx, "status", args, stdout, stderr, true)
	if g == nil {
		return code
	}
	defer g.close()
	if err := g.requireSandbox(); err != nil {
		return g.fail(err)
	}
	fmt.Fprintln(stdout, "Medindo por 10 s…")
	sig, err := g.signals(ctx, 10*time.Second)
	if err != nil {
		return g.fail(err)
	}
	if len(sig.Endpoints) == 0 {
		fmt.Fprintln(stdout, "Ainda sem dados de tráfego (ele começa alguns segundos depois do `sesim play`/`sesim next`). Tente de novo.")
	}
	sig.Write(stdout, nil)
	g.event("status_viewed", map[string]any{"phase": string(g.session.Phase)})
	return 0
}

// runCheck: Quick Check — apply the workspace and measure the live traffic (indicative).
func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	g, code := openGame(ctx, "check", args, stdout, stderr, true)
	if g == nil {
		return code
	}
	defer g.close()
	if err := g.requireSandbox(); err != nil {
		return g.fail(err)
	}
	start := time.Now()
	fmt.Fprintln(stdout, "Compilando o seu workspace…")
	if err := g.stack.Build(ctx); err != nil {
		g.event("quick_check_failed", map[string]any{"stage": "build"})
		return g.fail(fmt.Errorf("o build falhou; corrija e rode de novo:\n%w", err))
	}
	built := time.Now()
	fmt.Fprintln(stdout, "Aplicando migrações e reiniciando o app…")
	if err := g.stack.Migrate(ctx); err != nil {
		g.event("quick_check_failed", map[string]any{"stage": "migrate"})
		return g.fail(err)
	}
	migrated := time.Now()
	if err := g.stack.RestartApp(ctx); err != nil {
		g.event("quick_check_failed", map[string]any{"stage": "restart"})
		return g.fail(err)
	}
	applied := time.Now()
	fmt.Fprintln(stdout, "Aquecendo 15 s e medindo 30 s do tráfego real…")
	select {
	case <-ctx.Done():
		return 1
	case <-time.After(15 * time.Second):
	}
	sig, err := g.signals(ctx, 30*time.Second)
	if err != nil {
		return g.fail(err)
	}
	fmt.Fprintf(stdout, "\nQUICK CHECK (indicativo: uma medição curta, sem reset de dados; o que vale é `sesim test`)\n\n")
	sig.Write(stdout, g.session.LastCheck)
	g.session.LastCheck = &sig
	g.session.Checks++
	g.save()
	g.event("quick_check_run", map[string]any{
		"build_ms": built.Sub(start).Milliseconds(), "migrate_ms": migrated.Sub(built).Milliseconds(),
		"restart_ms": applied.Sub(migrated).Milliseconds(),
		"total_ms":   time.Since(start).Milliseconds(),
	})
	fmt.Fprintf(stdout, "\n(%s no total)\n", time.Since(start).Round(time.Second))
	return 0
}

// runHint: the next rung of the hint ladder.
func runHint(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	g, code := openGame(ctx, "hint", args, stdout, stderr, true)
	if g == nil {
		return code
	}
	defer g.close()
	h, ok := play.NextHint(g.pack.Hints, g.session.HintsShown)
	if !ok {
		fmt.Fprintln(stdout, "Não há mais dicas. As anteriores continuam valendo; releia com calma.")
		return 0
	}
	g.session.HintsShown++
	g.save()
	props := map[string]any{"level": h.Level}
	if !g.session.IncidentAt.IsZero() {
		props["incident_minutes"] = int(time.Since(g.session.IncidentAt).Minutes())
	}
	g.event("hint_requested", props)
	fmt.Fprintf(stdout, "Dica %d de %d · %s\n\n%s\n", h.Level, len(g.pack.Hints), h.Kind, h.Text)
	return 0
}

// runShell: interactive shell inside the Nexora network.
func runShell(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	g, code := openGame(ctx, "shell", args, stdout, stderr, true)
	if g == nil {
		return code
	}
	defer g.close()
	if err := g.requireSandbox(); err != nil {
		return g.fail(err)
	}
	g.event("shell_opened", nil)
	fmt.Fprintln(stdout, "Terminal na rede da Nexora (psql já conectado ao banco). `exit` para sair.")
	if err := g.stack.Shell(ctx, os.Stdin, stdout, stderr); err != nil {
		return g.fail(err)
	}
	return 0
}

// runStop: stop the background traffic; with --down also destroy the sandbox
// (the workspace and the progress stay; `sesim play` brings a fresh one up).
func runStop(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	down := slices.Contains(args, "--down")
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--down" })
	g, code := openGame(ctx, "stop", args, stdout, stderr, true)
	if g == nil {
		return code
	}
	defer g.close()
	if err := g.requireSandbox(); err != nil {
		return g.fail(err)
	}
	if err := g.stack.StopBackground(ctx); err != nil {
		return g.fail(err)
	}
	g.event("session_paused", map[string]any{"phase": string(g.session.Phase), "down": down})
	if !down {
		fmt.Fprintln(stdout, "Tráfego parado; a Nexora continua de pé. `sesim play` retoma; `sesim stop --down` desliga tudo.")
		return 0
	}
	g.stack.Close()
	if err := stack.Down(ctx, g.d, g.reg, g.stack.Run); err != nil {
		return g.fail(err)
	}
	g.stack, g.session.Sandbox = nil, ""
	g.save()
	fmt.Fprintln(stdout, "Nexora desligada (nada ficou para trás). Seu workspace e o progresso continuam; `sesim play` sobe de novo.")
	return 0
}
