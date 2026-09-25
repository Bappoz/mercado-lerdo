// Package devstack drives the local dev stack (deploy/dev/compose.yml) for the
// evaluator. It is a stopgap: from Chunk 8 on, the SandboxDriver provides the
// same operations (docs/architecture/sandbox.md §3) and this package goes away.
package devstack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/telemetry"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

// Stack is the dev stack rooted at a repository checkout.
type Stack struct {
	Root       string // repository root (holds deploy/dev/compose.yml)
	BaseURL    string // nginx, e.g. http://127.0.0.1:8080
	Prometheus string // e.g. http://127.0.0.1:9090
	LogDir     string // k6 stderr logs
	Log        *slog.Logger

	workspace string // absolute path of the app being evaluated
	http      *http.Client
	stopStats context.CancelFunc
}

// New returns a stack with dev defaults. root is made absolute because
// compose resolves relative paths against the build context, not the caller.
func New(root string, log *slog.Logger) *Stack {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return &Stack{
		Root: root, BaseURL: "http://127.0.0.1:8080", Prometheus: "http://127.0.0.1:9090",
		LogDir: filepath.Join(root, ".runs"), Log: log, http: &http.Client{Timeout: 10 * time.Second},
	}
}

// URL is where the oracle and health checks reach the app (through nginx).
func (s *Stack) URL() string { return s.BaseURL }

// HTTP is the client used against the stack.
func (s *Stack) HTTP() *http.Client { return s.http }

// Prepare builds the app from workspace, makes sure the pristine dataset
// template exists, and starts pushing container stats to Prometheus.
func (s *Stack) Prepare(ctx context.Context, workspace string) error {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("devstack: workspace: %w", err)
	}
	s.workspace = abs
	s.Log.Info("building app", "workspace", abs)
	if _, err := s.compose(ctx, true, "build", "app"); err != nil {
		return err
	}
	if err := s.ensureTemplate(ctx); err != nil {
		return err
	}
	return s.startStats(ctx)
}

// Close stops the container-stats bridge.
func (s *Stack) Close() {
	if s.stopStats != nil {
		s.stopStats()
	}
}

// ensureTemplate creates nexora_seed, the pristine scenario-start dataset every
// run is cloned from. It is seeded with the repository's migrations, not the
// workspace's: reference solutions must not leak into the baseline.
func (s *Stack) ensureTemplate(ctx context.Context) error {
	out, err := s.psql(ctx, "postgres", "SELECT count(*) FROM pg_database WHERE datname = 'nexora_seed'")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "1" {
		return nil
	}
	s.Log.Info("creating dataset template nexora_seed (one-off)")
	if _, err := s.psql(ctx, "postgres", "CREATE DATABASE nexora_seed OWNER nexora"); err != nil {
		return err
	}
	if _, err := s.psql(ctx, "nexora_seed",
		"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
		"GRANT USAGE ON SCHEMA public TO sesim_observer",
		"ALTER DEFAULT PRIVILEGES FOR ROLE nexora IN SCHEMA public GRANT SELECT ON TABLES TO sesim_observer"); err != nil {
		return err
	}
	pw := envOr("NEXORA_DB_PASSWORD", "nexora-dev")
	if _, err := s.composeEnv(ctx, false, []string{"NEXORA_SEED_DATABASE_URL=postgres://nexora:" + pw + "@postgres:5432/nexora_seed?sslmode=disable"},
		"run", "--rm", "--no-deps", "-T", "-e", "NEXORA_SEED_DATABASE_URL", "seed", "-migrations", "/migrations"); err != nil {
		return fmt.Errorf("devstack: seed template: %w", err)
	}
	_, err = s.psql(ctx, "postgres", "ALTER DATABASE nexora_seed WITH IS_TEMPLATE true")
	return err
}

// ResetData restores the pristine dataset, applies the workspace migrations
// and restarts the app, so every run measures the same starting state.
func (s *Stack) ResetData(ctx context.Context) error {
	if _, err := s.compose(ctx, true, "stop", "app"); err != nil {
		return err
	}
	if _, err := s.psql(ctx, "postgres",
		"DROP DATABASE IF EXISTS nexora WITH (FORCE)",
		"CREATE DATABASE nexora TEMPLATE nexora_seed OWNER nexora",
		"REVOKE CONNECT ON DATABASE nexora FROM PUBLIC",
		"GRANT CONNECT ON DATABASE nexora TO nexora, sesim_observer"); err != nil {
		return err
	}
	if _, err := s.psql(ctx, "nexora", "SELECT pg_stat_statements_reset()"); err != nil {
		return err
	}
	// The seed tool skips loading (data exists) and applies pending migrations.
	if _, err := s.compose(ctx, true, "run", "--rm", "--no-deps", "-T", "seed", "-migrations", "/migrations"); err != nil {
		return fmt.Errorf("devstack: workspace migrations: %w", err)
	}
	if _, err := s.compose(ctx, true, "up", "-d", "--no-deps", "--wait", "app"); err != nil {
		return err
	}
	return s.Settle(ctx)
}

// Settle waits until the app answers /healthz through nginx.
func (s *Stack) Settle(ctx context.Context) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+"/healthz", nil)
		if err != nil {
			return err
		}
		resp, err := s.http.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return errors.New("devstack: app did not become healthy within 90s")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// RunLoad runs the scenario's profile once and returns k6's view of it.
func (s *Stack) RunLoad(ctx context.Context, t scenario.Test, runID string) (workload.Result, error) {
	if err := os.MkdirAll(s.LogDir, 0o750); err != nil {
		return workload.Result{}, err
	}
	env := []string{
		fmt.Sprintf("RATE=%d", t.Rate), "DURATION=" + t.Duration.String(), "RAMP=" + t.Ramp.String(), "RUN_ID=" + runID,
	}
	args := []string{"run", "--rm", "--no-deps", "-T", "-e", "RATE", "-e", "DURATION", "-e", "RAMP", "-e", "RUN_ID",
		"k6", "run", "-q", "--no-color", "-o", "experimental-prometheus-rw", "--tag", "testid=" + runID,
		"/workloads/nexora/" + t.Profile + ".js"}
	cmd := s.cmd(ctx, false, env, args...)
	var stdout bytes.Buffer
	logFile, err := os.Create(filepath.Join(s.LogDir, runID+".k6.log")) //nolint:gosec // G304: our log dir, name built from a timestamp
	if err != nil {
		return workload.Result{}, err
	}
	defer logFile.Close()
	cmd.Stdout, cmd.Stderr = &stdout, logFile
	if err := cmd.Run(); err != nil {
		return workload.Result{}, fmt.Errorf("devstack: k6 %s: %w (see %s)", runID, err, logFile.Name())
	}
	return workload.ParseK6Summary(&stdout)
}

// startStats pushes container stats every second until Close.
func (s *Stack) startStats(ctx context.Context) error {
	dc, err := docker.New()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	s.stopStats = cancel
	pusher := telemetry.Pusher{URL: s.Prometheus + "/api/v1/otlp/v1/metrics", Service: "sesim"}
	go func() {
		defer dc.Close()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			stats, err := dc.Stats(ctx, "com.docker.compose.project=nexora-dev")
			if err == nil {
				err = pusher.Push(ctx, telemetry.FromResourceStats(stats))
			}
			if err != nil && ctx.Err() == nil {
				s.Log.Debug("container stats", "err", err)
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

// Query runs an instant PromQL query and returns the first sample.
func (s *Stack) Query(ctx context.Context, promql string, at time.Time) (float64, error) {
	return telemetry.QueryScalar(ctx, s.http, s.Prometheus, promql, at)
}

// psql runs each statement as its own -c, i.e. its own transaction
// (DROP/CREATE DATABASE refuse to run inside a transaction block).
func (s *Stack) psql(ctx context.Context, db string, stmts ...string) (string, error) {
	args := []string{"exec", "-T", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", db, "-At"}
	for _, st := range stmts {
		args = append(args, "-c", st)
	}
	return s.compose(ctx, false, args...)
}

func (s *Stack) compose(ctx context.Context, withWorkspace bool, args ...string) (string, error) {
	return s.composeEnv(ctx, withWorkspace, nil, args...)
}

func (s *Stack) composeEnv(ctx context.Context, withWorkspace bool, env []string, args ...string) (string, error) {
	cmd := s.cmd(ctx, withWorkspace, env, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.Stdin = nil // docker compose exec would otherwise consume our stdin
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("devstack: docker compose %s: %w: %s", strings.Join(args[:min(3, len(args))], " "), err, lastLines(errb.String(), 5))
	}
	return out.String(), nil
}

func (s *Stack) cmd(ctx context.Context, withWorkspace bool, env []string, args ...string) *exec.Cmd {
	full := append([]string{"compose", "-f", filepath.Join(s.Root, "deploy/dev/compose.yml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...) //nolint:gosec // G204: fixed binary; args are built by the platform, never by the player
	cmd.Env = append(os.Environ(), env...)
	if withWorkspace && s.workspace != "" {
		cmd.Env = append(cmd.Env,
			"NEXORA_WORKSPACE="+s.workspace,
			"NEXORA_DOCKERFILE="+filepath.Join(s.Root, "images/nexora/Dockerfile"))
	}
	return cmd
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], " | ")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
