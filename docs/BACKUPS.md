# Backups & Restore Runbook

Postgres backups are encrypted (gpg AES256) and uploaded to the same
S3-compatible bucket the application uses, under `backups/postgres/`.
Object storage versioning + lifecycle should be configured on top of
this for layered protection (see "S3 Object Lifecycle" below).

## Architecture

```
cron (in container)
  └─> pg_dump --format=plain --no-owner --no-privileges
        └─> gzip -9
              └─> gpg --symmetric --cipher-algo AES256
                    └─> aws s3 cp  →  s3://<bucket>/backups/postgres/excalidraw-<db>-<ts>.sql.gz.gpg
                          └─> retention sweep (delete > BACKUP_RETENTION_DAYS old)
```

The backup container is defined as a Compose `backup` service under the
`backup` profile, so it does not start by default. Enable explicitly:

```bash
docker compose --profile backup up -d backup
```

## Required configuration

In the root `.env`:

| Variable                     | Required | Default            | Notes |
| ---------------------------- | :------: | ------------------ | ----- |
| `BACKUP_GPG_PASSPHRASE`      | yes      | —                  | Symmetric encryption passphrase. Generate with `openssl rand -base64 48`. **Store separately from your DB credentials.** Without it, no backup can be restored. |
| `BACKUP_CRON`                | no       | `0 2 * * *`        | Standard crontab. Container runs UTC unless `TZ` overridden. |
| `BACKUP_RETENTION_DAYS`      | no       | `14`               | Older objects under the prefix are pruned each run. |
| `BACKUP_S3_PREFIX`           | no       | `backups/postgres` | Path inside the bucket. |
| `BACKUP_RUN_ON_START`        | no       | `false`            | Set `true` for an immediate sanity-check backup on container boot. |
| `TZ`                         | no       | `UTC`              | Container timezone for cron. |

The backup service reuses the application's database and storage env
vars (`EXCALIDRAW_DATABASE_*`, `EXCALIDRAW_STORAGE_*`).

> 🔒 The dumps include `rooms.encryption_key` (per-room AES-GCM keys for
> WebSocket message encryption). Treat the gpg passphrase with the same
> sensitivity as the database password.

## Verifying a backup ran

```bash
# Container logs
docker logs excalidraw-backup

# List recent backups in the bucket
docker compose run --rm backup \
  aws --endpoint-url "$S3_ENDPOINT" s3 ls "s3://${EXCALIDRAW_STORAGE_BUCKET}/backups/postgres/"
```

A successful run emits a line like:
```
[backup 2026-05-31T02:00:14Z] backup complete: uploaded excalidraw-excalidraw-20260531T020014Z.sql.gz.gpg
```

## Restore

> ⚠️ Restore overwrites the database. Quiesce or stop the backend
> before restoring on the live database. Always test restore on a
> staging copy first.

### One-off restore from the latest backup

```bash
docker compose --profile backup run --rm \
  -e RESTORE_CONFIRM=YES \
  backup \
  /usr/local/bin/restore.sh latest
```

### Restore a specific backup

```bash
# By filename (relative to BACKUP_S3_PREFIX)
docker compose --profile backup run --rm \
  -e RESTORE_CONFIRM=YES \
  backup \
  /usr/local/bin/restore.sh excalidraw-excalidraw-20260531T020014Z.sql.gz.gpg

# By full key under the bucket
docker compose --profile backup run --rm \
  -e RESTORE_CONFIRM=YES \
  backup \
  /usr/local/bin/restore.sh backups/postgres/excalidraw-excalidraw-20260531T020014Z.sql.gz.gpg
```

The `RESTORE_CONFIRM=YES` flag is mandatory. Without it the script
exits non-zero before touching anything.

### Manual restore (ad-hoc machine)

If the backup container is unavailable, restore by hand from any host
with `gpg`, `gunzip`, and `psql`:

```bash
aws --endpoint-url "$S3_ENDPOINT" s3 cp \
  s3://<bucket>/backups/postgres/<file>.sql.gz.gpg /tmp/dump.sql.gz.gpg

gpg --batch --passphrase "$BACKUP_GPG_PASSPHRASE" --decrypt /tmp/dump.sql.gz.gpg \
  | gunzip \
  | psql -h <host> -U <user> -d excalidraw -v ON_ERROR_STOP=1
```

## Disaster recovery checklist

When recovering from total data loss:

1. Stand up a fresh PostgreSQL instance.
2. Set `EXCALIDRAW_DATABASE_*` env to point at the new instance.
3. Run `restore.sh latest` (or a known-good earlier dump if recent
   dumps look corrupt).
4. Bring up backend with `EXCALIDRAW_WEBSOCKET_ENCRYPTION_ENABLED=true`
   — the restored `rooms.encryption_key` column means existing room
   keys are intact and clients can keep using their cached session
   without rejoining.
5. Sanity check: open a room, confirm elements load, confirm
   collaboration still works.

## S3 Object Lifecycle (recommended)

The retention sweep in `backup.sh` deletes by `LastModified` age, but
relies on the backup service running. Pair it with bucket-level
lifecycle for belt-and-braces protection:

- **Versioning on**: a malicious or buggy delete still leaves a
  recoverable previous version.
- **MFA delete** (AWS only): blocks accidental bucket-wide wipes.
- **Lifecycle**: transition objects in `backups/postgres/` to a
  cheaper storage class (e.g. S3 Glacier IR) after 7 days, expire
  after 90.

Configure these on the storage provider's console or via the existing
infrastructure-as-code. They are out of scope for the Compose config.

## Testing the runbook

Quarterly drill (recommended):

1. Spin up a throwaway Postgres on a sandbox host.
2. Point `EXCALIDRAW_DATABASE_*` env at the throwaway.
3. Run `restore.sh latest`.
4. Connect with `psql` and confirm `\dt` lists the expected tables and
   `SELECT count(*) FROM users` returns a sensible number.
5. Tear down the sandbox.

A restore that has never been tested is not a backup.
