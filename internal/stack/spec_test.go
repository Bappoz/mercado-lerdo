package stack

import (
	"slices"
	"strings"
	"testing"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

func testOptions() Options {
	return Options{Root: "/home/u/repo", Workspace: "/home/u/repo/workspace", Topology: topology.Default(), Secrets: NewSecrets()}
}

func TestSpecIsValid(t *testing.T) {
	o := testOptions()
	s := Spec(o)
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []sandbox.Container{builder(o), seeder(o, "/home/u/repo/nexora/migrations", "nexora")} {
		one := sandbox.Spec{Networks: s.Networks, Volumes: s.Volumes, Containers: []sandbox.Container{c}}
		if err := one.Validate(); err != nil {
			t.Errorf("%s: %v", c.Component, err)
		}
	}
}

// ADR-0014 / threat model T12: nothing the player controls may share a
// network with the metrics store.
func TestTelemetryNetworkIsolation(t *testing.T) {
	allowed := []string{"prometheus", "agent"}
	for _, c := range Spec(testOptions()).Containers {
		if slices.Contains(c.Networks, netTelemetry) && !slices.Contains(allowed, c.Component) {
			t.Errorf("%s is on the telemetry network", c.Component)
		}
		if c.Component == "prometheus" && !slices.Equal(c.Networks, []string{netTelemetry}) {
			t.Errorf("prometheus must be on the telemetry network only, got %v", c.Networks)
		}
	}
}

func TestBuilderHasNoNetwork(t *testing.T) {
	if n := builder(testOptions()).Networks; len(n) != 0 {
		t.Errorf("builder networks = %v, want none (offline build)", n)
	}
}

func TestNetworksAreInternal(t *testing.T) {
	for _, n := range Spec(testOptions()).Networks {
		if !n.Internal {
			t.Errorf("network %s is not internal", n.Name)
		}
	}
}

func TestSecretsAreRandomAndDistinct(t *testing.T) {
	a, b := NewSecrets(), NewSecrets()
	if a == b || a.Admin == a.App || len(a.Admin) != 32 {
		t.Errorf("secrets %+v %+v", a, b)
	}
}

func TestSpecFollowsTopology(t *testing.T) {
	o := testOptions()
	start := byComponent(playerComponents(o))
	if pg := start["postgres"][0]; pg.Resources.CPUs != 0.5 || !slices.Contains(pg.Cmd, "max_connections=50") {
		t.Errorf("postgres at the scenario start = %+v, want the C1 calibration (0.5 CPU, 50 connections)", pg)
	}

	o.Topology.Components.App.Replicas = 3
	o.Topology.Components.App.DBMaxConns = 7
	o.Topology.Components.PgBouncer = &topology.PgBouncer{Size: topology.S, PoolMode: "transaction", DefaultPoolSize: 20, MaxClientConn: 500}
	o.Topology.Components.App.Database = "pgbouncer"
	spec := Spec(o)
	if err := spec.Validate(); err != nil {
		t.Fatal(err)
	}
	got := byComponent(playerComponents(o))
	if n := len(got["app"]); n != 3 {
		t.Fatalf("app replicas = %d, want 3", n)
	}
	for i, a := range got["app"] {
		if a.Replica != i+1 || a.Env["NEXORA_DB_MAX_CONNS"] != "7" || !strings.Contains(a.Env["NEXORA_DATABASE_URL"], "@pgbouncer:6432/") {
			t.Errorf("app replica %d = %+v", i+1, a.Env)
		}
	}
	if len(got["pgbouncer"]) != 1 || slices.Contains(got["pgbouncer"][0].Networks, netTelemetry) {
		t.Errorf("pgbouncer = %+v", got["pgbouncer"])
	}
}

func TestPlan(t *testing.T) {
	s := &Stack{opts: testOptions()}
	with := func(f func(c *topology.Components)) topology.Topology {
		t := topology.Default()
		f(&t.Components)
		return t
	}
	bouncer := &topology.PgBouncer{Size: topology.S, PoolMode: "transaction", DefaultPoolSize: 20, MaxClientConn: 500}
	tests := []struct {
		name string
		to   topology.Topology
		want []Change
	}{
		{"nothing changed", topology.Default(), nil},
		{"more replicas: nginx must learn the new addresses", with(func(c *topology.Components) { c.App.Replicas = 2 }),
			[]Change{{"app", Recreate}, {"nginx", Recreate}}},
		{"install pgbouncer, app still on postgres", with(func(c *topology.Components) { c.PgBouncer = bouncer }),
			[]Change{{"pgbouncer", Create}}},
		{"app moves to pgbouncer", with(func(c *topology.Components) { c.PgBouncer = bouncer; c.App.Database = "pgbouncer" }),
			[]Change{{"pgbouncer", Create}, {"app", Recreate}, {"nginx", Recreate}}},
		{"postgres setting only", with(func(c *topology.Components) { c.Postgres.MaxConnections = 40 }),
			[]Change{{"postgres", Recreate}}},
		{"nginx size only", with(func(c *topology.Components) { c.Nginx.Size = topology.M }),
			[]Change{{"nginx", Recreate}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Plan(tt.to)
			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf("Plan = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
	s.opts.Topology.Components.PgBouncer = bouncer
	if got, _ := s.Plan(topology.Default()); !slices.Equal(got, []Change{{"pgbouncer", Remove}}) {
		t.Errorf("uninstall plan = %v", got)
	}
	bad := topology.Default()
	bad.Components.App.Replicas = 0
	if _, err := s.Plan(bad); err == nil {
		t.Error("Plan accepted an invalid topology")
	}
}
