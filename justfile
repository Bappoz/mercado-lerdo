# Canonical commands. CI runs the same recipes (`just check`).
set shell := ["bash", "-euo", "pipefail", "-c"]

module := "github.com/Bappoz/software-engineering-simulator"
# Go tools are pinned in tools/*.go.mod and built into bin/tools by `just tools`.
tools_bin := justfile_directory() / "bin/tools"
golangci := tools_bin / "golangci-lint"
dev := "docker compose -f deploy/dev/compose.yml"
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
ldflags := "-s -w -X " + module + "/internal/buildinfo.version=" + version
# ADR-0009 allowlist, shared with web/scripts/check-licenses.mjs
go_licenses := "MIT,ISC,BSD-2-Clause,BSD-3-Clause,Apache-2.0,0BSD,MPL-2.0,EPL-2.0"
platforms := "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

# List recipes
default:
    @just --list

# Install web dependencies and download Go modules
setup:
    go mod download
    npm --prefix web ci

# Build the pinned Go tools into bin/tools (cached by the Go build cache)
tools:
    @go build -modfile=tools/lint/go.mod -o {{tools_bin}}/ github.com/golangci/golangci-lint/v2/cmd/golangci-lint
    @go build -modfile=tools/go.mod -o {{tools_bin}}/ golang.org/x/vuln/cmd/govulncheck github.com/google/go-licenses/v2

# Format Go and web code in place
fmt: tools
    {{golangci}} fmt ./...
    cd nexora && {{golangci}} fmt ./...
    npm --prefix web run format

# Fail if anything is not formatted
fmt-check: tools
    {{golangci}} fmt --diff ./...
    cd nexora && {{golangci}} fmt --diff ./...

# Lint Go (golangci-lint) and web (Biome: lint + format + imports)
lint: tools
    {{golangci}} run ./...
    cd nexora && {{golangci}} run ./...
    npm --prefix web run check

# Fail if any vendored file is git-ignored (CI builds from a clean clone)
vendor-check:
    #!/usr/bin/env bash
    set -euo pipefail
    ignored=$(git ls-files --others --ignored --exclude-standard nexora/vendor)
    if [ -n "$ignored" ]; then echo "git-ignored files inside nexora/vendor:"; echo "$ignored"; exit 1; fi
    cd nexora && go mod verify >/dev/null && go build -mod=vendor ./... && echo "vendor ok"

# Type-check Go and TypeScript
typecheck:
    go vet ./...
    cd nexora && go vet ./...
    npm --prefix web run typecheck

# Run tests (race detector on)
test:
    go test -race ./...
    cd nexora && go test -race ./...

# Build bin/sesim for this machine and the web bundle
build:
    CGO_ENABLED=0 go build -trimpath -ldflags '{{ldflags}}' -o bin/sesim ./cmd/sesim
    npm --prefix web run build

# Cross-compile sesim for every supported platform into dist/
build-all:
    #!/usr/bin/env bash
    set -euo pipefail
    for p in {{platforms}}; do
      os="${p%/*}"; arch="${p#*/}"; ext=""; [ "$os" = windows ] && ext=".exe"
      CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags '{{ldflags}}' \
        -o "dist/sesim-${os}-${arch}${ext}" ./cmd/sesim
      echo "built dist/sesim-${os}-${arch}${ext}"
    done

# Known vulnerabilities in Go code and web production deps
vuln: tools
    {{tools_bin}}/govulncheck ./...
    cd nexora && {{tools_bin}}/govulncheck ./...
    npm --prefix web audit --omit=dev

# Enforce the dependency license policy (ADR-0009)
licenses: tools
    {{tools_bin}}/go-licenses check ./... --ignore {{module}} --allowed_licenses={{go_licenses}}
    cd nexora && {{tools_bin}}/go-licenses check ./... --ignore nexora --allowed_licenses={{go_licenses}}
    npm --prefix web run licenses

# Scan for secrets: git history if there is any, plus the working tree
secrets:
    #!/usr/bin/env bash
    set -euo pipefail
    if git rev-parse --verify -q HEAD >/dev/null; then gitleaks git --no-banner --redact; fi
    gitleaks dir --no-banner --redact .

# Full quality gate: vendor → format → lint → typecheck → test → build → licenses → vuln → secrets
check: vendor-check fmt-check lint typecheck test build licenses vuln secrets

# Report local toolchain and Docker availability
doctor:
    #!/usr/bin/env bash
    set -uo pipefail
    ok=1
    need() { if command -v "$1" >/dev/null; then printf '  ok   %-8s %s\n' "$1" "$($2 2>&1 | head -1)"; else printf '  MISS %s\n' "$1"; ok=0; fi; }
    need go "go version"; need node "node --version"; need npm "npm --version"; need gitleaks "gitleaks version"; need docker "docker --version"
    if command -v docker >/dev/null && ! docker info >/dev/null 2>&1; then echo "  FAIL docker daemon unreachable"; ok=0; fi
    [ "$ok" = 1 ] && echo "doctor: ok" || { echo "doctor: problems found"; exit 1; }

# Refresh nexora/vendor after changing its go.mod
nexora-vendor:
    cd nexora && go mod tidy && go mod vendor

# Start the Nexora dev stack (build, seed, wait until healthy)
dev-up:
    {{dev}} up -d --build --wait

# Stop the dev stack, keeping the database volume
dev-down:
    {{dev}} down

# Stop the dev stack and delete the database (next dev-up reseeds)
dev-reset:
    {{dev}} down -v

# Print the dataset hash of the running dev database
dev-hash:
    {{dev}} run --rm --no-deps seed -migrations /migrations

# Run a workload (normal-day | flash-sale) against the dev stack. Env: RATE, DURATION, RAMP, SEED
# Streams k6 metrics to Prometheus (testid=<run id>) and pushes container stats while it runs.
dev-load profile="flash-sale":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p .runs
    run_id="$(date +%Y%m%dT%H%M%S)-{{profile}}"
    out=".runs/$run_id.json"
    go build -o bin/sesim ./cmd/sesim
    bin/sesim dev container-stats --interval 1s 2> ".runs/$run_id.stats.log" &
    bridge=$!
    trap 'kill "$bridge" 2>/dev/null || true' EXIT
    {{dev}} run --rm --no-deps -T -e RATE -e DURATION -e RAMP -e SEED -e RUN_ID="$run_id" \
      k6 run -q --no-color -o experimental-prometheus-rw --tag testid="$run_id" \
      /workloads/nexora/{{profile}}.js </dev/null > "$out" 2> ".runs/$run_id.log"
    echo "run id: $run_id ($(grep -c 'Request Failed' ".runs/$run_id.log" || true) failed requests logged)"
    bin/sesim load-summary < "$out"

# Push container stats to the dev Prometheus until Ctrl-C
dev-stats:
    go run ./cmd/sesim dev container-stats

# Chunk 3 acceptance checks against the running dev stack
dev-smoke:
    scripts/dev-smoke.sh

# Production Test of a workspace against a scenario (dev stack). Extra args go to `sesim test`.
eval *args:
    go build -o bin/sesim ./cmd/sesim
    bin/sesim test {{args}}

# Scenario harness: broken start + every reference must give the expected verdicts. Extra args: --rounds N --only a,b --runs N
verify *args:
    go build -o bin/sesim ./cmd/sesim
    bin/sesim scenario verify {{args}}

# Chunk 5 acceptance: store isolation + Flash Sale signals in Prometheus
dev-telemetry-check:
    scripts/dev-telemetry-check.sh

# Reseed twice from scratch and compare dataset hashes (slow)
dev-determinism:
    #!/usr/bin/env bash
    set -euo pipefail
    hash() { {{dev}} down -v >/dev/null 2>&1; {{dev}} up -d --wait >/dev/null 2>&1; {{dev}} run --rm --no-deps seed -migrations /migrations 2>/dev/null; }
    a=$(hash); b=$(hash)
    echo "run 1: $a"; echo "run 2: $b"
    [ "$a" = "$b" ] && echo "deterministic: yes" || { echo "deterministic: NO"; exit 1; }

# Open psql as the nexora role inside the sandbox network
dev-psql:
    {{dev}} exec toolbox psql

# Shell in the toolbox container
dev-shell:
    {{dev}} exec toolbox bash

# Follow logs of the dev stack
dev-logs *services:
    {{dev}} logs -f {{services}}

# Remove build output
clean:
    rm -rf bin dist web/dist
