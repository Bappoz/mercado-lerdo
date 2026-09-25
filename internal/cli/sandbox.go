package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
)

const sandboxUsage = `Usage: sesim sandbox <command> [flags]

Manage Nexora sandboxes created through the Docker Engine API (no compose).

Commands:
  up     create a sandbox for a workspace and print how to reach it
  ls     list sandboxes on this machine
  exec   run a command in a component: exec <run> <component> -- <cmd...>
  down   destroy a sandbox and verify nothing is left: down <run>
  gc     destroy orphaned sandboxes (--all: every sandbox)
`

func runSandbox(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, sandboxUsage)
		return 2
	}
	d, err := docker.New()
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	defer d.Close()
	reg, err := stack.DefaultRegistry()
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	fail := func(err error) int { fmt.Fprintln(stderr, "sesim:", err); return 1 }

	switch args[0] {
	case "up":
		fs := flag.NewFlagSet("sandbox up", flag.ContinueOnError)
		fs.SetOutput(stderr)
		root := fs.String("root", ".", "repository root")
		ws := fs.String("workspace", "nexora", "the app to run")
		cpus := fs.Float64("pg-cpus", 0.5, "CPU quota of postgres")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		// A previous crash may have left a half-built run behind.
		if gone, err := stack.GC(ctx, d, reg, false); err == nil && len(gone) > 0 {
			fmt.Fprintf(stdout, "collected orphaned sandboxes: %v\n", gone)
		}
		log := slog.New(slog.NewTextHandler(stderr, nil))
		s, err := stack.Up(ctx, d, reg, stack.Options{Root: *root, Workspace: *ws, PGCPUs: *cpus}, log)
		if err != nil {
			if s != nil {
				fmt.Fprintf(stderr, "sesim: sandbox %s failed; `sesim sandbox down %s` removes it\n", s.Run, s.Run)
			}
			return fail(err)
		}
		nginx, err := s.NginxURL(ctx)
		if err != nil {
			return fail(err)
		}
		prom, err := s.PrometheusURL(ctx)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "sandbox %s ready\n  app:        %s\n  prometheus: %s\n", s.Run, nginx, prom)
		return 0

	case "ls":
		runs, err := d.Runs(ctx)
		if err != nil {
			return fail(err)
		}
		ids := make([]sandbox.RunID, 0, len(runs))
		for r := range runs {
			ids = append(ids, r)
		}
		slices.Sort(ids)
		fmt.Fprintf(stdout, "%-14s %-9s %-10s %-9s %s\n", "run", "state", "containers", "running", "workspace")
		for _, r := range ids {
			rec, _ := reg.Get(r)
			state, ws := "unknown", ""
			if rec != nil {
				state, ws = string(rec.State), rec.Workspace
			}
			if stack.Orphan(rec) {
				state = "orphan"
			}
			fmt.Fprintf(stdout, "%-14s %-9s %-10d %-9d %s\n", r, state, runs[r].Containers, runs[r].Running, ws)
		}
		return 0

	case "exec":
		if len(args) < 5 || args[3] != "--" {
			fmt.Fprintln(stderr, "usage: sesim sandbox exec <run> <component> -- <cmd...>")
			return 2
		}
		res, err := d.Exec(ctx, sandbox.RunID(args[1]), args[2], 1, args[4:], nil)
		if err != nil {
			return fail(err)
		}
		fmt.Fprint(stdout, res.Stdout)
		fmt.Fprint(stderr, res.Stderr)
		return res.ExitCode

	case "down":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: sesim sandbox down <run>")
			return 2
		}
		if err := stack.Down(ctx, d, reg, sandbox.RunID(args[1])); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "sandbox %s destroyed; no resources left\n", args[1])
		return 0

	case "gc":
		fs := flag.NewFlagSet("sandbox gc", flag.ContinueOnError)
		fs.SetOutput(stderr)
		all := fs.Bool("all", false, "destroy every sandbox, not only orphans")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		gone, err := stack.GC(ctx, d, reg, *all)
		fmt.Fprintf(stdout, "collected %d sandbox(es): %v\n", len(gone), gone)
		if err != nil {
			return fail(err)
		}
		return 0

	default:
		fmt.Fprintf(stderr, "sesim sandbox: unknown command %q\n\n%s", args[0], sandboxUsage)
		return 2
	}
}
