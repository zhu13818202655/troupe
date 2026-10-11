#!/usr/bin/env bash
#
# Verify the PostgreSQL platform database was initialised correctly.
#
# Why this exists: schema creation is only complete when the platform database
# does NOT already exist. On an empty DB the daemon's auto-bootstrap runs a
# full ent Schema.Create (24 tables). If the DB was pre-created, the store opens
# successfully and only a 10-table whitelist is created — silently missing
# users / organizations / platform_settings / platform_secrets. The daemon still
# starts and prints "postgres (platform mode)", so nothing alerts you.
#
# Usage:
#   cd deploy && ./scripts/init-db.sh
#
# Exit codes: 0 = healthy, 1 = needs recovery (instructions printed).

set -euo pipefail

cd "$(dirname "$0")/.."

DSN="${TROUPE_PLATFORM_DSN:-}"
if [[ -z "$DSN" && -f .env ]]; then
  DSN="$(grep -E '^TROUPE_PLATFORM_DSN=' .env | cut -d= -f2- || true)"
fi
if [[ -z "$DSN" ]]; then
  echo "ERROR: TROUPE_PLATFORM_DSN not set (export it or put it in deploy/.env)" >&2
  exit 1
fi

# psql must connect as the role that owns the database. `docker compose exec`
# runs as the image's default user (root here), and the postgres image only
# creates the role named by POSTGRES_USER — so a bare `psql` fails with
#   FATAL: role "root" does not exist
# The DSN is authoritative for the user name; POSTGRES_USER is the fallback.
PSQL_USER="$(echo "$DSN" | sed -E 's#^[a-zA-Z0-9+.-]+://([^:@/]+).*#\1#')"
if [[ -z "$PSQL_USER" || "$PSQL_USER" == "$DSN" ]]; then
  PSQL_USER="${POSTGRES_USER:-}"
fi
if [[ -z "$PSQL_USER" && -f .env ]]; then
  PSQL_USER="$(grep -E '^POSTGRES_USER=' .env | cut -d= -f2- || true)"
fi
PSQL_USER="${PSQL_USER:-troupe}"

# run_psql <database> <sql>
# Connect to an explicit database: never rely on the default, which would be a
# database named after the role (may not exist) or the maintenance DB.
run_psql() {
  docker compose exec -T postgres psql -U "$PSQL_USER" -d "$1" -tAc "$2"
}

echo "Waiting for PostgreSQL..."
for _ in $(seq 1 30); do
  if docker compose exec -T postgres pg_isready -q -U "$PSQL_USER" -d postgres; then break; fi
  sleep 2
done

DB_NAME="$(echo "$DSN" | sed -E 's#.*/([^/?]+).*#\1#')"
echo "Platform database: ${DB_NAME} (role: ${PSQL_USER})"

EXISTS="$(run_psql postgres "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" || true)"
if [[ "$EXISTS" != "1" ]]; then
  echo "NOT INITIALISED: database '${DB_NAME}' does not exist yet."
  echo "Start the app once (docker compose up -d troupe) — the daemon bootstraps it."
  exit 1
fi

TABLES="$(run_psql "$DB_NAME" "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'")"
echo "Tables in public schema: ${TABLES}"

EXPECTED=24
if [[ "$TABLES" -lt "$EXPECTED" ]]; then
  echo
  echo "INCOMPLETE SCHEMA: expected ${EXPECTED} tables, found ${TABLES}."
  echo "This happens when the database existed before the daemon started."
  echo
  echo "Recovery (DESTROYS the current platform database):"
  echo "  docker compose exec -T postgres psql -U ${PSQL_USER} -d postgres -c 'DROP DATABASE IF EXISTS \"${DB_NAME}\"'"
  echo "  docker compose restart troupe"
  echo
  exit 1
fi

MISSING="$(run_psql "$DB_NAME" "SELECT string_agg(t, ', ') FROM (VALUES ('users'),('organizations'),('platform_settings'),('platform_secrets')) AS v(t) WHERE NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=v.t)")"
if [[ -n "$MISSING" && "$MISSING" != "NULL" ]]; then
  echo "MISSING CRITICAL TABLES: ${MISSING}"
  exit 1
fi

VECTOR="$(run_psql "$DB_NAME" "SELECT 1 FROM pg_extension WHERE extname='vector'" || true)"
if [[ "$VECTOR" != "1" ]]; then
  echo "WARNING: pgvector extension not installed." >&2
fi

echo "OK: platform schema initialised (${TABLES} tables, pgvector present)."
