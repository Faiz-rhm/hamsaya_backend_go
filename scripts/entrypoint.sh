#!/bin/sh
set -e

# Wait for postgres DNS + TCP. Dokploy / compose depends_on with
# condition:service_healthy is unreliable across orchestrators, and the
# container can race the embedded DNS during the first few seconds.
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
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

echo "[entrypoint] Running database migrations..."
./migrate up

echo "[entrypoint] Starting server..."
exec ./main
