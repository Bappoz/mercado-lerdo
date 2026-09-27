// Package stack runs the Nexora sandbox on the sandbox driver: the concrete
// spec (containers, networks, limits) and the orchestration to bring it up,
// reset its data and tear it down. It replaces deploy/dev/compose.yml for the
// game (docs/architecture/sandbox.md).
package stack

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// Images, pinned by digest. sesim/* images are built locally by `just images`.
const (
	imagePostgres   = "postgres:18.6-alpine3.24@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873"
	imageNginx      = "nginxinc/nginx-unprivileged:1.30-alpine3.24@sha256:4714e0b1b2577eaa1a6131d07c958b67f0eb68e6d0521e90c6e5287db8cf0bc5"
	imageAlpine     = "alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6"
	imageGo         = "golang:1.27.1-alpine3.24@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414"
	imagePrometheus = "prom/prometheus:v3.14.0@sha256:5ce7540c3c00ef4ab0c9d2c995c6a5b9c421f44b4a115d97a2c7af3b1c21cbb0"
	imagePGExporter = "prometheuscommunity/postgres-exporter:v0.20.1@sha256:ac5ec343104fae0e2d84a27bb8d69b38430a11910c5382cad85d478d2bab713e"
	imageNGExporter = "nginx/nginx-prometheus-exporter:1.5.3@sha256:85666e7fde7e253f1c6d978f9a961245df4c9cdb7677e1866d9bb28e8f9ee41d"
	imageSeed       = "sesim/nexora-seed:dev"
	imageObserver   = "sesim/observer-svc:dev"
	imageToolbox    = "sesim/toolbox:dev"
	imagePgBouncer  = "sesim/pgbouncer:dev"
	imageK6         = "grafana/k6:2.3.0@sha256:9c2dee7f8ed74d317e4027c06a10f169b625638189de8d4555d0b3486a5aeb34"
)

// Networks (ADR-0014). All internal: nothing in the sandbox reaches the LAN or the internet.
const (
	netPlay      = "play"      // the player's system
	netIngress   = "ingress"   // load generator -> nginx
	netTelemetry = "telemetry" // metric writes -> prometheus
)

// Options parameterize the Nexora sandbox.
type Options struct {
	Root      string // repository root (configs under deploy/dev)
	Workspace string // the player's app
	// Topology is what the player's system runs (the deployed one, which the
	// workspace file may have moved past). Its S sizes are the C1 calibration.
	Topology topology.Topology
	Secrets  Secrets
	// PaymentLatency is the emulated gateway's "median,p99" (scenario world).
	PaymentLatency string
}

// Secrets are per-run database passwords, never shown to the player.
type Secrets struct {
	Admin, App, Observer string
}

// NewSecrets returns random passwords.
func NewSecrets() Secrets {
	pw := func() string {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}
	return Secrets{Admin: pw(), App: pw(), Observer: pw()}
}

const (
	mb   = int64(1) << 20
	secs = time.Second
)

func health(cmd ...string) *sandbox.Health {
	return &sandbox.Health{Cmd: cmd, Interval: 2 * secs, Timeout: 2 * secs, Retries: 20}
}

func bind(src, target string) sandbox.Mount {
	return sandbox.Mount{Type: sandbox.BindMount, Source: src, Target: target, ReadOnly: true}
}

// wsBind is bind for a path the player controls: inside the workspace, it
// may not lead out of it through a symlink, which the daemon would follow on
// the host as root (threat model T17b).
func wsBind(o Options, src, target string) sandbox.Mount {
	m := bind(src, target)
	if rel, err := filepath.Rel(o.Workspace, src); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
		m.Within = o.Workspace
	}
	return m
}

func tmpfs(target string) sandbox.Mount {
	return sandbox.Mount{Type: sandbox.TmpfsMount, Target: target}
}

// Spec is the long-running part of the Nexora sandbox.
func Spec(o Options) sandbox.Spec {
	dev := func(p string) string { return filepath.Join(o.Root, "deploy", "dev", p) }
	spec := sandbox.Spec{
		Networks: []sandbox.Network{
			{Name: netPlay, Internal: true}, {Name: netIngress, Internal: true}, {Name: netTelemetry, Internal: true},
		},
		Volumes: []sandbox.Volume{{Name: "pgdata"}, {Name: "bin"}, {Name: "promdata"}, {Name: "go-build", Cache: true}},
		Containers: []sandbox.Container{
			{
				Component: "payments", Replica: 1, Image: imageObserver, Networks: []string{netPlay}, ReadOnly: true,
				Env:       map[string]string{"PAYMENTS_LATENCY": o.PaymentLatency},
				Resources: sandbox.Resources{CPUs: 0.25, Memory: 64 * mb, Pids: 64},
				Health:    health("wget", "-qO-", "http://127.0.0.1:8090/healthz"),
			},
			{
				Component: "toolbox", Replica: 1, Image: imageToolbox, Networks: []string{netPlay},
				Env:       map[string]string{"PGHOST": "postgres", "PGUSER": "nexora", "PGDATABASE": "nexora", "PGPASSWORD": o.Secrets.App},
				Resources: sandbox.Resources{CPUs: 0.5, Memory: 128 * mb, Pids: 256},
			},
			{
				Component: "postgres-exporter", Replica: 1, Image: imagePGExporter, Networks: []string{netPlay}, ReadOnly: true,
				Cmd: []string{"--collector.stat_statements", "--collector.stat_statements.include_query", "--collector.stat_statements.limit=20"},
				Env: map[string]string{
					"DATA_SOURCE_URI": "postgres:5432/nexora?sslmode=disable", "DATA_SOURCE_USER": "sesim_observer", "DATA_SOURCE_PASS": o.Secrets.Observer,
				},
				Resources: sandbox.Resources{CPUs: 0.1, Memory: 64 * mb, Pids: 32},
			},
			{
				Component: "nginx-exporter", Replica: 1, Image: imageNGExporter, Networks: []string{netPlay}, ReadOnly: true,
				Cmd:       []string{"--nginx.scrape-uri=http://nginx:8081/stub_status"},
				Resources: sandbox.Resources{CPUs: 0.1, Memory: 32 * mb, Pids: 32},
			},
			{
				Component: "prometheus", Replica: 1, Image: imagePrometheus, Networks: []string{netTelemetry}, ReadOnly: true,
				Cmd: []string{"--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus",
					"--storage.tsdb.retention.time=6h", "--web.enable-remote-write-receiver", "--web.enable-otlp-receiver"},
				Mounts: []sandbox.Mount{
					bind(dev("prometheus/prometheus.yml"), "/etc/prometheus/prometheus.yml"),
					{Type: sandbox.VolumeMount, Source: "promdata", Target: "/prometheus"},
				},
				Resources: sandbox.Resources{CPUs: 0.5, Memory: 512 * mb, Pids: 128},
				Health:    health("wget", "-qO-", "http://127.0.0.1:9090/-/ready"),
			},
			{
				Component: "agent", Replica: 1, Image: imagePrometheus, Networks: []string{netPlay, netTelemetry}, ReadOnly: true,
				Cmd:       []string{"--agent", "--config.file=/etc/prometheus/agent.yml", "--storage.agent.path=/tmp/agent"},
				Mounts:    []sandbox.Mount{bind(dev("prometheus/agent.yml"), "/etc/prometheus/agent.yml"), tmpfs("/tmp")},
				Resources: sandbox.Resources{CPUs: 0.25, Memory: 128 * mb, Pids: 64},
			},
		},
	}
	spec.Containers = append(spec.Containers, playerComponents(o)...)
	return spec
}

// playerComponents are the containers topology.yaml decides, in start order.
func playerComponents(o Options) []sandbox.Container {
	c := o.Topology.Components
	tier := func(k topology.Kind, s topology.Size) topology.Tier { return topology.Catalog[k].Sizes[s] }
	res := func(t topology.Tier, pids int64) sandbox.Resources {
		return sandbox.Resources{CPUs: t.CPUs, Memory: t.Memory, Pids: pids}
	}
	out := []sandbox.Container{{
		Component: "postgres", Replica: 1, Image: imagePostgres,
		// synchronous_commit=off: a commit does not wait for the WAL fsync. The
		// sandbox is disposable, and fsync latency is the host disk's (SSD,
		// HDD, Docker Desktop), which must not decide a verdict.
		Cmd: []string{"postgres", "-c", "max_connections=" + strconv.Itoa(c.Postgres.MaxConnections),
			"-c", "shared_buffers=64MB", "-c", "shared_preload_libraries=pg_stat_statements",
			"-c", "synchronous_commit=off"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": o.Secrets.Admin, "NEXORA_DB_PASSWORD": o.Secrets.App, "SESIM_OBSERVER_PASSWORD": o.Secrets.Observer,
		},
		Networks: []string{netPlay},
		Mounts: []sandbox.Mount{
			{Type: sandbox.VolumeMount, Source: "pgdata", Target: "/var/lib/postgresql"},
			bind(filepath.Join(o.Root, "deploy", "dev", "initdb"), "/docker-entrypoint-initdb.d"),
			tmpfs("/var/run/postgresql"), tmpfs("/tmp"),
		},
		ReadOnly: true,
		// The official entrypoint starts as root, fixes ownership, then drops to postgres.
		CapAdd:    []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID"},
		Resources: res(tier(topology.KindPostgres, c.Postgres.Size), 256),
		ShmBytes:  64 * mb,
		// TCP, not the socket: during first init the entrypoint runs a temporary
		// server that listens on the socket only and is then shut down.
		Health: health("pg_isready", "-h", "127.0.0.1", "-U", "postgres", "-d", "nexora"),
	}}
	dbHost := "postgres:5432"
	if b := c.PgBouncer; b != nil {
		out = append(out, sandbox.Container{
			Component: "pgbouncer", Replica: 1, Image: imagePgBouncer, Networks: []string{netPlay}, ReadOnly: true,
			Env: map[string]string{
				"PGB_DB_HOST": "postgres", "PGB_USER": "nexora", "PGB_PASSWORD": o.Secrets.App,
				"PGB_POOL_MODE": b.PoolMode, "PGB_DEFAULT_POOL_SIZE": strconv.Itoa(b.DefaultPoolSize),
				"PGB_MAX_CLIENT_CONN": strconv.Itoa(b.MaxClientConn),
			},
			Mounts:    []sandbox.Mount{tmpfs("/tmp")},
			Resources: res(tier(topology.KindPgBouncer, b.Size), 64),
			Health:    health("nc", "-z", "127.0.0.1", "6432"),
		})
		if c.App.Database == "pgbouncer" {
			dbHost = "pgbouncer:6432"
		}
	}
	for i := 1; i <= c.App.Replicas; i++ {
		out = append(out, sandbox.Container{
			Component: "app", Replica: i, Image: imageAlpine, User: "10001:10001",
			Entrypoint: []string{"/app/nexora"},
			Env: map[string]string{
				"NEXORA_DATABASE_URL": "postgres://nexora:" + o.Secrets.App + "@" + dbHost + "/nexora?sslmode=disable",
				"NEXORA_PAYMENT_URL":  "http://payments:8090", "NEXORA_DB_MAX_CONNS": strconv.Itoa(c.App.DBMaxConns),
			},
			Networks:       []string{netPlay},
			Mounts:         []sandbox.Mount{{Type: sandbox.VolumeMount, Source: "bin", Target: "/app", ReadOnly: true}, tmpfs("/tmp")},
			ReadOnly:       true,
			Resources:      res(tier(topology.KindApp, c.App.Size), 256),
			Health:         health("wget", "-qO-", "http://127.0.0.1:8080/healthz"),
			RestartOnCrash: c.App.AutoRestart,
		})
	}
	conf := NginxConf(o)
	out = append(out, sandbox.Container{
		Component: "nginx", Replica: 1, Image: imageNginx, Networks: []string{netPlay, netIngress},
		// Not read by nginx: a changed config must change the spec, so a
		// deploy recreates the container (an editor that replaces the file
		// would leave a bind mount on the old inode anyway).
		Env:       map[string]string{"SESIM_CONF_SHA256": fileHash(conf)},
		Mounts:    []sandbox.Mount{wsBind(o, conf, "/etc/nginx/conf.d/default.conf"), tmpfs("/tmp")},
		ReadOnly:  true,
		Resources: res(tier(topology.KindNginx, c.Nginx.Size), 128),
		Health:    health("wget", "-qO-", "http://127.0.0.1:8080/nginx-health"),
	})
	return out
}

// NginxConf is the nginx configuration of the workspace, or the scenario
// start's for workspaces created before it moved there.
func NginxConf(o Options) string {
	if p := filepath.Join(o.Workspace, "nginx", "default.conf"); fileExists(p) {
		return p
	}
	return filepath.Join(o.Root, "nexora", "nginx", "default.conf")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return !errors.Is(err, os.ErrNotExist)
}

func fileHash(p string) string {
	b, err := os.ReadFile(p) //nolint:gosec // G304: a config file of the workspace or the repository
	if err != nil {
		return "unreadable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// builder compiles the workspace into the bin volume, without network
// (dependencies come from vendor/) and with a Go build cache shared across runs.
// It runs as root, and root without capabilities cannot read a workspace the
// host user made private (0700 dirs, 0600 files): DAC_OVERRIDE restores that,
// limited to what is mounted here. `go build` runs no player code.
func builder(o Options) sandbox.Container {
	return sandbox.Container{
		Component: "builder", Replica: 1, Image: imageGo,
		Entrypoint: []string{"sh", "-c", "cd /src && go build -trimpath -o /out/nexora ./cmd/nexora"},
		Env: map[string]string{
			"CGO_ENABLED": "0", "GOFLAGS": "-mod=vendor", "GOCACHE": "/cache", "HOME": "/tmp", "GOTOOLCHAIN": "local",
		},
		Mounts: []sandbox.Mount{
			wsBind(o, o.Workspace, "/src"),
			{Type: sandbox.VolumeMount, Source: "go-build", Target: "/cache"},
			{Type: sandbox.VolumeMount, Source: "bin", Target: "/out"},
			tmpfs("/tmp"),
		},
		ReadOnly:  true,
		CapAdd:    []string{"DAC_OVERRIDE"},
		Resources: sandbox.Resources{CPUs: 2, Memory: 1024 * mb, Pids: 512},
	}
}

// seeder applies the migrations in dir to database db (and loads the dataset
// when the database is empty). It runs as the host user, who owns the
// workspace, so private file modes do not stop it.
func seeder(o Options, migrations, db string) sandbox.Container {
	return sandbox.Container{
		Component: "seed", Replica: 1, Image: imageSeed, Networks: []string{netPlay}, ReadOnly: true,
		User: fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		Cmd:  []string{"-migrations", "/migrations"},
		Env: map[string]string{
			"NEXORA_SEED_DATABASE_URL": "postgres://nexora:" + o.Secrets.App + "@postgres:5432/" + db + "?sslmode=disable",
		},
		Mounts:    []sandbox.Mount{wsBind(o, migrations, "/migrations")},
		Resources: sandbox.Resources{CPUs: 1, Memory: 256 * mb, Pids: 64},
	}
}

// loadGen runs a k6 workload from the observer zone: it reaches nginx on the
// ingress network and writes its metrics on the telemetry network. It is
// never on the player's network. id tags every metric (testid).
func loadGen(o Options, component, profile string, rate int, duration, ramp time.Duration, id string) sandbox.Container {
	return sandbox.Container{
		Component: component, Replica: 1, Image: imageK6,
		Cmd: []string{"run", "-q", "--no-color", "-o", "experimental-prometheus-rw", "--tag", "testid=" + id,
			"/workloads/nexora/" + profile + ".js"},
		Env: map[string]string{
			"BASE_URL":                       "http://nginx:8080",
			"K6_PROMETHEUS_RW_SERVER_URL":    "http://prometheus:9090/api/v1/write",
			"K6_PROMETHEUS_RW_PUSH_INTERVAL": "2s",
			"K6_FEATURES":                    "native-histograms",
			"RATE":                           strconv.Itoa(rate),
			"DURATION":                       duration.String(),
			"RAMP":                           ramp.String(),
			"RUN_ID":                         id,
		},
		Networks:  []string{netIngress, netTelemetry},
		Mounts:    []sandbox.Mount{bind(filepath.Join(o.Root, "workloads"), "/workloads"), tmpfs("/tmp")},
		ReadOnly:  true,
		Resources: sandbox.Resources{CPUs: 1, Memory: 512 * mb, Pids: 512},
	}
}
