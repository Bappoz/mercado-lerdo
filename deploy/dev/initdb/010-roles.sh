#!/usr/bin/env bash
# Runs once, on an empty data directory, as the postgres superuser.
# Roles: nexora (app + player, owner of the nexora database, not superuser)
#        sesim_observer (read-only monitoring for exporters and the evaluator).
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -v nexora_pw="$NEXORA_DB_PASSWORD" -v observer_pw="$SESIM_OBSERVER_PASSWORD" <<'SQL'
CREATE ROLE nexora LOGIN PASSWORD :'nexora_pw';
CREATE ROLE sesim_observer LOGIN PASSWORD :'observer_pw' IN ROLE pg_monitor;
CREATE DATABASE nexora OWNER nexora;
REVOKE CONNECT ON DATABASE nexora FROM PUBLIC;
GRANT CONNECT ON DATABASE nexora TO nexora, sesim_observer;
SQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname nexora <<'SQL'
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
GRANT USAGE ON SCHEMA public TO sesim_observer;
ALTER DEFAULT PRIVILEGES FOR ROLE nexora IN SCHEMA public GRANT SELECT ON TABLES TO sesim_observer;
SQL
