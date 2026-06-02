# Postgres backup container
#
# Runs a nightly pg_dump, encrypts it with gpg, and uploads to the same
# S3-compatible bucket the application uses. Old backups are pruned by
# retention window. See ops/backup/README.md and docs/BACKUPS.md.
FROM alpine:3.20

RUN apk add --no-cache \
    bash \
    coreutils \
    findutils \
    gnupg \
    gzip \
    postgresql16-client \
    aws-cli \
    tzdata \
    dcron

# Backup scripts
COPY backup.sh   /usr/local/bin/backup.sh
COPY restore.sh  /usr/local/bin/restore.sh
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/backup.sh /usr/local/bin/restore.sh /usr/local/bin/entrypoint.sh

# Default schedule: daily at 02:00 UTC. Override with BACKUP_CRON env.
ENV BACKUP_CRON="0 2 * * *"

# Backup retention in days (default 14)
ENV BACKUP_RETENTION_DAYS=14

# S3 prefix under the bucket
ENV BACKUP_S3_PREFIX=backups/postgres

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
