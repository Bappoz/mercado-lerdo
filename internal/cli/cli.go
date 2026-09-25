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
  sesim <command>

Commands:
  version        print the binary version
  load-summary   read a k6 summary (JSON) on stdin and print per-endpoint stats
  test           run the scenario's Production Test against a workspace (dev stack)
  scenario       scenario tools: "scenario verify" checks the broken start and every reference
  dev            developer commands for the local dev stack
  help           print this help
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
	case "scenario":
		if len(args) < 2 || args[1] != "verify" {
			fmt.Fprintln(stderr, "usage: sesim scenario verify [flags]")
			return 2
		}
		return runScenarioVerify(ctx, args[2:], stdout, stderr)
	case "dev":
		return runDev(ctx, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "sesim: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
