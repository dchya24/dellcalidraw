#!/usr/bin/env bash
# Container entrypoint: install crontab, run a backup immediately if
# RUN_ON_START=true (useful for sanity check), then exec cron in
# foreground so docker keeps the container alive.
set -euo pipefail

CRON_SCHEDULE="${BACKUP_CRON:-0 2 * * *}"

# Capture the current process environment for the cron job. cron strips
# almost everything by default; writing it into /etc/environment + the
# crontab itself keeps things portable.
env_file="/var/log/cron-env"
{
  printenv | sed 's/^\([A-Za-z_][A-Za-z0-9_]*\)=\(.*\)$/export \1=\x27\2\x27/'
} > "$env_file"

cron_log="/var/log/cron.log"
touch "$cron_log"

cat > /etc/crontabs/root <<EOF
${CRON_SCHEDULE} . ${env_file} && /usr/local/bin/backup.sh >> ${cron_log} 2>&1
EOF

echo "[entrypoint $(date -u +%FT%TZ)] cron schedule: ${CRON_SCHEDULE}"
echo "[entrypoint $(date -u +%FT%TZ)] retention: ${BACKUP_RETENTION_DAYS} days"
echo "[entrypoint $(date -u +%FT%TZ)] s3 prefix: ${BACKUP_S3_PREFIX}"

if [[ "${RUN_ON_START:-false}" == "true" ]]; then
  echo "[entrypoint $(date -u +%FT%TZ)] RUN_ON_START=true, running initial backup"
  /usr/local/bin/backup.sh || echo "[entrypoint] initial backup failed (continuing)"
fi

# Tail the cron log in the background so docker logs sees backup output
tail -F "$cron_log" &

exec crond -f -l 8
