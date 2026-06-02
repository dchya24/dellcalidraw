# GitHub Secrets Configuration

This document lists all GitHub Secrets required for the CD pipeline to deploy to VPS.

## Setup Location

Add these secrets in your GitHub repository:
**Settings → Secrets and variables → Actions → Repository secrets**

## Required Secrets

### VPS Connection (Required)

| Secret Name | Description | Example |
|-------------|-------------|---------|
| `VPS_HOST` | Hostname or IP address of your VPS | `203.0.113.42` or `vps.example.com` |
| `VPS_USER` | SSH user for deployment | `deploy` |
| `VPS_SSH_KEY` | Private SSH key (PEM format) for the deploy user | `-----BEGIN OPENSSH PRIVATE KEY-----...` |
| `VPS_PORT` | SSH port | `22` |
| `VPS_DEPLOY_PATH` | Absolute path on VPS containing docker-compose.yml | `/srv/dellcalidraw` |
| `GHCR_PULL_TOKEN` | GitHub Personal Access Token with `read:packages` scope (only for private repos) | `ghp_xxxxxxxxxxxxx` |

### Application Configuration (Required)

| Secret Name | Description | Example | Default |
|-------------|-------------|---------|---------|
| `BACKEND_IMAGE` | Backend Docker image | `ghcr.io/your-org/dellcalidraw/excalidraw-be:latest` | - |
| `FRONTEND_IMAGE` | Frontend Docker image | `ghcr.io/your-org/dellcalidraw/excalidraw-fe:latest` | - |
| `BACKEND_PORT` | Host port for backend | `8080` | `8080` |
| `FRONTEND_PORT` | Host port for frontend | `3000` | `3000` |
| `APP_ENV` | Application environment | `production` | `production` |

### Database Configuration (Required)

| Secret Name | Description | Example | Default |
|-------------|-------------|---------|---------|
| `DB_HOST` | PostgreSQL host | `db.internal` or `host.docker.internal` | - |
| `DB_PORT` | PostgreSQL port | `5432` | `5432` |
| `DB_USER` | Database username | `excalidraw` | - |
| `DB_PASSWORD` | Database password | `your-secure-password` | - |
| `DB_NAME` | Database name | `excalidraw` | `excalidraw` |
| `DB_SSLMODE` | SSL mode for database connection | `require` or `disable` | `require` |

### Object Storage Configuration (Required)

| Secret Name | Description | Example | Default |
|-------------|-------------|---------|---------|
| `STORAGE_ENDPOINT` | S3-compatible storage endpoint | `s3.us-east-1.amazonaws.com` or `minio.internal:9000` | - |
| `STORAGE_ACCESS_KEY` | Storage access key | `AKIAIOSFODNN7EXAMPLE` | - |
| `STORAGE_SECRET_KEY` | Storage secret key | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` | - |
| `STORAGE_BUCKET` | Storage bucket name | `excalidraw-files` | `excalidraw-files` |
| `STORAGE_REGION` | Storage region | `us-east-1` | `us-east-1` |
| `STORAGE_USE_SSL` | Use SSL for storage connection | `true` or `false` | `true` |

### WebSocket Configuration (Optional)

| Secret Name | Description | Example | Default |
|-------------|-------------|---------|---------|
| `WEBSOCKET_ENCRYPTION_ENABLED` | Enable per-room message encryption | `true` or `false` | `true` |

### Email Configuration (Optional, but recommended for production)

| Secret Name | Description | Example | Default |
|-------------|-------------|---------|---------|
| `EMAIL_PROVIDER` | Email provider type | `smtp` or `log` | `log` |
| `EMAIL_FROM` | Sender email address | `noreply@example.com` | - |
| `EMAIL_BASE_URL` | Base URL for email links | `https://your-app.example.com` | `http://localhost:3000` |
| `EMAIL_SMTP_HOST` | SMTP server hostname | `smtp.gmail.com` | - |
| `EMAIL_SMTP_PORT` | SMTP server port | `587` | `587` |
| `EMAIL_SMTP_USERNAME` | SMTP username | `your-email@gmail.com` | - |
| `EMAIL_SMTP_PASSWORD` | SMTP password or app password | `your-app-password` | - |
| `EMAIL_SMTP_STARTTLS` | Enable STARTTLS | `true` or `false` | `true` |

### Backup Configuration (Optional, only if using backup service)

| Secret Name | Description | Example | Default |
|-------------|-------------|---------|---------|
| `BACKUP_GPG_PASSPHRASE` | GPG passphrase for backup encryption | `your-strong-passphrase` | - |
| `BACKUP_CRON` | Cron schedule for backups | `0 2 * * *` | `0 2 * * *` |
| `BACKUP_RETENTION_DAYS` | Number of days to retain backups | `14` | `14` |
| `BACKUP_S3_PREFIX` | S3 prefix for backup files | `backups/postgres` | `backups/postgres` |
| `BACKUP_RUN_ON_START` | Run backup immediately on container start | `false` | `false` |
| `TZ` | Timezone for backup schedule | `Asia/Jakarta` or `UTC` | `UTC` |

## Quick Setup Guide

### 1. Generate SSH Key for Deployment

```bash
# Generate SSH keypair
ssh-keygen -t ed25519 -f deploy_key -N ""

# Copy public key to VPS
ssh-copy-id -i deploy_key.pub -p 22 deploy@your-vps-ip

# Copy private key content for GitHub Secret
cat deploy_key
# Paste the entire output (including BEGIN/END lines) into VPS_SSH_KEY secret
```

### 2. Minimal Secrets (for testing)

At minimum, you need these to get the deployment working:

**VPS Connection:**
- `VPS_HOST`
- `VPS_USER`
- `VPS_SSH_KEY`
- `VPS_PORT`
- `VPS_DEPLOY_PATH`

**Application:**
- `BACKEND_IMAGE=ghcr.io/<owner>/<repo>/excalidraw-be:latest`
- `FRONTEND_IMAGE=ghcr.io/<owner>/<repo>/excalidraw-fe:latest`

**Database:**
- `DB_HOST`
- `DB_USER`
- `DB_PASSWORD`

**Storage:**
- `STORAGE_ENDPOINT`
- `STORAGE_ACCESS_KEY`
- `STORAGE_SECRET_KEY`

All other secrets have defaults and are optional.

### 3. Add Secrets to GitHub

```bash
# Using GitHub CLI (recommended)
gh secret set VPS_HOST -b "203.0.113.42"
gh secret set VPS_USER -b "deploy"
gh secret set VPS_SSH_KEY < deploy_key
gh secret set VPS_PORT -b "22"
gh secret set VPS_DEPLOY_PATH -b "/srv/dellcalidraw"

gh secret set BACKEND_IMAGE -b "ghcr.io/<owner>/<repo>/excalidraw-be:latest"
gh secret set FRONTEND_IMAGE -b "ghcr.io/<owner>/<repo>/excalidraw-fe:latest"
gh secret set BACKEND_PORT -b "8080"
gh secret set FRONTEND_PORT -b "3000"
gh secret set APP_ENV -b "production"

gh secret set DB_HOST -b "your-db-host"
gh secret set DB_PORT -b "5432"
gh secret set DB_USER -b "excalidraw"
gh secret set DB_PASSWORD -b "your-secure-password"
gh secret set DB_NAME -b "excalidraw"
gh secret set DB_SSLMODE -b "require"

gh secret set STORAGE_ENDPOINT -b "s3.us-east-1.amazonaws.com"
gh secret set STORAGE_ACCESS_KEY -b "your-access-key"
gh secret set STORAGE_SECRET_KEY -b "your-secret-key"
gh secret set STORAGE_BUCKET -b "excalidraw-files"
gh secret set STORAGE_REGION -b "us-east-1"
gh secret set STORAGE_USE_SSL -b "true"

# Optional: Email configuration
gh secret set EMAIL_PROVIDER -b "smtp"
gh secret set EMAIL_FROM -b "noreply@example.com"
gh secret set EMAIL_BASE_URL -b "https://your-app.example.com"
gh secret set EMAIL_SMTP_HOST -b "smtp.gmail.com"
gh secret set EMAIL_SMTP_PORT -b "587"
gh secret set EMAIL_SMTP_USERNAME -b "your-email@gmail.com"
gh secret set EMAIL_SMTP_PASSWORD -b "your-app-password"
```

Or add them manually via GitHub UI:
1. Go to your repository
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Add name and value for each secret

## How It Works

When you push to `main`:
1. GitHub Actions builds Docker images and pushes them to GHCR
2. The `deploy` job SSHes into your VPS
3. Renders `.env` file from all GitHub Secrets
4. Runs `docker compose pull && docker compose up -d`
5. Your app restarts with the new configuration

**No manual SSH needed!** Just update secrets in GitHub UI and push to trigger deployment.

## Security Notes

- GitHub Secrets are encrypted at rest
- Secrets are only exposed to workflow runs
- Audit logs track secret access
- Use `read:packages` PAT for `GHCR_PULL_TOKEN` (minimal permissions)
- The `.env` file on VPS is set to `chmod 600` (owner read/write only)
- Never commit `.env` to the repository

## Updating Configuration

To update any configuration:
1. Update the secret value in GitHub UI
2. Push any commit to `main` (or manually trigger workflow)
3. CD pipeline will render new `.env` and restart containers
4. Changes take effect immediately

No VPS SSH access required!
