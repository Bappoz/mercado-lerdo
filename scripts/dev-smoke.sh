#!/usr/bin/env bash
# Chunk 3 acceptance checks against a running dev stack (`just dev-up`).
set -euo pipefail

base="http://127.0.0.1:${NEXORA_DEV_PORT:-8080}"
dc=(docker compose -f deploy/dev/compose.yml)
fail=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=1; }
check() { local name=$1; shift; if "$@"; then pass "$name"; else bad "$name"; fi; }
jqok() { jq -e "$@" >/dev/null; } # jq as a predicate; output discarded, exit code kept

echo "== services"
for s in postgres payments app nginx; do
  state=$("${dc[@]}" ps --format '{{.Health}}' "$s" 2>/dev/null || true)
  check "$s healthy ($state)" test "$state" = healthy
done
seed_exit=$("${dc[@]}" ps -a --format '{{.ExitCode}}' seed)
check "seed exited 0" test "$seed_exit" = 0

echo "== api through nginx"
check "GET /healthz" curl -fsS -o /dev/null "$base/healthz"

search=$(curl -fsS "$base/search?q=lamp&limit=20")
n=$(jq '.products | length' <<<"$search")
check "search returns results ($n)" test "$n" -gt 0
check "every result name contains 'lamp' (case-insensitive)" \
  jqok '[.products[].name | ascii_downcase | contains("lamp")] | all' <<<"$search"
check "results ordered by created_at desc" \
  jqok '[.products[].created_at] as $c | $c == ($c | sort | reverse)' <<<"$search"
check "literal % is not a wildcard" \
  jqok '.products | length == 0' <<<"$(curl -fsS "$base/search?q=%25%25")"
check "short query rejected with 400" \
  test "$(curl -s -o /dev/null -w '%{http_code}' "$base/search?q=a")" = 400

echo "== search is slow for the planned reason"
plan=$("${dc[@]}" exec -T toolbox psql -Atc \
  "EXPLAIN SELECT id FROM products WHERE name ILIKE '%lamp%' ORDER BY created_at DESC LIMIT 20" </dev/null)
check "plan uses Seq Scan on products" grep -q 'Seq Scan on products' <<<"$plan"

echo "== checkout end to end"
token=$(curl -fsS "$base/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"user00042@nexora.test","password":"nexora-demo"}' | jq -r .token)
check "seeded user can log in" test -n "$token" -a "$token" != null
ref="smoke-$(date +%s%N)"
code=$(curl -s -o "$tmp/order.json" -w '%{http_code}' "$base/checkout" \
  -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -H "Idempotency-Key: $ref" \
  -d "{\"order_ref\":\"$ref\",\"items\":[{\"product_id\":42,\"quantity\":1}]}")
check "checkout returns 201 ($code)" test "$code" = 201
charges=$("${dc[@]}" exec -T toolbox curl -fsS "http://payments:8090/charges/$ref" </dev/null | jq .count)
check "payment gateway charged exactly once ($charges)" test "$charges" = 1
check "order visible in GET /orders" \
  jqok --arg r "$ref" '[.orders[].order_ref] | index($r) != null' \
  <<<"$(curl -fsS "$base/orders" -H "Authorization: Bearer $token")"

echo "== isolation"
check "sandbox containers cannot reach the internet (internal network)" \
  bash -c "! ${dc[*]} exec -T toolbox curl -fsS --max-time 3 -o /dev/null https://example.com 2>/dev/null"
check "metrics port is not published to the host" \
  bash -c "! curl -fsS --max-time 2 -o /dev/null http://127.0.0.1:9091/metrics 2>/dev/null"

echo; [ "$fail" = 0 ] && echo "all checks passed" || echo "some checks FAILED"
exit "$fail"
