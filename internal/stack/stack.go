package stack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
)

// Stack is one running Nexora sandbox.
type Stack struct {
	Run  sandbox.RunID
	d    *docker.Client
	opts Options
	spec sandbox.Spec
	log  *slog.Logger

	http      *http.Client
	appURL    string // resolved lazily; container IPs are stable for the run
	promURL   string
	stopStats context.CancelFunc
}

// Up creates and starts a Nexora sandbox for opts.Workspace: builds the app,
// seeds the pristine dataset template, clones it for the app, and starts
// every service. On failure the run stays registered as "creating" and is
// collected by GC once this process is gone.
func Up(ctx context.Context, d *docker.Client, reg Registry, opts Options, log *slog.Logger) (*Stack, error) {
	for _, p := range []*string{&opts.Root, &opts.Workspace} {
		abs, err := filepath.Abs(*p)
		if err != nil {
			return nil, fmt.Errorf("stack: %w", err)
		}
		*p = abs
	}
	opts.Secrets = NewSecrets()
	s := newStack(sandbox.NewRunID(), d, opts, log)
	rec := Record{Run: s.Run, PID: os.Getpid(), State: Creating, Workspace: opts.Workspace, Secrets: opts.Secrets, Created: time.Now().UTC()}
	if err := reg.Put(rec); err != nil {
		return nil, err
	}
	step := func(name string, f func() error) error {
		start := time.Now()
		if err := f(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		log.Info("sandbox step", "run", s.Run, "step", name, "ms", time.Since(start).Milliseconds())
		return nil
	}
	var err error
	for _, st := range []struct {
		name string
		f    func() error
	}{
		{"create", func() error { return d.Create(ctx, s.Run, s.spec) }},
		{"build app", func() error { return s.Build(ctx) }},
		{"start postgres", func() error { return s.startHealthy(ctx, "postgres") }},
		{"seed template", func() error { return s.seedTemplate(ctx) }},
		{"reset data", func() error { return s.resetData(ctx) }},
		{"start services", func() error {
			for _, c := range []string{"payments", "app", "nginx", "prometheus", "agent", "postgres-exporter", "nginx-exporter", "toolbox"} {
				if err := s.startHealthy(ctx, c); err != nil {
					return err
				}
			}
			return nil
		}},
	} {
		if err != nil {
			break
		}
		err = step(st.name, st.f)
	}
	if err != nil {
		return s, fmt.Errorf("stack: up %s: %w", s.Run, err)
	}
	rec.State = Ready
	return s, reg.Put(rec)
}

// Attach returns a handle to a registered run.
func Attach(d *docker.Client, rec Record, root string, log *slog.Logger) (*Stack, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("stack: %w", err)
	}
	return newStack(rec.Run, d, Options{Root: abs, Workspace: rec.Workspace, Secrets: rec.Secrets}, log), nil
}

func newStack(run sandbox.RunID, d *docker.Client, o Options, log *slog.Logger) *Stack {
	return &Stack{Run: run, d: d, opts: o, spec: Spec(o), log: log, http: &http.Client{Timeout: 10 * time.Second}}
}

func (s *Stack) startHealthy(ctx context.Context, component string) error {
	if err := s.d.Start(ctx, s.Run, component); err != nil {
		return err
	}
	return s.d.WaitHealthy(ctx, s.Run, component, 90*time.Second)
}

// Build compiles the workspace into the bin volume.
func (s *Stack) Build(ctx context.Context) error {
	res, err := s.d.RunOnce(ctx, s.Run, s.spec, builder(s.opts))
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("go build failed (exit %d):\n%s", res.ExitCode, strings.TrimSpace(res.Stdout+res.Stderr))
	}
	return nil
}

// psql runs each statement as its own transaction, as the postgres superuser
// over the local socket inside the postgres container.
func (s *Stack) psql(ctx context.Context, db string, stmts ...string) error {
	cmd := []string{"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", db, "-At"}
	for _, st := range stmts {
		cmd = append(cmd, "-c", st)
	}
	res, err := s.d.Exec(ctx, s.Run, "postgres", 1, cmd, nil)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("psql: exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// seedTemplate creates nexora_seed from the repository's migrations (the
// scenario start), never the workspace's.
func (s *Stack) seedTemplate(ctx context.Context) error {
	if err := s.psql(ctx, "postgres", "CREATE DATABASE nexora_seed OWNER nexora"); err != nil {
		return err
	}
	if err := s.psql(ctx, "nexora_seed",
		"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
		"GRANT USAGE ON SCHEMA public TO sesim_observer",
		"ALTER DEFAULT PRIVILEGES FOR ROLE nexora IN SCHEMA public GRANT SELECT ON TABLES TO sesim_observer"); err != nil {
		return err
	}
	if err := s.runSeed(ctx, filepath.Join(s.opts.Root, "nexora", "migrations"), "nexora_seed"); err != nil {
		return err
	}
	return s.psql(ctx, "postgres", "ALTER DATABASE nexora_seed WITH IS_TEMPLATE true")
}

func (s *Stack) runSeed(ctx context.Context, migrations, db string) error {
	res, err := s.d.RunOnce(ctx, s.Run, s.spec, seeder(s.opts, migrations, db))
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("seed/migrate %s failed (exit %d):\n%s", db, res.ExitCode, strings.TrimSpace(res.Stdout+res.Stderr))
	}
	return nil
}

// resetData clones the template into nexora and applies the workspace migrations.
func (s *Stack) resetData(ctx context.Context) error {
	if err := s.psql(ctx, "postgres",
		"DROP DATABASE IF EXISTS nexora WITH (FORCE)",
		"CREATE DATABASE nexora TEMPLATE nexora_seed OWNER nexora",
		"REVOKE CONNECT ON DATABASE nexora FROM PUBLIC",
		"GRANT CONNECT ON DATABASE nexora TO nexora, sesim_observer"); err != nil {
		return err
	}
	if err := s.psql(ctx, "nexora", "SELECT pg_stat_statements_reset()"); err != nil {
		return err
	}
	return s.runSeed(ctx, filepath.Join(s.opts.Workspace, "migrations"), "nexora")
}

// Addr returns where the host reaches component (first replica) on network.
func (s *Stack) Addr(ctx context.Context, component, network string) (netip.Addr, error) {
	return s.d.Addr(ctx, s.Run, component, 1, network)
}

// NginxURL is the app's public entry point.
func (s *Stack) NginxURL(ctx context.Context) (string, error) {
	if s.appURL == "" {
		a, err := s.Addr(ctx, "nginx", netPlay)
		if err != nil {
			return "", err
		}
		s.appURL = "http://" + netip.AddrPortFrom(a, 8080).String()
	}
	return s.appURL, nil
}

// PrometheusURL is the metrics store.
func (s *Stack) PrometheusURL(ctx context.Context) (string, error) {
	if s.promURL == "" {
		a, err := s.Addr(ctx, "prometheus", netTelemetry)
		if err != nil {
			return "", err
		}
		s.promURL = "http://" + netip.AddrPortFrom(a, 9090).String()
	}
	return s.promURL, nil
}

// Exec runs cmd in the first replica of component.
func (s *Stack) Exec(ctx context.Context, component string, cmd []string, stdin io.Reader) (docker.ExecResult, error) {
	return s.d.Exec(ctx, s.Run, component, 1, cmd, stdin)
}

// Down destroys the run, verifies nothing is left and forgets it.
func Down(ctx context.Context, d *docker.Client, reg Registry, run sandbox.RunID) error {
	if err := d.Destroy(ctx, run); err != nil {
		return err
	}
	left, err := d.Leftovers(ctx, run)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("stack: %s: resources left after destroy: %s", run, strings.Join(left, "; "))
	}
	return reg.Delete(run)
}

// GC destroys orphaned runs (all managed runs with all=true) and returns them.
func GC(ctx context.Context, d *docker.Client, reg Registry, all bool) ([]sandbox.RunID, error) {
	runs, err := d.Runs(ctx)
	if err != nil {
		return nil, err
	}
	var collected []sandbox.RunID
	var errs []error
	for run := range runs {
		rec, err := reg.Get(run)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !all && !Orphan(rec) {
			continue
		}
		if err := Down(ctx, d, reg, run); err != nil {
			errs = append(errs, err)
			continue
		}
		collected = append(collected, run)
	}
	return collected, errors.Join(errs...)
}
