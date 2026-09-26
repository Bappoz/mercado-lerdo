package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

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
