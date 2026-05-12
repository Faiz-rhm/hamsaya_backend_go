#!/bin/sh
set -e

echo "[entrypoint] Running database migrations..."
./migrate up

echo "[entrypoint] Starting server..."
exec ./main
