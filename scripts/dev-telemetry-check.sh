#!/usr/bin/env bash
# Chunk 5 acceptance: network isolation of the metrics store, and the four
# signals of a Flash Sale visible in Prometheus over the same time window.
set -euo pipefail

prom="http://127.0.0.1:${PROMETHEUS_DEV_PORT:-9090}"
dc=(docker compose -f deploy/dev/compose.yml)
fail=0
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=1; }
check() { local name=$1; shift; if "$@"; then pass "$name"; else bad "$name"; fi; }
# query <promql> [time] -> first sample value, or "none"
query() {
  curl -fsS "$prom/api/v1/query" --data-urlencode "query=$1" ${2:+--data-urlencode "time=$2"} |
    jq -r '.data.result[0].value[1] // "none"'
}
unreachable() { # service, url
  ! "${dc[@]}" exec -T "$1" wget -q -T 3 -O /dev/null "$2" </dev/null 2>/dev/null
}
gt() { awk -v a="$1" -v b="$2" 'BEGIN { exit !(a != "none" && a+0 > b+0) }'; }

echo "== isolation of the metrics store"
check "toolbox (play) cannot reach prometheus" unreachable toolbox http://prometheus:9090/-/ready
check "nginx (player config) cannot reach prometheus" unreachable nginx http://prometheus:9090/-/ready
check "app cannot reach prometheus" unreachable app http://prometheus:9090/-/ready
check "agent (play+telemetry) can reach prometheus" bash -c "! $(declare -f unreachable); dc=(${dc[*]}); unreachable agent http://prometheus:9090/-/ready"

echo "== flash sale with telemetry"
start=$(date +%s)
DURATION=${DURATION:-30s} RAMP=${RAMP:-10s} just dev-load flash-sale | grep -E '^run id|^search'
end=$(date +%s)
sleep 5 # let the last remote-write/OTLP batches land
w="$(( end - start ))s"
testid=$(ls -t .runs/*-flash-sale.json | head -1 | xargs basename | sed 's/\.json$//')

p95=$(query "histogram_quantile(0.95, sum(rate(k6_http_req_duration_seconds{testid=\"$testid\",name=\"search\"}[$w])))" "$end")
cpu=$(query "max_over_time((rate(sesim_container_cpu_usage_seconds_total{container=\"postgres\"}[10s]) / on(container) sesim_container_cpu_quota_cores{container=\"postgres\"})[$w:2s])" "$end")
thr=$(query "increase(sesim_container_cpu_throttled_periods_total{container=\"postgres\"}[$w]) / increase(sesim_container_cpu_periods_total{container=\"postgres\"}[$w])" "$end")
wait=$(query "sum(increase(nexora_db_pool_empty_acquire_wait_seconds_total[$w]))" "$end")
top=$(curl -fsS "$prom/api/v1/query" --data-urlencode "time=$end" \
  --data-urlencode "query=topk(1, increase(pg_stat_statements_seconds_total[$w])) + on(queryid) group_left(query) (0 * pg_stat_statements_query_id)" |
  jq -r '.data.result[0].metric.query // "none"')

echo "== signals over the last $w (testid $testid)"
printf '  %-44s %s\n' "client search p95 (s, k6)" "$p95" \
  "postgres CPU / quota (peak, host cgroups)" "$cpu" \
  "postgres throttled periods ratio" "$thr" \
  "app pool: time waiting for a connection (s)" "$wait" \
  "top query by DB time (pg_stat_statements)" "$(tr -s "[:space:]" " " <<<"${top:0:100}")"
check "client-side search p95 above 1 s" gt "$p95" 1 # k6 remote write reports seconds
check "postgres CPU pinned at its quota (>= 90%)" gt "$cpu" 0.9
check "app waited for pool connections" gt "$wait" 0
check "search query is the top DB consumer" grep -qi 'ILIKE' <<<"$top"

echo; [ "$fail" = 0 ] && echo "all checks passed" || echo "some checks FAILED"
exit "$fail"
