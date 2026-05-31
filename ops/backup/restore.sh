#!/usr/bin/env bash
# Restore an encrypted Postgres backup from S3.
#
# Usage:
#   restore.sh <s3-key>          # restore specific backup by S3 key (relative to prefix or absolute)
#   restore.sh latest            # restore the most recent backup under BACKUP_S3_PREFIX
#
# Required env: same DB and S3 vars as backup.sh, plus BACKUP_GPG_PASSPHRASE.
#
# Safety:
#   - Will REFUSE to run unless RESTORE_CONFIRM=YES is set in env. This
#     prevents accidental restores wiping the running database.
#   - Pipes psql -v ON_ERROR_STOP=1 so the restore aborts on the first error.
#   - Restores into the database named in EXCALIDRAW_DATABASE_DBNAME.

set -euo pipefail

log() { echo "[restore $(date -u +%FT%TZ)] $*"; }
fail() { echo "[restore $(date -u +%FT%TZ)] ERROR: $*" >&2; exit 1; }

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

if [[ "${RESTORE_CONFIRM:-}" != "YES" ]]; then
  fail "set RESTORE_CONFIRM=YES to acknowledge that this will overwrite '${EXCALIDRAW_DATABASE_DBNAME}'"
fi

if [[ $# -lt 1 ]]; then
  fail "usage: restore.sh <s3-key|latest>"
fi
TARGET="$1"

DB_PORT="${EXCALIDRAW_DATABASE_PORT:-5432}"
DB_SSLMODE="${EXCALIDRAW_DATABASE_SSLMODE:-require}"
S3_PREFIX="${BACKUP_S3_PREFIX:-backups/postgres}"
USE_SSL="${EXCALIDRAW_STORAGE_USE_SSL:-true}"
REGION="${EXCALIDRAW_STORAGE_REGION:-us-east-1}"

if [[ "$USE_SSL" == "true" ]]; then
  S3_ENDPOINT="https://${EXCALIDRAW_STORAGE_ENDPOINT}"
else
  S3_ENDPOINT="http://${EXCALIDRAW_STORAGE_ENDPOINT}"
fi

export AWS_ACCESS_KEY_ID="$EXCALIDRAW_STORAGE_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$EXCALIDRAW_STORAGE_SECRET_KEY"
export AWS_DEFAULT_REGION="$REGION"

# Resolve target key
if [[ "$TARGET" == "latest" ]]; then
  KEY="$(aws --endpoint-url "$S3_ENDPOINT" s3api list-objects-v2 \
    --bucket "$EXCALIDRAW_STORAGE_BUCKET" \
    --prefix "${S3_PREFIX}/" \
    --query 'sort_by(Contents, &LastModified)[-1].Key' \
    --output text 2>/dev/null)"
  if [[ -z "$KEY" || "$KEY" == "None" ]]; then
    fail "no backups found under s3://${EXCALIDRAW_STORAGE_BUCKET}/${S3_PREFIX}/"
  fi
elif [[ "$TARGET" == s3://* ]]; then
  fail "pass the key relative to the bucket, not a full s3:// URI"
elif [[ "$TARGET" == *"/"* ]]; then
  KEY="$TARGET"
else
  KEY="${S3_PREFIX}/${TARGET}"
fi

log "target s3://${EXCALIDRAW_STORAGE_BUCKET}/${KEY}"

TMP_DIR="$(mktemp -d /tmp/pgrestore.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

ENCRYPTED="${TMP_DIR}/dump.sql.gz.gpg"
aws --endpoint-url "$S3_ENDPOINT" s3 cp "s3://${EXCALIDRAW_STORAGE_BUCKET}/${KEY}" "$ENCRYPTED"
log "downloaded $(stat -c %s "$ENCRYPTED") bytes"

export PGHOST="$EXCALIDRAW_DATABASE_HOST"
export PGPORT="$DB_PORT"
export PGUSER="$EXCALIDRAW_DATABASE_USER"
export PGPASSWORD="$EXCALIDRAW_DATABASE_PASSWORD"
export PGSSLMODE="$DB_SSLMODE"

log "restoring into ${EXCALIDRAW_DATABASE_DBNAME}"
gpg --batch --yes --passphrase "$BACKUP_GPG_PASSPHRASE" --decrypt "$ENCRYPTED" \
  | gunzip \
  | psql -v ON_ERROR_STOP=1 -d "$EXCALIDRAW_DATABASE_DBNAME"

log "restore complete"
