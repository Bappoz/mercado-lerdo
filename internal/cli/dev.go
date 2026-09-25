package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox/docker"
	"github.com/Bappoz/software-engineering-simulator/internal/telemetry"
)

const devUsage = `Usage: sesim dev <command> [flags]

Developer commands for the local dev stack (deploy/dev).

Commands:
  container-stats   sample container resources from Docker and push them to Prometheus (OTLP)
`

func runDev(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, devUsage)
		return 2
	}
	switch args[0] {
	case "container-stats":
		return runContainerStats(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "sesim dev: unknown command %q\n\n%s", args[0], devUsage)
		return 2
	}
}

func runContainerStats(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("container-stats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	label := fs.String("label", "com.docker.compose.project=nexora-dev", "sample containers with this label (key=value)")
	url := fs.String("otlp", "http://127.0.0.1:9090/api/v1/otlp/v1/metrics", "OTLP/HTTP metrics endpoint")
	interval := fs.Duration("interval", 2*time.Second, "sampling interval")
	once := fs.Bool("once", false, "print one sample to stdout instead of pushing")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	dc, err := docker.New()
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	defer dc.Close()

	if *once {
		stats, err := dc.Stats(ctx, *label)
		if err != nil {
			fmt.Fprintln(stderr, "sesim:", err)
			return 1
		}
		for _, s := range telemetry.FromResourceStats(stats) {
			fmt.Fprintf(stdout, "%s{container=%q,replica=%q} %g\n", s.Name, s.Labels["container"], s.Labels["replica"], s.Value)
		}
		return 0
	}

	pusher := telemetry.Pusher{URL: *url, Service: "sesim"}
	fmt.Fprintf(stderr, "sesim: pushing container stats for %s to %s every %s (Ctrl-C to stop)\n", *label, *url, *interval)
	tick := time.NewTicker(*interval)
	defer tick.Stop()
	for {
		stats, err := dc.Stats(ctx, *label)
		if err == nil {
			err = pusher.Push(ctx, telemetry.FromResourceStats(stats))
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			// Keep going: a restarting container or a Prometheus restart is transient.
			fmt.Fprintln(stderr, "sesim:", err)
		}
		select {
		case <-ctx.Done():
			return 0
		case <-tick.C:
		}
	}
}
