#!/usr/bin/env bash
# Chunk 10a acceptance: a player changes topology.yaml and nginx/default.conf,
# deploys, and the running sandbox follows; a broken nginx config is refused
# without touching production; nothing is left behind.
set -uo pipefail

sesim=bin/sesim
fail=0
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=1; }
check() { local name=$1; shift; if "$@"; then pass "$name"; else bad "$name"; fi; }
tmp=$(mktemp -d)
ws="$tmp/ws"
run=""
cleanup() { [ -n "$run" ] && $sesim stop --down --workspace "$ws" >/dev/null 2>&1; rm -rf "$tmp"; }
trap cleanup EXIT
g() { $sesim "$@" --workspace "$ws" </dev/null; }
count() { docker ps -q --filter "label=dev.sesim.run=$run" --filter "label=dev.sesim.component=$1" | wc -l; }
ctr() { docker inspect -f '{{.Id}}' "sesim-$run-$1-1" 2>/dev/null; }

echo "== start"
g play >/dev/null 2>"$tmp/play.err"; check "play brings the Nexora up" test $? = 0
run=$(jq -r .sandbox "$ws/.sesim/state.json")
check "the workspace has topology.yaml and nginx/default.conf" test -f "$ws/topology.yaml" -a -f "$ws/nginx/default.conf"
check "starts at the default cost (US\$ 70)" bash -c "$sesim topology --workspace $ws | grep -q 'US\\$ 70/mês'"

echo "== install pgbouncer, point the app at it, two replicas"
check "install edits topology.yaml" bash -c "$sesim install pgbouncer --workspace $ws >/dev/null && grep -q '^  pgbouncer:' $ws/topology.yaml"
check "installing twice is refused" bash -c "! $sesim install pgbouncer --workspace $ws >/dev/null 2>&1"
sed -i 's/database: postgres/database: pgbouncer/; s/replicas: 1/replicas: 2/' "$ws/topology.yaml"
g deploy >"$tmp/deploy.out" 2>&1; check "deploy succeeds" test $? = 0
check "two app replicas" test "$(count app)" = 2
check "pgbouncer runs" test "$(count pgbouncer)" = 1
pools=$(docker exec "sesim-$run-toolbox-1" psql -h pgbouncer -p 6432 pgbouncer -Atc 'SHOW POOLS' 2>/dev/null)
check "the app's database traffic goes through pgbouncer" bash -c "grep -q '^nexora|nexora|' <<<'$pools'"
g status >"$tmp/status.out" 2>&1
check "traffic flows without errors after the deploy" bash -c "grep -E '^  (search|checkout) ' $tmp/status.out | awk '{ if (\$3 != \"0.0%\") exit 1 }' && grep -q '^  search ' $tmp/status.out"
check "the next deploy changes nothing" bash -c "$sesim topology --workspace $ws | grep -q 'igual ao workspace'"

echo "== a broken nginx config is refused and production is untouched"
before=$(ctr nginx)
cp "$ws/nginx/default.conf" "$tmp/good.conf"
sed -i 's/proxy_read_timeout 30s;/proxy_read_timeout 30s/' "$ws/nginx/default.conf"
g deploy >"$tmp/bad.out" 2>&1; code=$?
check "deploy fails" test "$code" != 0
check "the error is nginx's own" grep -q 'nginx: \[emerg\]' "$tmp/bad.out"
check "nginx was not recreated" test "$(ctr nginx)" = "$before"
cp "$tmp/good.conf" "$ws/nginx/default.conf"

echo "== back to the default topology"
cp "$(dirname "$0")/../nexora/topology.yaml" "$ws/topology.yaml"
g deploy >/dev/null 2>&1; check "deploy succeeds" test $? = 0
check "one app replica" test "$(count app)" = 1
check "pgbouncer removed" test "$(count pgbouncer)" = 0

echo "== down"
g stop --down >/dev/null 2>&1; check "stop --down succeeds" test $? = 0
left=$(docker ps -aq --filter "label=dev.sesim.run=$run"; docker network ls -q --filter "label=dev.sesim.run=$run"; docker volume ls -q --filter "label=dev.sesim.run=$run")
check "nothing is left behind" test -z "$left"
run=""

[ "$fail" = 0 ] && echo "topology-check: ok" || { echo "topology-check: FAILED"; exit 1; }
