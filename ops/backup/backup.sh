#!/usr/bin/env bash
# Postgres → gzip → gpg → S3 backup. Designed to be idempotent and to
# fail loudly: missing required env, pg_dump errors, gpg errors, and
# upload errors all exit non-zero so the cron container restarts and
# the next monitoring tick catches the gap.
#
# Required env:
#   EXCALIDRAW_DATABASE_HOST
#   EXCALIDRAW_DATABASE_PORT       (default 5432)
#   EXCALIDRAW_DATABASE_USER
#   EXCALIDRAW_DATABASE_PASSWORD
#   EXCALIDRAW_DATABASE_DBNAME
#   EXCALIDRAW_DATABASE_SSLMODE    (default require)
#   EXCALIDRAW_STORAGE_ENDPOINT    (e.g. s3.us-east-1.amazonaws.com or minio:9000)
#   EXCALIDRAW_STORAGE_ACCESS_KEY
#   EXCALIDRAW_STORAGE_SECRET_KEY
#   EXCALIDRAW_STORAGE_BUCKET
#   EXCALIDRAW_STORAGE_REGION      (default us-east-1)
#   EXCALIDRAW_STORAGE_USE_SSL     (true|false, default true)
#   BACKUP_GPG_PASSPHRASE          required — symmetric encryption passphrase
#
# Optional env:
#   BACKUP_S3_PREFIX               default "backups/postgres"
#   BACKUP_RETENTION_DAYS          default 14

set -euo pipefail

log() { echo "[backup $(date -u +%FT%TZ)] $*"; }
fail() { echo "[backup $(date -u +%FT%TZ)] ERROR: $*" >&2; exit 1; }

require() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    fail "missing required env: $name"
  fi
}

require EXCALIDRAW_DATABASE_HOST
require EXCALIDRAW_DATABASE_USER
require EXCALIDRAW_DATABASE_PASSWORD
require EXCALIDRAW_DATABASE_DBNAME
require EXCALIDRAW_STORAGE_ENDPOINT
require EXCALIDRAW_STORAGE_ACCESS_KEY
require EXCALIDRAW_STORAGE_SECRET_KEY
require EXCALIDRAW_STORAGE_BUCKET
require BACKUP_GPG_PASSPHRASE

DB_PORT="${EXCALIDRAW_DATABASE_PORT:-5432}"
DB_SSLMODE="${EXCALIDRAW_DATABASE_SSLMODE:-require}"
S3_PREFIX="${BACKUP_S3_PREFIX:-backups/postgres}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
USE_SSL="${EXCALIDRAW_STORAGE_USE_SSL:-true}"
REGION="${EXCALIDRAW_STORAGE_REGION:-us-east-1}"

# Configure S3 endpoint for AWS CLI. Endpoint defaults to https://;
# allow plaintext for local MinIO when USE_SSL=false.
if [[ "$USE_SSL" == "true" ]]; then
  S3_ENDPOINT="https://${EXCALIDRAW_STORAGE_ENDPOINT}"
else
  S3_ENDPOINT="http://${EXCALIDRAW_STORAGE_ENDPOINT}"
fi

export AWS_ACCESS_KEY_ID="$EXCALIDRAW_STORAGE_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$EXCALIDRAW_STORAGE_SECRET_KEY"
export AWS_DEFAULT_REGION="$REGION"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
DUMP_NAME="excalidraw-${EXCALIDRAW_DATABASE_DBNAME}-${TS}.sql.gz.gpg"
TMP_DIR="$(mktemp -d /tmp/pgbackup.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

DUMP_PATH="${TMP_DIR}/${DUMP_NAME}"

log "starting backup: db=${EXCALIDRAW_DATABASE_DBNAME} host=${EXCALIDRAW_DATABASE_HOST} bucket=${EXCALIDRAW_STORAGE_BUCKET}"

# pg_dump uses libpq env vars
export PGHOST="$EXCALIDRAW_DATABASE_HOST"
export PGPORT="$DB_PORT"
export PGUSER="$EXCALIDRAW_DATABASE_USER"
export PGPASSWORD="$EXCALIDRAW_DATABASE_PASSWORD"
export PGSSLMODE="$DB_SSLMODE"

# pg_dump -Fc would be faster to restore but harder to inspect. Plain
# format keeps the runbook simple and lets ops grep through dumps.
pg_dump --format=plain --no-owner --no-privileges "$EXCALIDRAW_DATABASE_DBNAME" \
  | gzip -9 \
  | gpg --batch --yes --passphrase "$BACKUP_GPG_PASSPHRASE" --symmetric --cipher-algo AES256 --output "$DUMP_PATH"

DUMP_SIZE="$(stat -c %s "$DUMP_PATH")"
log "dump created: ${DUMP_PATH} (${DUMP_SIZE} bytes)"

if [[ "$DUMP_SIZE" -lt 1024 ]]; then
  fail "dump suspiciously small (<1KB): ${DUMP_SIZE} bytes"
fi

S3_KEY="${S3_PREFIX}/${DUMP_NAME}"
S3_URI="s3://${EXCALIDRAW_STORAGE_BUCKET}/${S3_KEY}"

aws --endpoint-url "$S3_ENDPOINT" s3 cp "$DUMP_PATH" "$S3_URI"
log "uploaded ${S3_URI}"

# Retention: prune objects older than RETENTION_DAYS under the prefix.
# We list, parse LastModified, and delete what falls outside the window.
CUTOFF_EPOCH="$(date -u -d "${RETENTION_DAYS} days ago" +%s 2>/dev/null || \
  date -u -v"-${RETENTION_DAYS}d" +%s)"
log "pruning backups older than ${RETENTION_DAYS} days (cutoff epoch ${CUTOFF_EPOCH})"

LIST_OUTPUT="$(aws --endpoint-url "$S3_ENDPOINT" s3api list-objects-v2 \
  --bucket "$EXCALIDRAW_STORAGE_BUCKET" \
  --prefix "${S3_PREFIX}/" \
  --query 'Contents[].[Key,LastModified]' \
  --output text || true)"

if [[ -z "$LIST_OUTPUT" || "$LIST_OUTPUT" == "None" ]]; then
  log "no existing backups to consider for pruning"
else
  while read -r KEY LAST_MODIFIED; do
    [[ -z "$KEY" || "$KEY" == "None" ]] && continue
    OBJ_EPOCH="$(date -u -d "$LAST_MODIFIED" +%s 2>/dev/null || echo 0)"
    if (( OBJ_EPOCH > 0 && OBJ_EPOCH < CUTOFF_EPOCH )); then
      aws --endpoint-url "$S3_ENDPOINT" s3 rm "s3://${EXCALIDRAW_STORAGE_BUCKET}/${KEY}" >/dev/null
      log "pruned s3://${EXCALIDRAW_STORAGE_BUCKET}/${KEY}"
    fi
  done <<< "$LIST_OUTPUT"
fi

log "backup complete: uploaded ${DUMP_NAME}"
