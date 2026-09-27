package stack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/telemetry"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

// This file holds what the game and the evaluator do with a running sandbox.
// *Stack implements evaluator.Env.

// BackgroundComponent runs the story's traffic.
const BackgroundComponent = "k6-background"

// SetWorkspace points later builds and migrations at another copy of the app
// (the scenario harness evaluates every reference in the same sandbox).
func (s *Stack) SetWorkspace(workspace string) error {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("stack: %w", err)
	}
	s.opts.Workspace = abs
	return nil
}

// Prepare builds workspace, applies its topology and starts pushing container
// stats (evaluator.Env). ResetData restarts the app on the new build.
func (s *Stack) Prepare(ctx context.Context, workspace string) error {
	if err := s.SetWorkspace(workspace); err != nil {
		return err
	}
	if err := s.Build(ctx); err != nil {
		return err
	}
	t, err := topology.Load(s.opts.Workspace)
	if err != nil {
		return err
	}
	if _, err := s.Apply(ctx, t, func(line string) { s.log.Info("deploy", "step", line) }); err != nil {
		return err
	}
	return s.StartStats(ctx)
}

// Deploy ships the workspace to the running sandbox: build, pending
// migrations, the topology and nginx configuration, and the new binary on
// every app replica. Data is kept (unlike a Production Test run).
func (s *Stack) Deploy(ctx context.Context, say func(string)) ([]Change, error) {
	t, err := topology.Load(s.opts.Workspace)
	if err != nil {
		return nil, err
	}
	// A crashed replica comes back with the deploy; nginx's config test also
	// needs the app's name to resolve, which a stopped container does not.
	if err := s.StartApp(ctx); err != nil {
		return nil, err
	}
	// Everything that can be refused is refused before anything changes.
	if _, err := s.Preflight(ctx, t); err != nil {
		return nil, err
	}
	say("Compilando o seu workspace…")
	if err := s.Build(ctx); err != nil {
		return nil, fmt.Errorf("o build falhou; corrija e rode de novo:\n%w", err)
	}
	say("Aplicando migrações…")
	if err := s.Migrate(ctx); err != nil {
		return nil, err
	}
	say("Aplicando a topologia…")
	changes, err := s.Apply(ctx, t, say)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(changes, func(c Change) bool { return c.Component == "app" }) {
		say("Reiniciando o app com o build novo…")
		if err := s.RestartApp(ctx); err != nil {
			return changes, err
		}
	}
	return changes, nil
}

// Close stops the container-stats pusher.
func (s *Stack) Close() {
	if s.stopStats != nil {
		s.stopStats()
		s.stopStats = nil
	}
}

// Migrate applies pending workspace migrations to the live database.
func (s *Stack) Migrate(ctx context.Context) error {
	return s.runSeed(ctx, filepath.Join(s.opts.Workspace, "migrations"), "nexora")
}

// ResetData restores the pristine dataset plus the workspace migrations and
// restarts the app on it (evaluator.Env).
func (s *Stack) ResetData(ctx context.Context) error {
	if err := s.d.Stop(ctx, s.Run, "app"); err != nil {
		return err
	}
	if err := s.resetData(ctx); err != nil {
		return err
	}
	if err := s.startHealthy(ctx, "app"); err != nil {
		return err
	}
	return s.Settle(ctx)
}

// RestartApp restarts the app on the last build and waits until the process
// answers. It does not wait for "healthy": under load the app's /healthz
// fails while the database is saturated, which is what the player debugs.
func (s *Stack) RestartApp(ctx context.Context) error {
	if err := s.d.Restart(ctx, s.Run, "app"); err != nil {
		return err
	}
	return s.waitHealth(ctx, 60*time.Second, http.StatusOK, http.StatusServiceUnavailable) // 503 comes from the app itself
}

// Healthy reports whether the store answers /healthz with 200 through nginx
// right now: one quick request, for the story's "the store is back up".
func (s *Stack) Healthy(ctx context.Context) bool {
	base, err := s.NginxURL(ctx)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Settle waits until /healthz answers 200 through nginx (evaluator.Env).
func (s *Stack) Settle(ctx context.Context) error {
	return s.waitHealth(ctx, 90*time.Second, http.StatusOK)
}

func (s *Stack) waitHealth(ctx context.Context, timeout time.Duration, ok ...int) error {
	base, err := s.NginxURL(ctx)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
		if err != nil {
			return err
		}
		if resp, err := s.http.Do(req); err == nil {
			_ = resp.Body.Close()
			for _, code := range ok {
				if resp.StatusCode == code {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("stack: app did not answer within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// RunLoad runs the scenario's profile once and returns k6's summary (evaluator.Env).
func (s *Stack) RunLoad(ctx context.Context, t scenario.Test, runID string) (workload.Result, error) {
	res, err := s.d.RunOnce(ctx, s.Run, s.spec, loadGen(s.opts, "k6", t.Profile, t.Rate, t.Duration, t.Ramp, runID))
	if err != nil {
		return workload.Result{}, err
	}
	if res.ExitCode != 0 {
		return workload.Result{}, fmt.Errorf("stack: k6 exited %d: %s", res.ExitCode, lastLines(res.Stderr, 5))
	}
	return workload.ParseK6Summary(strings.NewReader(res.Stdout))
}

// StartBackground replaces the story's background traffic.
func (s *Stack) StartBackground(ctx context.Context, profile string, rate int, id string) error {
	if err := s.StopBackground(ctx); err != nil {
		return err
	}
	return s.d.Launch(ctx, s.Run, s.spec, loadGen(s.opts, BackgroundComponent, profile, rate, 12*time.Hour, 20*time.Second, id))
}

// StopBackground removes the background traffic, if any.
func (s *Stack) StopBackground(ctx context.Context) error {
	return s.d.RemoveComponent(ctx, s.Run, BackgroundComponent)
}

// CPU returns each component's average CPU use over window as a fraction of
// its quota (all replicas together).
func (s *Stack) CPU(ctx context.Context, window time.Duration) (map[string]float64, error) {
	label := docker.LabelRun + "=" + string(s.Run)
	a, err := s.d.Stats(ctx, label)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(window):
	}
	b, err := s.d.Stats(ctx, label)
	if err != nil {
		return nil, err
	}
	before := map[string]float64{}
	for _, r := range a {
		before[fmt.Sprintf("%s#%d", r.Component, r.Replica)] = r.CPUUsageSeconds
	}
	// A component with replicas is judged as a whole: its busy seconds over
	// the cores all its replicas are allowed.
	used, quota := map[string]float64{}, map[string]float64{}
	for _, r := range b {
		prev, ok := before[fmt.Sprintf("%s#%d", r.Component, r.Replica)]
		if !ok || r.CPUQuotaCores == 0 {
			continue
		}
		used[r.Component] += r.CPUUsageSeconds - prev
		quota[r.Component] += r.CPUQuotaCores
	}
	out := map[string]float64{}
	for c, u := range used {
		out[c] = u / window.Seconds() / quota[c]
	}
	return out, nil
}

// Query runs an instant PromQL query (evaluator.Env).
func (s *Stack) Query(ctx context.Context, promql string, at time.Time) (float64, error) {
	base, err := s.PrometheusURL(ctx)
	if err != nil {
		return 0, err
	}
	return telemetry.QueryScalar(ctx, s.http, base, promql, at)
}

// QueryVector runs an instant PromQL query and returns every sample.
func (s *Stack) QueryVector(ctx context.Context, promql string, at time.Time) ([]telemetry.Series, error) {
	base, err := s.PrometheusURL(ctx)
	if err != nil {
		return nil, err
	}
	return telemetry.QueryVector(ctx, s.http, base, promql, at)
}

// QueryRange runs a PromQL range query.
func (s *Stack) QueryRange(ctx context.Context, promql string, start, end time.Time, step time.Duration) ([]telemetry.RangeSeries, error) {
	base, err := s.PrometheusURL(ctx)
	if err != nil {
		return nil, err
	}
	return telemetry.QueryRange(ctx, s.http, base, promql, start, end, step)
}

// URL is the app through nginx (evaluator.Env). Empty if it cannot be resolved.
func (s *Stack) URL() string {
	u, _ := s.NginxURL(context.Background())
	return u
}

// HTTP is the client used against the sandbox (evaluator.Env).
func (s *Stack) HTTP() *http.Client { return s.http }

// Shell opens an interactive shell in the toolbox. The docker CLI is used on
// purpose: it handles the terminal's raw mode and resizing.
func (s *Stack) Shell(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker", "exec", "-it", docker.ContainerName(s.Run, "toolbox", 1), "bash") //nolint:gosec // G204: fixed binary; the name is built by the driver
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	return cmd.Run()
}

// Terminal opens the same shell as Shell, for a terminal that is not ours
// (the web UI's).
func (s *Stack) Terminal(ctx context.Context, cols, rows uint) (*docker.TTY, error) {
	return s.d.ExecTTY(ctx, s.Run, "toolbox", 1, []string{"bash"}, cols, rows)
}

// StartStats pushes container stats to Prometheus every second until Close.
// It is idempotent.
func (s *Stack) StartStats(ctx context.Context) error {
	if s.stopStats != nil {
		return nil
	}
	base, err := s.PrometheusURL(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	s.stopStats = cancel
	pusher := telemetry.Pusher{URL: base + "/api/v1/otlp/v1/metrics", Service: "sesim"}
	label := docker.LabelRun + "=" + string(s.Run)
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			stats, err := s.d.Stats(ctx, label)
			if err == nil {
				err = pusher.Push(ctx, telemetry.FromResourceStats(stats))
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				s.log.Debug("container stats", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], " | ")
}

// StartApp starts every app replica that is not running (a crash left it
// down) and waits until the app answers. Running replicas are untouched.
func (s *Stack) StartApp(ctx context.Context) error {
	if err := s.d.Start(ctx, s.Run, "app"); err != nil {
		return err
	}
	return s.waitHealth(ctx, 60*time.Second, http.StatusOK, http.StatusServiceUnavailable)
}

// Crash makes replica's process abort, as a panic or an out-of-memory kill
// would (evaluator.Crasher). SIGABRT, not `docker kill`: the runtime must see
// the process die on its own, or a restart policy never applies. The Go
// runtime aborts on SIGABRT even as PID 1.
func (s *Stack) Crash(ctx context.Context, component string, replica int) error {
	res, err := s.d.Exec(ctx, s.Run, component, replica, []string{"kill", "-ABRT", "1"}, nil)
	if errors.Is(err, docker.ErrNotRunning) {
		return nil // already down: a crash has nothing left to crash
	}
	if err != nil {
		return fmt.Errorf("stack: crash %s#%d: %w", component, replica, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("stack: crash %s#%d: kill exited %d: %s", component, replica, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}
