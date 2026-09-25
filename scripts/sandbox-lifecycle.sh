#!/usr/bin/env bash
# Chunk 8 acceptance: the sandbox driver creates, serves, isolates and
# destroys a Nexora sandbox, and GC collects what a killed sesim left behind.
set -uo pipefail

sesim=bin/sesim
fail=0
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=1; }
check() { local name=$1; shift; if "$@"; then pass "$name"; else bad "$name"; fi; }
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
runs() { $sesim sandbox ls | awk 'NR>1 {print $1}'; }
leftovers() { docker ps -aq --filter "label=dev.sesim.run=$1"; docker network ls -q --filter "label=dev.sesim.run=$1"; docker volume ls -q --filter "label=dev.sesim.run=$1"; }

echo "== up"
before=$(runs)
$sesim sandbox up >"$tmp/up.out" 2>"$tmp/up.err"; code=$?
check "sandbox up succeeds" test "$code" = 0
run=$(awk '/^sandbox .* ready$/ {print $2}' "$tmp/up.out")
app=$(awk '/app:/ {print $2}' "$tmp/up.out")
echo "  run=$run app=$app"

echo "== reach and use it"
check "host reaches the app by container IP (no published ports)" curl -fsS -o /dev/null --max-time 5 "$app/healthz"
n=$(curl -fsS "$app/search?q=lamp&limit=20" | jq '.products | length')
check "search answers ($n results)" test "${n:-0}" -gt 0
count=$($sesim sandbox exec "$run" toolbox -- psql -Atc "select count(*) from products" </dev/null)
check "exec psql in the toolbox sees the seeded dataset ($count products)" test "$count" = 100000
check "no container publishes a port" test -z "$(docker ps --filter "label=dev.sesim.run=$run" --format '{{.Ports}}' | grep -E '[0-9]+->' )"

echo "== isolation"
check "toolbox cannot reach the internet" bash -c "! $sesim sandbox exec $run toolbox -- curl -fsS --max-time 3 -o /dev/null https://example.com </dev/null 2>/dev/null"
check "toolbox cannot reach prometheus (telemetry network)" bash -c "! $sesim sandbox exec $run toolbox -- curl -fsS --max-time 3 -o /dev/null http://prometheus:9090/-/ready </dev/null 2>/dev/null"
check "no privileged container, all drop every capability" test -z "$(docker ps -q --filter "label=dev.sesim.run=$run" | xargs docker inspect --format '{{.HostConfig.Privileged}} {{.HostConfig.CapDrop}}' | grep -v '^false \[ALL\]$')"
check "every container has cpu, memory and pids limits" test -z "$(docker ps -q --filter "label=dev.sesim.run=$run" | xargs docker inspect --format '{{.HostConfig.NanoCpus}} {{.HostConfig.Memory}} {{.HostConfig.PidsLimit}}' | grep -E '(^0 )|( 0 )|( 0$)|<nil>')"
check "docker socket is mounted nowhere" test -z "$(docker ps -q --filter "label=dev.sesim.run=$run" | xargs docker inspect --format '{{range .Mounts}}{{.Source}} {{end}}' | grep docker.sock)"

echo "== kill sesim in the middle of up, then gc (while $run is alive)"
$sesim sandbox up >/dev/null 2>&1 &
pid=$!
sleep 8
kill -9 "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
killed=$(comm -13 <( (echo "$before"; echo "$run") | sort) <(runs | sort) | head -1)
check "a half-built sandbox was left behind (${killed:-none})" test -n "$killed"
check "it is listed as orphan" bash -c "$sesim sandbox ls | grep -q '^$killed .*orphan'"
$sesim sandbox gc >"$tmp/gc.out" 2>&1
check "gc collects it" grep -q "$killed" "$tmp/gc.out"
check "no resources left for $killed" test -z "$(leftovers "$killed")"
check "gc spares the live sandbox $run" bash -c "$sesim sandbox ls | grep -q '^$run .*ready'"
check "and it still serves" curl -fsS -o /dev/null --max-time 5 "$app/healthz"

echo "== down"
$sesim sandbox down "$run" >/dev/null 2>"$tmp/down.err"
check "down succeeds" test $? = 0
check "no resources left for $run" test -z "$(leftovers "$run")"

echo; [ "$fail" = 0 ] && echo "all checks passed" || echo "some checks FAILED"
exit "$fail"
