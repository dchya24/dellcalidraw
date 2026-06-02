# ops/backup

Postgres backup container for the Dellcalidraw stack.

- Encrypts dumps with `gpg --symmetric --cipher-algo AES256`
- Uploads to S3 (or any S3-compatible endpoint: AWS, MinIO, R2, etc.)
- Prunes objects older than `BACKUP_RETENTION_DAYS`

## Build

```bash
docker compose --profile backup build backup
```

## Run

The service is gated by the `backup` Compose profile — it doesn't start
with the default `docker compose up`. Enable explicitly:

```bash
docker compose --profile backup up -d backup
```

## Manual / one-shot backup

```bash
docker compose --profile backup run --rm backup /usr/local/bin/backup.sh
```

## Restore

See **[../../docs/BACKUPS.md](../../docs/BACKUPS.md)** for the full
runbook (one-shot restore, disaster recovery checklist, drill).

## Files

| File             | Purpose |
| ---------------- | ------- |
| `Dockerfile`     | Alpine + postgres-client + aws-cli + gpg + dcron |
| `entrypoint.sh`  | Installs the crontab and execs cron in the foreground |
| `backup.sh`      | pg_dump → gzip → gpg → S3, then prune by retention |
| `restore.sh`     | S3 → gpg → gunzip → psql (requires `RESTORE_CONFIRM=YES`) |
