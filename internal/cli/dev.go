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

const devUsage = `Uso: sesim dev <comando> [opções]

Comandos de desenvolvimento para a stack local (deploy/dev).

Comandos:
  container-stats   amostra os recursos dos containers no Docker e envia ao Prometheus (OTLP)
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
		fmt.Fprintf(stderr, "sesim dev: comando desconhecido %q\n\n%s", args[0], devUsage)
		return 2
	}
}

func runContainerStats(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dev container-stats", flag.ContinueOnError)
	ptFlags(fs, stderr)
	label := fs.String("label", "com.docker.compose.project=nexora-dev", "amostra os containers com este label (chave=valor)")
	url := fs.String("otlp", "http://127.0.0.1:9090/api/v1/otlp/v1/metrics", "endpoint de métricas OTLP/HTTP")
	interval := fs.Duration("interval", 2*time.Second, "intervalo de amostragem")
	once := fs.Bool("once", false, "imprime uma amostra na saída em vez de enviar")
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
	fmt.Fprintf(stderr, "sesim: enviando os recursos dos containers de %s para %s a cada %s (Ctrl-C para parar)\n", *label, *url, *interval)
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
