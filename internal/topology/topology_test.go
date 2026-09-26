package topology

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScenarioStartIsDefault(t *testing.T) {
	got, err := Load("../../nexora")
	if err != nil {
		t.Fatal(err)
	}
	if got != Default() {
		t.Errorf("nexora/topology.yaml = %+v, want Default()", got)
	}
	if c := got.MonthlyCost(); c != 70 {
		t.Errorf("start cost = %v, want 70", c)
	}
}

func TestLoadWithoutFile(t *testing.T) {
	if got, err := Load(t.TempDir()); err != nil || got != Default() {
		t.Errorf("Load(empty) = %+v, %v; want Default()", got, err)
	}
}

func TestParseRejects(t *testing.T) {
	base := func(mod string) string {
		return "apiVersion: sesim.dev/v0alpha1\nkind: Topology\ncomponents:\n" +
			"  nginx: {size: S}\n  app: {size: S, replicas: 1, database: postgres, dbMaxConns: 10}\n" +
			"  postgres: {size: S, maxConnections: 50}\n" + mod
	}
	if _, err := Parse([]byte(base(""))); err != nil {
		t.Fatalf("valid file rejected: %v", err)
	}
	tests := []struct{ name, src, want string }{
		{"unknown component", base("  valkey: {size: S}\n"), "valkey"},
		{"unknown field (typo)", strings.Replace(base(""), "replicas", "replica", 1), "replica"},
		{"image is not settable", strings.Replace(base(""), "nginx: {size: S}", "nginx: {size: S, image: evil}", 1), "image"},
		{"size outside the catalog", strings.Replace(base(""), "postgres: {size: S", "postgres: {size: XL", 1), "postgres.size"},
		{"too many replicas", strings.Replace(base(""), "replicas: 1", "replicas: 9", 1), "app.replicas"},
		{"max_connections too high", strings.Replace(base(""), "maxConnections: 50", "maxConnections: 5000", 1), "maxConnections"},
		{"pgbouncer target without pgbouncer", strings.Replace(base(""), "database: postgres", "database: pgbouncer", 1), "not in the topology"},
		{"bad pool mode", base("  pgbouncer: {size: S, poolMode: statement, defaultPoolSize: 20, maxClientConn: 100}\n"), "poolMode"},
		{"wrong apiVersion", strings.Replace(base(""), "v0alpha1", "v9", 1), "apiVersion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.src))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestMonthlyCost(t *testing.T) {
	top := Default()
	top.Components.App.Replicas = 3
	top.Components.Postgres.Size = M
	top.Components.PgBouncer = &PgBouncer{Size: S, PoolMode: "transaction", DefaultPoolSize: 20, MaxClientConn: 500}
	// nginx S 10 + 3 × app S 20 + postgres M 80 + pgbouncer S 8
	if got := top.MonthlyCost(); got != 158 {
		t.Errorf("cost = %v, want 158", got)
	}
}

func TestInstallKeepsCommentsAndIsIdempotent(t *testing.T) {
	src, err := os.ReadFile("../../nexora/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Install(src, KindPgBouncer)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Topologia da Nexora", "pgbouncer:", "poolMode: transaction", "instalado pelo app Docs"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("installed file lacks %q:\n%s", want, out)
		}
	}
	top, err := Parse(out)
	if err != nil || !top.Installed(KindPgBouncer) || top.Components.App.Database != "postgres" {
		t.Fatalf("installed topology = %+v, %v (the app keeps its database until the player points it)", top, err)
	}
	if _, err := Install(out, KindPgBouncer); !errors.Is(err, ErrInstalled) {
		t.Errorf("second install err = %v, want ErrInstalled", err)
	}
	if _, err := Install(src, KindPostgres); err == nil {
		t.Error("installing a mandatory component should fail")
	}
}

func TestInstallFileWithoutTopology(t *testing.T) {
	ws := t.TempDir()
	if err := InstallFile(ws, KindPgBouncer); err != nil {
		t.Fatal(err)
	}
	top, err := Load(ws)
	if err != nil || !top.Installed(KindPgBouncer) {
		t.Fatalf("Load after install = %+v, %v", top, err)
	}
	if _, err := os.Stat(filepath.Join(ws, File)); err != nil {
		t.Fatal(err)
	}
}
