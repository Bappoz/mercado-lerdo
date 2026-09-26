#!/bin/sh
# Writes pgbouncer's config from the environment and starts it in the foreground.
set -eu
: "${PGB_DB_HOST:?}" "${PGB_USER:?}" "${PGB_PASSWORD:?}"
dir=/tmp/pgbouncer
mkdir -p "$dir"
umask 077
# The password is plain text in the userlist so PgBouncer can also log in to
# Postgres with SCRAM on the client's behalf.
printf '"%s" "%s"\n' "$PGB_USER" "$PGB_PASSWORD" > "$dir/userlist.txt"
cat > "$dir/pgbouncer.ini" <<INI
[databases]
* = host=$PGB_DB_HOST port=5432

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
unix_socket_dir =
auth_type = scram-sha-256
auth_file = $dir/userlist.txt
pool_mode = ${PGB_POOL_MODE:-transaction}
default_pool_size = ${PGB_DEFAULT_POOL_SIZE:-20}
max_client_conn = ${PGB_MAX_CLIENT_CONN:-500}
; protocol-level prepared statements (pgx uses them) survive transaction pooling
max_prepared_statements = 200
; the player may inspect the pooler: psql -h pgbouncer -p 6432 pgbouncer -c 'SHOW POOLS'
stats_users = $PGB_USER
ignore_startup_parameters = extra_float_digits
logfile =
log_connections = 0
log_disconnections = 0
INI
exec pgbouncer "$dir/pgbouncer.ini"
