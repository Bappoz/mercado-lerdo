package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

var actionWords = map[stack.Action]string{stack.Create: "criar", stack.Recreate: "recriar", stack.Remove: "remover"}

// runDeploy: ship the workspace to the running Nexora, keeping its data.
func runDeploy(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "deploy", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	changes, err := t.g.Deploy(ctx, t.say)
	if err != nil {
		return t.fail(err)
	}
	if len(changes) == 0 {
		fmt.Fprintln(stdout, "Deploy feito: código e migrações; a topologia não mudou.")
		return 0
	}
	fmt.Fprintln(stdout, "Deploy feito. Na topologia:")
	for _, c := range changes {
		fmt.Fprintf(stdout, "  %-10s %s\n", c.Component, actionWords[c.Action])
	}
	return 0
}

// runTopology: the workspace's topology, its cost and what a deploy would change.
func runTopology(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "topology", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	v := t.g.Topology()
	if v.WorkspaceError != "" {
		return t.fail(fmt.Errorf("topology.yaml inválido:\n%s", v.WorkspaceError))
	}
	writeTopology(stdout, *v.Workspace)
	fmt.Fprintf(stdout, "\nCusto modelado: US$ %.0f/mês", v.WorkspaceCost)
	if v.Budget > 0 {
		fmt.Fprintf(stdout, " (orçamento: US$ %.0f)", v.Budget)
	}
	fmt.Fprintln(stdout)
	switch {
	case v.Deployed == nil:
		fmt.Fprintln(stdout, "A Nexora não está rodando.")
	case len(v.Pending) == 0:
		fmt.Fprintln(stdout, "Em produção: igual ao workspace.")
	default:
		fmt.Fprintf(stdout, "Em produção custa US$ %.0f/mês. O próximo `sesim deploy` vai:\n", v.DeployedCost)
		for _, c := range v.Pending {
			fmt.Fprintf(stdout, "  %s %s\n", actionWords[c.Action], c.Component)
		}
	}
	return 0
}

func writeTopology(w io.Writer, t topology.Topology) {
	c := t.Components
	line := func(k topology.Kind, size topology.Size, n int, extra string) {
		tier := topology.Catalog[k].Sizes[size]
		fmt.Fprintf(w, "  %-10s %s ×%d  %.2g CPU  %4d MiB  US$ %3.0f  %s\n",
			k, size, n, tier.CPUs, tier.Memory>>20, tier.Price*float64(n), extra)
	}
	line(topology.KindNginx, c.Nginx.Size, 1, "config: nginx/default.conf")
	line(topology.KindApp, c.App.Size, c.App.Replicas, fmt.Sprintf("→ %s, pool de %d por réplica", c.App.Database, c.App.DBMaxConns))
	if b := c.PgBouncer; b != nil {
		line(topology.KindPgBouncer, b.Size, 1, fmt.Sprintf("modo %s, %d conexões com o banco", b.PoolMode, b.DefaultPoolSize))
	}
	line(topology.KindPostgres, c.Postgres.Size, 1, fmt.Sprintf("max_connections %d", c.Postgres.MaxConnections))
}

// runDocs: the catalog, or one component's page.
func runDocs(_ context.Context, args []string, stdout, stderr io.Writer) int {
	kinds := make([]string, 0, len(topology.Catalog))
	for k := range topology.Catalog {
		kinds = append(kinds, string(k))
	}
	slices.Sort(kinds)
	if len(args) == 0 {
		fmt.Fprintln(stdout, "Componentes do catálogo (sesim docs <componente>):")
		for _, k := range kinds {
			e := topology.Catalog[topology.Kind(k)]
			fmt.Fprintf(stdout, "  %-10s %s\n", k, e.Summary)
		}
		return 0
	}
	e, ok := topology.Catalog[topology.Kind(args[0])]
	if !ok {
		fmt.Fprintf(stderr, "sesim: componente desconhecido %q (há: %s)\n", args[0], strings.Join(kinds, ", "))
		return 2
	}
	fmt.Fprintf(stdout, "%s\n\n%s\n\nQuando usar: %s\n\nCustos e cuidados: %s\n\nConfiguração em topology.yaml:\n", e.Name, e.Summary, e.WhenToUse, e.Tradeoffs)
	for _, s := range e.Settings {
		fmt.Fprintf(stdout, "  %-16s %-22s %s\n", s.Field, s.Range, s.Doc)
	}
	fmt.Fprintln(stdout, "\nTamanhos:")
	for _, size := range []topology.Size{topology.S, topology.M, topology.L} {
		if t, ok := e.Sizes[size]; ok {
			fmt.Fprintf(stdout, "  %s  %.2g CPU  %4d MiB  US$ %.0f/mês\n", size, t.CPUs, t.Memory>>20, t.Price)
		}
	}
	if e.Optional {
		fmt.Fprintf(stdout, "\nInstalar: sesim install %s (edita o topology.yaml; aplique com sesim deploy)\n", e.Kind)
	}
	return 0
}

// runInstall: add an optional component to the workspace's topology.yaml.
func runInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, "usage: sesim install <component> [flags]")
		return 2
	}
	k := topology.Kind(args[0])
	t, code := openGame(ctx, "install", args[1:], stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	if err := t.g.Install(k); err != nil {
		return t.fail(err)
	}
	fmt.Fprintf(stdout, "%s adicionado ao topology.yaml. Nada muda em produção até o próximo `sesim deploy`.\n", topology.Catalog[k].Name)
	return 0
}

// runSupport: what customers complained about in the last ten minutes.
func runSupport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "support", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	tickets, err := t.g.Tickets(ctx)
	if err != nil {
		return t.fail(err)
	}
	t.g.Event("support_viewed", map[string]any{"view": "cli", "tickets": len(tickets)})
	if len(tickets) == 0 {
		fmt.Fprintln(stdout, "Nenhuma reclamação nos últimos 10 minutos.")
		return 0
	}
	for _, k := range tickets {
		fmt.Fprintf(stdout, "%-38s ×%-4d \"%s\"\n", k.Subject, k.Count, k.Quote)
	}
	return 0
}

// runLab: an ungraded load experiment against production as it is now.
func runLab(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lab", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f gameFlags
	f.register(fs)
	var p game.LabParams
	fs.StringVar(&p.Profile, "profile", "rush", "load profile: "+strings.Join(game.LabProfiles, ", "))
	fs.IntVar(&p.Rate, "rate", 40, "requests per second at peak (1-150)")
	fs.IntVar(&p.Duration, "duration", 40, "seconds at peak, after a 10 s ramp (20-120)")
	fs.BoolVar(&p.Crash, "crash", false, "crash the app's first instance halfway")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	g, err := f.open(ctx, slog.New(slog.DiscardHandler))
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	t := &terminal{g: g, stdout: stdout, stderr: stderr}
	defer g.Close()
	if !g.HasSession() {
		return t.fail(game.ErrNoSession)
	}
	r, err := g.Lab(ctx, p, t.say)
	if err != nil {
		return t.fail(err)
	}
	fmt.Fprintf(stdout, "\n  %-10s %9s %8s %9s %9s %9s\n", "página", "requests", "erros", "p50", "p95", "p99")
	for _, e := range r.Endpoints {
		fmt.Fprintf(stdout, "  %-10s %9d %7.2f%% %7.0fms %7.0fms %7.0fms\n", e.Name, e.Requests, e.ErrorRate*100, e.P50ms, e.P95ms, e.P99ms)
	}
	fmt.Fprintf(stdout, "\nErros no total: %.2f%%", r.ErrorRate*100)
	if r.Crashed {
		fmt.Fprint(stdout, " (com a instância 1 derrubada no meio)")
	}
	fmt.Fprintln(stdout)
	if in := r.Payments; in != nil {
		fmt.Fprintf(stdout, "Cobranças × pedidos: %d cobrados sem pedido, %d cobrados em dobro, %d pedidos duplicados, %d pagos sem cobrança.\n",
			in.Lost, in.DuplicateCharges, in.DuplicateOrders, in.Phantom)
	}
	return 0
}

var missionMark = map[string]string{"done": "[x]", "current": "[>]", "locked": "[ ]"}

// runCampaign: the stage's missions.
func runCampaign(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "campaign", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	v, err := t.g.Campaign()
	if err != nil {
		return t.fail(err)
	}
	fmt.Fprintln(stdout, v.Stage)
	fmt.Fprintln(stdout)
	for _, m := range v.Missions {
		fmt.Fprintf(stdout, "  %s %s", missionMark[m.State], m.Title)
		if r := m.Result; r != nil {
			fmt.Fprintf(stdout, "  (%d min, %d testes, %d dicas%s)", int(r.DoneAt.Sub(r.StartedAt).Minutes()), r.Tests, r.Hints,
				map[bool]string{true: ", do estado canônico", false: ""}[r.Canonical])
		}
		fmt.Fprintln(stdout)
	}
	if v.CanAdvance {
		fmt.Fprintln(stdout, "\nPróximo: `sesim advance` (com o seu código) ou `sesim advance --canonical` (do estado canônico).")
	}
	return 0
}

// runAdvance: move a completed game to the next scenario of its stage.
func runAdvance(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	canonical := slices.Contains(args, "--canonical")
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--canonical" })
	t, code := openGame(ctx, "advance", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	next, err := t.g.Advance(ctx, canonical, t.say)
	if err != nil {
		return t.fail(err)
	}
	fmt.Fprintf(stdout, "Pronto: próximo cenário, %s. `sesim play` sobe a Nexora e mostra o briefing.\n", next.Manifest.Metadata.Title)
	return 0
}
