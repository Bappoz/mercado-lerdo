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
  help           print this help
`

// Run executes the command in args (without the program name) and returns the exit code.
func Run(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "sesim: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
