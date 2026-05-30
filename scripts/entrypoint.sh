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

# Run migrations. Any failure aborts the boot — we NEVER auto-drop the
# database on error. A prior version reset the DB in place whenever migrate
# output contained "already exists"; that condition can occur on a perfectly
# healthy production database (e.g. a botched migration creating an object
# that partially exists), so the auto-reset was a total-data-loss footgun.
# To recover a genuinely corrupt fresh DB, an operator sets RESET_DB_ON_BOOT=true
# explicitly (handled above) for a single boot.
echo "[entrypoint] Running database migrations..."
if ! ./migrate up; then
  echo "[entrypoint] Migration failed. Aborting boot (database left untouched)."
  echo "[entrypoint] If and only if this is a known-corrupt FRESH DB, re-deploy once with RESET_DB_ON_BOOT=true."
  exit 1
fi

# Apply the master seed (production-essential, idempotent): super-admin
# user, sell_categories, business_categories, daily_post_limits,
# starter custom_roles. Skipping the admin step alone would leave a
# freshly-initialised database with no admin user and the admin panel
# returns 403. Skip with SEED_MASTER_ON_BOOT=false once defaults have
# been rotated so re-deploys do not silently reset credentials.
#
# Legacy: SEED_ADMIN_ON_BOOT=false still disables the seed for
# operators carrying that flag forward from earlier releases.
if [ "${SEED_MASTER_ON_BOOT:-${SEED_ADMIN_ON_BOOT:-true}}" = "true" ]; then
  echo "[entrypoint] Applying master seed (admin + categories + roles)..."
  ./seed-master || echo "[entrypoint] seed-master returned non-zero (continuing)."
else
  echo "[entrypoint] SEED_MASTER_ON_BOOT=false — skipping master seed."
fi

echo "[entrypoint] Starting server..."
exec ./main
