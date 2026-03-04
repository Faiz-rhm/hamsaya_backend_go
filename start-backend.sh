#!/usr/bin/env bash
# Start Docker services (Postgres, Redis, MinIO) then run the backend server.
set -e
cd "$(dirname "$0")"

echo "Starting Docker containers (postgres, redis, minio)..."
docker compose up -d postgres redis minio

echo "Waiting 10s for Postgres to be ready..."
sleep 10

# Free port 8080 if something is using it
if lsof -ti :8080 >/dev/null 2>&1; then
  echo "Stopping process on port 8080..."
  lsof -ti :8080 | xargs kill -9 2>/dev/null || true
  sleep 2
fi

echo "Starting backend server..."
exec go run cmd/server/main.go
