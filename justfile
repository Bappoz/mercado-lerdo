# Canonical commands. CI runs the same recipes (`just check`).
set shell := ["bash", "-euo", "pipefail", "-c"]

module := "github.com/Bappoz/software-engineering-simulator"
golangci := "go tool -modfile=tools/lint/go.mod golangci-lint"
gotool := "go tool -modfile=tools/go.mod"
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

# Format Go and web code in place
fmt:
    {{golangci}} fmt ./...
    npm --prefix web run format

# Fail if anything is not formatted
fmt-check:
    {{golangci}} fmt --diff ./...

# Lint Go (golangci-lint) and web (Biome: lint + format + imports)
lint:
    {{golangci}} run ./...
    npm --prefix web run check

# Type-check Go and TypeScript
typecheck:
    go vet ./...
    npm --prefix web run typecheck

# Run tests (race detector on)
test:
    go test -race ./...

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
vuln:
    {{gotool}} govulncheck ./...
    npm --prefix web audit --omit=dev

# Enforce the dependency license policy (ADR-0009)
licenses:
    {{gotool}} go-licenses check ./... --ignore {{module}} --allowed_licenses={{go_licenses}}
    npm --prefix web run licenses

# Scan for secrets: git history if there is any, plus the working tree
secrets:
    #!/usr/bin/env bash
    set -euo pipefail
    if git rev-parse --verify -q HEAD >/dev/null; then gitleaks git --no-banner --redact; fi
    gitleaks dir --no-banner --redact .

# Full quality gate: format → lint → typecheck → test → build → licenses → vuln → secrets
check: fmt-check lint typecheck test build licenses vuln secrets

# Report local toolchain and Docker availability
doctor:
    #!/usr/bin/env bash
    set -uo pipefail
    ok=1
    need() { if command -v "$1" >/dev/null; then printf '  ok   %-8s %s\n' "$1" "$($2 2>&1 | head -1)"; else printf '  MISS %s\n' "$1"; ok=0; fi; }
    need go "go version"; need node "node --version"; need npm "npm --version"; need gitleaks "gitleaks version"; need docker "docker --version"
    if command -v docker >/dev/null && ! docker info >/dev/null 2>&1; then echo "  FAIL docker daemon unreachable"; ok=0; fi
    [ "$ok" = 1 ] && echo "doctor: ok" || { echo "doctor: problems found"; exit 1; }

# Remove build output
clean:
    rm -rf bin dist web/dist
