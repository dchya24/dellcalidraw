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
2. Clone the repo (or copy just `docker-compose.yml` + `.env`):
   ```bash
   git clone <repo-url> /srv/dellcalidraw
   cd /srv/dellcalidraw
   cp .env.example .env
   # fill in DB / storage credentials and the GHCR-prefixed image tags:
   #   BACKEND_IMAGE=ghcr.io/<owner>/<repo>/excalidraw-be:latest
   #   FRONTEND_IMAGE=ghcr.io/<owner>/<repo>/excalidraw-fe:latest
   ```
3. (Private packages only) `docker login ghcr.io` once with a PAT that has
   `read:packages`.
4. Confirm a baseline deploy works manually:
   ```bash
   docker compose pull && docker compose up -d
   ```

#### GitHub repository secrets

Add these under *Settings → Secrets and variables → Actions*:

| Secret              | Purpose                                                |
|---------------------|--------------------------------------------------------|
| `VPS_HOST`          | Hostname or IP of the VPS                              |
| `VPS_USER`          | SSH user (e.g. `deploy`)                               |
| `VPS_SSH_KEY`       | Private key (PEM) for that user. Public key in `~/.ssh/authorized_keys` on the VPS. |
| `VPS_PORT`          | SSH port (e.g. `22`)                                   |
| `VPS_DEPLOY_PATH`   | Absolute path on the VPS that contains `docker-compose.yml` and `.env` (e.g. `/srv/dellcalidraw`) |
| `GHCR_PULL_TOKEN`   | (private packages only) PAT with `read:packages` so the VPS can pull. Leave unset if the package is public. |

Generate the SSH keypair locally, install the public half on the VPS, and
paste the **private** half into `VPS_SSH_KEY`:

```bash
ssh-keygen -t ed25519 -f deploy_key -N ""
ssh-copy-id -i deploy_key.pub -p <port> deploy@<vps>
# then `pbcopy < deploy_key` (or just open the file) and store as VPS_SSH_KEY
```

Once the secrets are in place, every push to `main` will publish images and
deploy them to the VPS. The `concurrency: vps-deploy` group serializes
deploys so two simultaneous main pushes won't fight over the host.

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
