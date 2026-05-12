#!/bin/sh
set -e
set -o pipefail  # so 'migrate | tee' exits non-zero when migrate fails

# Wait for postgres DNS + TCP. Dokploy / compose depends_on with
# condition:service_healthy is unreliable across orchestrators, and the
# container can race the embedded DNS during the first few seconds.
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-hamsaya}"
REDIS_HOST="${REDIS_HOST:-redis}"
REDIS_PORT="${REDIS_PORT:-6379}"

wait_for_tcp() {
  name="$1"; host="$2"; port="$3"; timeout="${4:-90}"
  echo "[entrypoint] Waiting for ${name} (${host}:${port})..."
  i=0
  until nc -z "${host}" "${port}" 2>/dev/null; do
    i=$((i + 1))
    if [ "${i}" -ge "${timeout}" ]; then
      echo "[entrypoint] Timed out after ${timeout}s waiting for ${name} (${host}:${port})"
      exit 1
    fi
    sleep 1
  done
  echo "[entrypoint] ${name} reachable after ${i}s."
}

wait_for_tcp postgres "${DB_HOST}" "${DB_PORT}" 90
wait_for_tcp redis "${REDIS_HOST}" "${REDIS_PORT}" 90

# MinIO is reachable as STORAGE_ENDPOINT=host:port. Wait but don't block.
if [ -n "${STORAGE_ENDPOINT}" ]; then
  storage_host="${STORAGE_ENDPOINT%%:*}"
  storage_port="${STORAGE_ENDPOINT##*:}"
  if [ "${storage_host}" != "${storage_port}" ]; then
    wait_for_tcp minio "${storage_host}" "${storage_port}" 60 || true
  fi
fi

reset_db() {
  echo "[entrypoint] Dropping and recreating database ${DB_NAME}..."
  PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d postgres -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS ${DB_NAME} WITH (FORCE);" \
    -c "CREATE DATABASE ${DB_NAME};"
  echo "[entrypoint] Database recreated."
}

# Operator-forced reset.
if [ "${RESET_DB_ON_BOOT}" = "true" ]; then
  echo "[entrypoint] RESET_DB_ON_BOOT=true — forcing reset before migrations."
  reset_db
fi

# Run migrations. If the run fails because the schema is half-populated
# (the classic corrupted-initdb signature), reset the database in place
# and retry once. Any other failure is real and exits non-zero.
echo "[entrypoint] Running database migrations..."
migrate_log=$(mktemp)
if ! ./migrate up 2>&1 | tee "${migrate_log}"; then
  if grep -q "already exists" "${migrate_log}"; then
    echo "[entrypoint] Migrations failed with 'already exists' — corrupt initdb state detected. Resetting and retrying..."
    reset_db
    ./migrate up
  else
    echo "[entrypoint] Migration failed for a non-recoverable reason. Aborting."
    rm -f "${migrate_log}"
    exit 1
  fi
fi
rm -f "${migrate_log}"

echo "[entrypoint] Starting server..."
exec ./main
