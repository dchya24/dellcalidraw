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
- Runs on push to main branch
- Builds and pushes Docker images to GitHub Container Registry (ghcr.io)
- Tags images with branch, commit SHA, and semantic version

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
