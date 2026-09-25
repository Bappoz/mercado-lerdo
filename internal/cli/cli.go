// Package cli implements the sesim commands. It holds no game logic: it only
// parses arguments and delegates to the other packages.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/Bappoz/software-engineering-simulator/internal/buildinfo"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

const usage = `sesim — Software Engineering Simulator (working name)

Usage:
  sesim <command> [flags]

Playing (output in PT-BR):
  play           start (or resume) the scenario: workspace, stack, briefing, traffic
  next           end the calm phase; the incident starts
  status         production signals now (traffic, errors, latency, CPU)
  check          Quick Check: apply your workspace and measure ~45 s (indicative)
  test           Production Test (graded); in a game, shows the debrief on PASS
  hint           the next hint (5 levels)
  shell          terminal inside the Nexora network
  stop           stop the background traffic (--down: also destroy the sandbox)

Tools:
  sandbox          up | ls | exec | down <run> | gc [--all]: sandboxes via the Docker API
  scenario verify  run the broken start and every reference; report unexpected verdicts
  load-summary     read a k6 summary (JSON) on stdin and print per-endpoint stats
  dev              developer commands for the local dev stack
  version          print the binary version
  help             print this help
`

// Run executes the command in args (without the program name) and returns the exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, buildinfo.Read())
		return 0
	case "load-summary":
		res, err := workload.ParseK6Summary(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "sesim:", err)
			return 1
		}
		res.WriteTable(stdout)
		return 0
	case "test":
		return runTest(ctx, args[1:], stdout, stderr)
	case "play":
		return runPlay(ctx, args[1:], stdout, stderr)
	case "next":
		return runNext(ctx, args[1:], stdout, stderr)
	case "status":
		return runStatus(ctx, args[1:], stdout, stderr)
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "hint":
		return runHint(ctx, args[1:], stdout, stderr)
	case "shell":
		return runShell(ctx, args[1:], stdout, stderr)
	case "stop":
		return runStop(ctx, args[1:], stdout, stderr)
	case "scenario":
		if len(args) < 2 || args[1] != "verify" {
			fmt.Fprintln(stderr, "usage: sesim scenario verify [flags]")
			return 2
		}
		return runScenarioVerify(ctx, args[2:], stdout, stderr)
	case "dev":
		return runDev(ctx, args[1:], stdout, stderr)
	case "sandbox":
		return runSandbox(ctx, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "sesim: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
