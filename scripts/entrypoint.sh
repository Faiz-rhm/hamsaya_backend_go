#!/bin/sh
set -e

# Wait for postgres DNS + TCP. Dokploy / compose depends_on with
# condition:service_healthy is unreliable across orchestrators, and the
# container can race the embedded DNS during the first few seconds.
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-hamsaya}"

echo "[entrypoint] Waiting for ${DB_HOST}:${DB_PORT}..."
i=0
until nc -z "${DB_HOST}" "${DB_PORT}" 2>/dev/null; do
  i=$((i + 1))
  if [ "${i}" -ge 60 ]; then
    echo "[entrypoint] Timed out after 60s waiting for ${DB_HOST}:${DB_PORT}"
    exit 1
  fi
  sleep 1
done
echo "[entrypoint] Database reachable after ${i}s."

# Self-heal corrupt DB state (one-shot). When postgres was previously
# booted with ./migrations bind-mounted into /docker-entrypoint-initdb.d,
# initdb ran .up.sql AND .down.sql files in alphabetical order and left a
# partial schema with an empty schema_migrations table. The migrator then
# crash-loops on "relation already exists". Detect that exact state and
# drop+recreate the database so migrate up starts from a clean slate.
#
# Trigger conditions (BOTH must be true to avoid trashing healthy DBs):
#   1. schema_migrations table is empty or missing
#   2. At least one app table (users) already exists
#
# Operator override: set RESET_DB_ON_BOOT=true to force a reset regardless.
echo "[entrypoint] Checking for corrupt initdb state..."
PG="psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -tA"
export PGPASSWORD="${DB_PASSWORD}"

needs_reset=0
if [ "${RESET_DB_ON_BOOT}" = "true" ]; then
  echo "[entrypoint] RESET_DB_ON_BOOT=true — forcing reset."
  needs_reset=1
else
  users_exists=$(${PG} -d "${DB_NAME}" -c "SELECT to_regclass('public.users') IS NOT NULL;" 2>/dev/null || echo "f")
  if [ "${users_exists}" = "t" ]; then
    migrations_count=$(${PG} -d "${DB_NAME}" -c "SELECT COALESCE((SELECT COUNT(*) FROM schema_migrations), 0);" 2>/dev/null || echo "0")
    if [ "${migrations_count}" = "0" ]; then
      echo "[entrypoint] Detected partial schema with empty schema_migrations — auto-reset."
      needs_reset=1
    fi
  fi
fi

if [ "${needs_reset}" = "1" ]; then
  echo "[entrypoint] Dropping and recreating database ${DB_NAME}..."
  ${PG} -d postgres -c "DROP DATABASE IF EXISTS ${DB_NAME} WITH (FORCE);"
  ${PG} -d postgres -c "CREATE DATABASE ${DB_NAME};"
  echo "[entrypoint] Database recreated."
fi
unset PGPASSWORD

echo "[entrypoint] Running database migrations..."
./migrate up

echo "[entrypoint] Starting server..."
exec ./main
