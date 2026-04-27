#!/usr/bin/env bash
# Daily Postgres dump → S3 (or any S3-compatible store).
#
# Designed to run inside the `db-backup` sidecar container OR as a host cron
# job. The script:
#   1. Creates a dump with pg_dump --format=custom (compressed + parallel).
#   2. Uploads to S3 under "$S3_PREFIX/$(date +%Y/%m)/dump-$(date +%Y%m%d-%H%M%S).dump".
#   3. Prunes objects older than $RETENTION_DAYS (default 7).
#
# Required env:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE  — Postgres credentials
#   S3_BUCKET                                       — e.g. hamsaya-backups
#   S3_PREFIX                                       — e.g. postgres/prod
#   S3_ENDPOINT (optional)                          — for non-AWS providers
#   AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY        — auth
#   AWS_DEFAULT_REGION (optional)                   — defaults to us-east-1
#
# Optional env:
#   RETENTION_DAYS  — default 7
#
# Exit codes:
#   0  success
#   1  dump failed
#   2  upload failed
#   3  prune failed (non-fatal — already-uploaded backup is intact)

set -euo pipefail

: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGPASSWORD:?PGPASSWORD is required}"
: "${PGDATABASE:?PGDATABASE is required}"
: "${S3_BUCKET:?S3_BUCKET is required}"

PGPORT="${PGPORT:-5432}"
S3_PREFIX="${S3_PREFIX:-postgres}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"

TIMESTAMP="$(date -u +%Y%m%d-%H%M%S)"
MONTH_PATH="$(date -u +%Y/%m)"
DUMP_FILE="/tmp/dump-${TIMESTAMP}.dump"
S3_KEY="${S3_PREFIX}/${MONTH_PATH}/dump-${TIMESTAMP}.dump"

awsArgs=()
if [[ -n "${S3_ENDPOINT:-}" ]]; then
  awsArgs+=(--endpoint-url "${S3_ENDPOINT}")
fi

cleanup() {
  rm -f "${DUMP_FILE}"
}
trap cleanup EXIT

echo "[$(date -Iseconds)] Dumping ${PGDATABASE} from ${PGHOST}:${PGPORT}..."
if ! PGPASSWORD="${PGPASSWORD}" pg_dump \
  --host="${PGHOST}" \
  --port="${PGPORT}" \
  --username="${PGUSER}" \
  --dbname="${PGDATABASE}" \
  --format=custom \
  --compress=9 \
  --no-owner \
  --no-privileges \
  --file="${DUMP_FILE}"; then
  echo "[$(date -Iseconds)] ERROR: pg_dump failed" >&2
  exit 1
fi

DUMP_SIZE=$(wc -c < "${DUMP_FILE}" | tr -d ' ')
echo "[$(date -Iseconds)] Dump complete (${DUMP_SIZE} bytes), uploading to s3://${S3_BUCKET}/${S3_KEY}"

if ! aws s3 cp "${awsArgs[@]}" "${DUMP_FILE}" "s3://${S3_BUCKET}/${S3_KEY}" --no-progress; then
  echo "[$(date -Iseconds)] ERROR: s3 upload failed" >&2
  exit 2
fi

echo "[$(date -Iseconds)] Upload complete. Pruning objects older than ${RETENTION_DAYS} days..."

# Prune. We list and delete in two steps so a transient failure of one
# doesn't leave the bucket inconsistent.
CUTOFF_TS=$(date -u -v "-${RETENTION_DAYS}d" +%s 2>/dev/null \
  || date -u -d "${RETENTION_DAYS} days ago" +%s)

PRUNED=0
while IFS= read -r line; do
  # Lines look like:  2026-04-20 13:37:42  234567 postgres/prod/2026/04/dump-...dump
  obj_date=$(echo "$line" | awk '{print $1, $2}')
  obj_key=$(echo "$line" | awk '{for(i=4;i<=NF;i++) printf "%s%s", $i, (i<NF?" ":"")}')
  obj_ts=$(date -u -j -f "%Y-%m-%d %H:%M:%S" "${obj_date}" +%s 2>/dev/null \
    || date -u -d "${obj_date}" +%s)
  if [[ "${obj_ts}" -lt "${CUTOFF_TS}" ]]; then
    echo "  pruning ${obj_key} (age $(( (CUTOFF_TS - obj_ts) / 86400 ))d past cutoff)"
    if aws s3 rm "${awsArgs[@]}" "s3://${S3_BUCKET}/${obj_key}" --no-progress >/dev/null; then
      PRUNED=$((PRUNED + 1))
    fi
  fi
done < <(aws s3 ls "${awsArgs[@]}" "s3://${S3_BUCKET}/${S3_PREFIX}/" --recursive 2>/dev/null || true)

echo "[$(date -Iseconds)] Pruned ${PRUNED} object(s). Backup complete."
