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

const sandboxUsage = `Uso: sesim sandbox <comando> [opções]

Gerencia as sandboxes do jogo, criadas pela API do Docker Engine (sem compose).

Comandos:
  up     cria uma sandbox para um workspace e mostra como acessá-la
  ls     lista as sandboxes desta máquina
  exec   roda um comando num componente: exec <run> <componente> -- <cmd...>
  down   destrói uma sandbox e confere que nada ficou para trás: down <run>
  gc     destrói as sandboxes órfãs (--all: todas)
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
		ptFlags(fs, stderr)
		root := fs.String("root", ".", "raiz do repositório do jogo")
		ws := fs.String("workspace", "nexora", "o app a rodar")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		// A previous crash may have left a half-built run behind.
		if gone, err := stack.GC(ctx, d, reg, false); err == nil && len(gone) > 0 {
			fmt.Fprintf(stdout, "sandboxes órfãs removidas: %v\n", gone)
		}
		log := slog.New(slog.NewTextHandler(stderr, nil))
		s, err := stack.Up(ctx, d, reg, stack.Options{Root: *root, Workspace: *ws}, log)
		if err != nil {
			if s != nil {
				fmt.Fprintf(stderr, "sesim: a sandbox %s falhou; `sesim sandbox down %s` a remove\n", s.Run, s.Run)
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
		fmt.Fprintf(stdout, "sandbox %s pronta\n  app:        %s\n  prometheus: %s\n", s.Run, nginx, prom)
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
		fmt.Fprintf(stdout, "%-14s %-9s %-10s %-9s %s\n", "run", "estado", "containers", "rodando", "workspace")
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
			fmt.Fprintln(stderr, "uso: sesim sandbox exec <run> <componente> -- <cmd...>")
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
			fmt.Fprintln(stderr, "uso: sesim sandbox down <run>")
			return 2
		}
		if err := stack.Down(ctx, d, reg, sandbox.RunID(args[1])); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "sandbox %s destruída; nenhum recurso ficou para trás\n", args[1])
		return 0

	case "gc":
		fs := flag.NewFlagSet("sandbox gc", flag.ContinueOnError)
		ptFlags(fs, stderr)
		all := fs.Bool("all", false, "destrói todas as sandboxes, não só as órfãs")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		gone, err := stack.GC(ctx, d, reg, *all)
		fmt.Fprintf(stdout, "%d sandbox(es) removida(s): %v\n", len(gone), gone)
		if err != nil {
			return fail(err)
		}
		return 0

	default:
		fmt.Fprintf(stderr, "sesim sandbox: comando desconhecido %q\n\n%s", args[0], sandboxUsage)
		return 2
	}
}
