# Deployment Guide

## Prerequisites
- Docker installed on your system
- Docker Compose installed
- For CI/CD: GitHub repository with GitHub Actions enabled

## Local Development

### Quick Start
```bash
# Build and start all services
make build
make up

# Or use docker-compose directly
docker-compose up -d
```

### Development Mode (with hot reload)
```bash
# Start development environment
make dev

# Or
docker-compose -f docker-compose.dev.yml up -d
```

### View Logs
```bash
# All services
make logs

# Frontend only
make logs-fe

# Backend only
make logs-be
```

### Stop Services
```bash
make down
```

## Production Deployment

`docker-compose.yml` only defines the application services (`backend` and
`frontend`). PostgreSQL and the S3-compatible object store (MinIO, AWS S3,
Cloudflare R2, etc.) are **external dependencies** and must be provisioned
separately. The backend reads connection info from environment variables.

### 1. Configure environment

```bash
cp .env.example .env
# fill in EXCALIDRAW_DATABASE_*, EXCALIDRAW_STORAGE_*, image tags, etc.
```

Docker Compose auto-loads `.env` from the project root. You can also export
the variables in the shell or pass them via your orchestrator.

### 2. Choose where the images come from

- Build locally: leave `BACKEND_IMAGE` / `FRONTEND_IMAGE` at their defaults,
  then `docker compose build`.
- Pull from registry (recommended): set
  ```
  BACKEND_IMAGE=ghcr.io/<owner>/<repo>/excalidraw-be:<tag>
  FRONTEND_IMAGE=ghcr.io/<owner>/<repo>/excalidraw-fe:<tag>
  ```
  then `docker compose pull && docker compose up -d`.

### Using Docker Compose
```bash
# Build and start production
make prod-up

# Stop production
make prod-down
```

### Using GitHub Actions CI/CD

The repository includes automated CI/CD pipelines:

**CI Pipeline** (`.github/workflows/ci.yml`):
- Runs on push to main/develop and pull requests
- Frontend: lint, type check, build
- Backend: format check, build, test
- Docker build validation

**CD Pipeline** (`.github/workflows/cd.yml`):
- Runs on push to main branch (or `workflow_dispatch`)
- Job 1 (`build-and-push`) builds and pushes Docker images to GitHub
  Container Registry (`ghcr.io`). Tags: branch, commit SHA, `latest` (only
  when pushed from the default branch), and SemVer when a `vX.Y.Z` tag is
  pushed.
- Job 2 (`deploy`) SSHes into a VPS and runs
  `docker compose pull && docker compose up -d`. Only fires on direct
  pushes to `main`.

#### One-time VPS setup

On the VPS, as the deploy user:

1. Install Docker + Compose plugin.
2. Clone the repo (or copy just `docker-compose.yml`):
   ```bash
   git clone <repo-url> /srv/dellcalidraw
   cd /srv/dellcalidraw
   ```
3. (Private packages only) `docker login ghcr.io` once with a PAT that has
   `read:packages`.
4. **That's it!** The CD pipeline will automatically create `.env` from GitHub Secrets on the first deploy.

**Note:** You no longer need to manually create or edit `.env` on the VPS. The CD pipeline renders it from GitHub Secrets on every deployment.

#### GitHub repository secrets

The CD pipeline now **automatically renders `.env` on the VPS from GitHub Secrets** — you no longer need to SSH into the VPS to update environment variables. 

Add secrets under *Settings → Secrets and variables → Actions*. See the complete list and setup guide in **[docs/GITHUB_SECRETS.md](docs/GITHUB_SECRETS.md)**.

**Minimal required secrets:**

**VPS Connection:**
- `VPS_HOST` — Hostname or IP of the VPS
- `VPS_USER` — SSH user (e.g. `deploy`)
- `VPS_SSH_KEY` — Private key (PEM) for that user
- `VPS_PORT` — SSH port (e.g. `22`)
- `VPS_DEPLOY_PATH` — Path containing `docker-compose.yml` (e.g. `/srv/dellcalidraw`)
- `GHCR_PULL_TOKEN` — (private packages only) PAT with `read:packages`

**Application:**
- `BACKEND_IMAGE` — e.g. `ghcr.io/<owner>/<repo>/excalidraw-be:latest`
- `FRONTEND_IMAGE` — e.g. `ghcr.io/<owner>/<repo>/excalidraw-fe:latest`
- `BACKEND_PORT`, `FRONTEND_PORT`, `APP_ENV`

**Database:**
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`

**Storage:**
- `STORAGE_ENDPOINT`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_BUCKET`, `STORAGE_REGION`, `STORAGE_USE_SSL`

**Optional:** Email, WebSocket, Backup configuration (see [docs/GITHUB_SECRETS.md](docs/GITHUB_SECRETS.md))

Generate the SSH keypair locally, install the public half on the VPS, and
paste the **private** half into `VPS_SSH_KEY`:

```bash
ssh-keygen -t ed25519 -f deploy_key -N ""
ssh-copy-id -i deploy_key.pub -p <port> deploy@<vps>
# then `cat deploy_key` and paste the entire output as VPS_SSH_KEY secret
```

**Quick setup with GitHub CLI:**

```bash
# VPS connection
gh secret set VPS_HOST -b "your-vps-ip"
gh secret set VPS_USER -b "deploy"
gh secret set VPS_SSH_KEY < deploy_key
gh secret set VPS_PORT -b "22"
gh secret set VPS_DEPLOY_PATH -b "/srv/dellcalidraw"

# Images
gh secret set BACKEND_IMAGE -b "ghcr.io/<owner>/<repo>/excalidraw-be:latest"
gh secret set FRONTEND_IMAGE -b "ghcr.io/<owner>/<repo>/excalidraw-fe:latest"

# Database (example)
gh secret set DB_HOST -b "your-db-host"
gh secret set DB_PASSWORD -b "your-secure-password"
# ... (see docs/GITHUB_SECRETS.md for complete list)
```

Once the secrets are in place, every push to `main` will:
1. Build and push Docker images to GHCR
2. SSH into the VPS
3. Render `.env` from GitHub Secrets
4. Pull latest images and restart containers

**To update configuration:** Just update the secret in GitHub UI and push to `main`. No VPS SSH needed!

The `concurrency: vps-deploy` group serializes deploys so two simultaneous main pushes won't fight over the host.

### Manual Docker Build and Push

```bash
# Build images
make build-fe
make build-be

# Tag images
docker tag excalidraw-fe:latest ghcr.io/your-username/excalidraw-fe:latest
docker tag excalidraw-be:latest ghcr.io/your-username/excalidraw-be:latest

# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Push images
docker push ghcr.io/your-username/excalidraw-fe:latest
docker push ghcr.io/your-username/excalidraw-be:latest
```

## Environment Configuration

### Root `.env` (production stack)
Used by `docker-compose.yml` to wire the backend to external Postgres and
object storage. Copy from `.env.example` and fill in real values.

### Frontend
Create `excalidraw-fe/.env`:
```env
VITE_API_URL=http://localhost:8080
```

### Backend
Create `excalidraw-be/.env`:
```env
PORT=8080
HOST=0.0.0.0
```
Non-secret defaults can stay here. Real secrets (DB password, storage keys,
JWT signing key, AI API key) should be supplied via the host environment or
a secrets manager rather than committed.

## Service URLs
- Frontend: http://localhost:3000
- Backend: http://localhost:8080
- Backend Health: http://localhost:8080/health

## Health Checks
Both services include Docker health checks:
- Frontend: HTTP GET on port 80
- Backend: HTTP GET on port 8080/health

## Cleaning Up
```bash
# Remove all containers and images
make clean
```

## Available Make Commands
Run `make help` to see all available commands.
