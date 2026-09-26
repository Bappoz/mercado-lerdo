// Package topology reads the player's topology.yaml: which components the
// Nexora runs, how many and how big. It is a closed catalog (ADR-0010): the
// file can only pick among components, sizes and bounded settings the
// platform defines, never images, mounts or capabilities. The sandbox spec
// and the modeled cost are derived from it.
package topology

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// APIVersion of topology.yaml (unstable).
const APIVersion = "sesim.dev/v0alpha1"

// File is where a workspace keeps its topology.
const File = "topology.yaml"

// Size is a component's resource tier.
type Size string

// Sizes, smallest first.
const (
	S Size = "S"
	M Size = "M"
	L Size = "L"
)

// Topology is the whole topology.yaml.
type Topology struct {
	APIVersion string     `yaml:"apiVersion" json:"apiVersion"`
	Kind       string     `yaml:"kind" json:"kind"`
	Components Components `yaml:"components" json:"components"`
}

// Components of the Nexora. nginx, app and postgres are always there;
// the rest are optional and absent when nil.
type Components struct {
	Nginx     Nginx      `yaml:"nginx" json:"nginx"`
	App       App        `yaml:"app" json:"app"`
	Postgres  Postgres   `yaml:"postgres" json:"postgres"`
	PgBouncer *PgBouncer `yaml:"pgbouncer,omitempty" json:"pgbouncer,omitempty"`
}

// Nginx is the entry point. Its configuration is the workspace's
// nginx/default.conf.
type Nginx struct {
	Size Size `yaml:"size" json:"size"`
}

// App is the Nexora service, built from the workspace.
type App struct {
	Size     Size `yaml:"size" json:"size"`
	Replicas int  `yaml:"replicas" json:"replicas"`
	// Database is where the app connects: "postgres" or "pgbouncer".
	Database string `yaml:"database" json:"database"`
	// DBMaxConns is the size of each replica's connection pool (NEXORA_DB_MAX_CONNS).
	DBMaxConns int `yaml:"dbMaxConns" json:"dbMaxConns"`
}

// Postgres is the database.
type Postgres struct {
	Size           Size `yaml:"size" json:"size"`
	MaxConnections int  `yaml:"maxConnections" json:"maxConnections"`
}

// PgBouncer is a connection pooler between the app and Postgres.
type PgBouncer struct {
	Size            Size   `yaml:"size" json:"size"`
	PoolMode        string `yaml:"poolMode" json:"poolMode"` // transaction | session
	DefaultPoolSize int    `yaml:"defaultPoolSize" json:"defaultPoolSize"`
	MaxClientConn   int    `yaml:"maxClientConn" json:"maxClientConn"`
}

// Default is the topology of the Stage 1 start, and of workspaces created
// before topology.yaml existed.
func Default() Topology {
	return Topology{
		APIVersion: APIVersion, Kind: "Topology",
		Components: Components{
			Nginx:    Nginx{Size: S},
			App:      App{Size: S, Replicas: 1, Database: "postgres", DBMaxConns: 10},
			Postgres: Postgres{Size: S, MaxConnections: 50},
		},
	}
}

// Parse decodes and validates a topology. Unknown fields are errors: a typo
// must not silently fall back to a default.
func Parse(b []byte) (Topology, error) {
	var t Topology
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&t); err != nil && !errors.Is(err, io.EOF) {
		return Topology{}, fmt.Errorf("topology: %w", err)
	}
	if err := t.Validate(); err != nil {
		return Topology{}, err
	}
	return t, nil
}

// Load reads <workspace>/topology.yaml; a workspace without one gets Default.
func Load(workspace string) (Topology, error) {
	b, err := os.ReadFile(filepath.Join(workspace, File)) //nolint:gosec // G304: the player's own workspace
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Topology{}, fmt.Errorf("topology: %w", err)
	}
	return Parse(b)
}

// Validate checks every field against the catalog.
func (t Topology) Validate() error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if t.APIVersion != APIVersion || t.Kind != "Topology" {
		fail("apiVersion/kind must be %s/Topology", APIVersion)
	}
	c := t.Components
	size := func(kind Kind, s Size) {
		if _, ok := Catalog[kind].Sizes[s]; !ok {
			fail("%s.size: %q is not one of %v", kind, s, Catalog[kind].sizeNames())
		}
	}
	between := func(field string, v, lo, hi int) {
		if v < lo || v > hi {
			fail("%s: %d is outside [%d, %d]", field, v, lo, hi)
		}
	}
	size(KindNginx, c.Nginx.Size)
	size(KindApp, c.App.Size)
	size(KindPostgres, c.Postgres.Size)
	between("app.replicas", c.App.Replicas, 1, 4)
	between("app.dbMaxConns", c.App.DBMaxConns, 1, 100)
	between("postgres.maxConnections", c.Postgres.MaxConnections, 20, 400)
	switch c.App.Database {
	case "postgres":
	case "pgbouncer":
		if c.PgBouncer == nil {
			fail("app.database: pgbouncer is not in the topology (add components.pgbouncer)")
		}
	default:
		fail("app.database: %q is not postgres or pgbouncer", c.App.Database)
	}
	if b := c.PgBouncer; b != nil {
		size(KindPgBouncer, b.Size)
		if b.PoolMode != "transaction" && b.PoolMode != "session" {
			fail("pgbouncer.poolMode: %q is not transaction or session", b.PoolMode)
		}
		between("pgbouncer.defaultPoolSize", b.DefaultPoolSize, 1, 200)
		between("pgbouncer.maxClientConn", b.MaxClientConn, 10, 2000)
	}
	return errors.Join(errs...)
}

// CostMeasure is the evaluator measure of MonthlyCost. It may only feed the
// cost dimension: objectives grade properties, never technologies (ADR-0010).
const CostMeasure = "monthly_cost"

// MonthlyCost is the modeled infrastructure cost, in US$ per month.
func (t Topology) MonthlyCost() float64 {
	c := t.Components
	total := Catalog[KindNginx].Sizes[c.Nginx.Size].Price +
		float64(c.App.Replicas)*Catalog[KindApp].Sizes[c.App.Size].Price +
		Catalog[KindPostgres].Sizes[c.Postgres.Size].Price
	if b := c.PgBouncer; b != nil {
		total += Catalog[KindPgBouncer].Sizes[b.Size].Price
	}
	return total
}
