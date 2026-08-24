# One-Domain Migration (Traefik)

Goal: serve frontend and backend from a **single domain** (`draw.domain.id`)
instead of two subdomains (`draw.domain.id` for FE, `draw-api.domain.id` for BE).

Why:
- One origin = no CORS on the API path.
- Frontend image becomes **domain-agnostic** — no rebuild when the domain or
  port changes.
- Traefik (already the VPS entrypoint) terminates TLS and routes by path.

---

## Current architecture

```
Browser
  ├── draw.domain.id ──────────────> FE (nginx static, container)
  └── draw-api.domain.id ──────────> BE (Go, container)
```

Effect: cross-origin API calls (FE on `draw.domain.id` calling
`draw-api.domain.id`) require CORS to be configured correctly.

## Target architecture

```
Internet ─> Traefik (:80/:443, TLS)
              ├── draw.domain.id/api/*  ──> backend container :8080
              ├── draw.domain.id/ws     ──> backend container :8080 (WS upgrade)
              └── draw.domain.id/*      ──> frontend nginx container
```

Same origin (`draw.domain.id`) for static assets **and** API/WebSocket.
CORS is no longer a concern on the API path.

---

## What changed in this repo

This migration **removes build-time env injection** from the frontend. The
frontend now falls back to same-origin URLs configured by the reverse proxy.

### Frontend services

All API/WS base-URL helpers no longer append a hardcoded `:8080` port. When
`VITE_API_URL` / `VITE_WS_URL` are unset (they now are, in production), the
client uses the page's own origin, letting the proxy decide routing:

| File | Change |
|---|---|
| `src/services/api.ts` | same-origin fallback (no `:8080`) |
| `src/services/fileService.ts` | same-origin fallback (no `:8080`) |
| `src/services/tokenRefreshService.ts` | same-origin fallback; SSR guard `http://localhost` (was `:8080`) |
| `src/services/ai/aiService.ts` | same-origin fallback; removed debug `console.log` |
| `src/services/roomService.ts` | WS same-origin fallback (`/ws`, no `:8080`) |

### Build / CI / CD

| File | Change |
|---|---|
| `excalidraw-fe/Dockerfile` | removed `ARG`/`ENV` for `VITE_API_URL` / `VITE_WS_URL` |
| `.github/workflows/ci.yml` | removed `build-args` from frontend image build |
| `.github/workflows/cd.yml` | removed `build-args` from frontend image build |

No Go/backend changes are needed: the backend already serves routes under
`/api/...` (REST) and `/ws` (WebSocket), matching what the frontend calls.

### Why this makes the image domain-agnostic

Production images no longer embed a specific API/WS origin. The same image
works for `draw.domain.id`, a staging domain, or a plain IP, solely depending
on how Traefik routes requests.

---

## Backend route map (for the Traefik config)

Backend paths the frontend actually calls:

| Frontend call | Backend route |
|---|---|
| `/api/auth/*` | `/api/auth/*` |
| `/api/files/*` | `/api/files/*` |
| `/api/rooms/{id}/canvas/*` | `/api/rooms/{id}/canvas/*` |
| `/api/rooms/{id}/files/*` | `/api/rooms/{id}/files/*` |
| `/api/ai/*` | `/api/ai/*` |
| `/api/stats` | `/api/stats` |
| `/api/rooms/{id}/link` | `/api/rooms/{id}/link` |
| `/api/ai/health` → backend `/api/ai/health` | `aiHandler` `/health` is **not** the container health endpoint |
| `/ws` (WebSocket) | `/ws` |

Notes:
- **No `stripPrefix` is needed.** The frontend sends `/api/...` and the backend
  registers `/api/...` directly. Forward the path unchanged.
- The container healthcheck (`docker-compose.yml`) hits `localhost:8080/health`
  inside the container — unrelated to Traefik routing.
- WebSocket upgrade must be forwarded with `ws`/`wss` scheme and the
  appropriate upgrade headers (Traefik handles this when given `webSocket`
  capability).

---

## Traefik configuration (VPS)

Traefik is the primary reverse proxy and other projects already run behind it.
This project does **not** need a second proxy layer — just join the shared
network Traefik can route to, and add a router + services.

### 1. Wire the project's containers into Traefik's network

In `docker-compose.yml`, attach the shared network. Replace the standalone
`excalidraw-network` (or add) a network Traefik can see. Example using a shared
`traefik` external network:

```yaml
networks:
  traefik:
    external: true
  excalidraw-network:
    driver: bridge

services:
  backend:
    networks:
      - excalidraw-network
      - traefik
  frontend:
    networks:
      - excalidraw-network
      - traefik
```

> Traefik is typically configured with `providers.docker` + `exposedByDefault:
> false`, so adding the network enables discovery. Your compose
> `docker-compose.yml` currently exposes host ports (`8080`, `3000`). Once
> Traefik routes internally, you may keep those host binds for debugging or
> remove them — Traefik will reach containers over the docker network directly.

### 2. Add labels (or a Traefik file provider)

The exact mechanism depends on how Traefik is configured (Docker provider
labels vs. a dynamic config file). The routing is:

| Route rule | Service | Middleware |
|---|---|---|
| Host `` `draw.domain.id` `` && PathPrefix `` `/api` `` | backend `:8080` | — (pass-through) |
| Host `` `draw.domain.id` `` && Path `` `/ws` `` | backend `:8080` | `webSocket` option on the service |
| Host `` `draw.domain.id` `` | frontend `:80` | — |

With Docker provider labels this looks like:

```yml
# backend service
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.excalidraw-api.rule=Host(`draw.domain.id`) && PathPrefix(`/api`)"
  - "traefik.http.routers.excalidraw-api.entrypoints=websecure"
  - "traefik.http.routers.excalidraw-api.tls.certresolver=letsencrypt"
  - "traefik.http.services.excalidraw-api.loadbalancer.server.port=8080"
  # WebSocket
  - "traefik.http.routers.excalidraw-ws.rule=Host(`draw.domain.id`) && Path(`/ws`)"
  - "traefik.http.routers.excalidraw-ws.entrypoints=websecure"
  - "traefik.http.routers.excalidraw-ws.tls.certresolver=letsencrypt"
  - "traefik.http.routers.excalidraw-ws.service=excalidraw-api"
  - "traefik.http.services.excalidraw-api.loadbalancer.server.scheme=http"

# frontend service
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.excalidraw-fe.rule=Host(`draw.domain.id`)"
  - "traefik.http.routers.excalidraw-fe.entrypoints=websecure"
  - "traefik.http.routers.excalidraw-fe.tls.certresolver=letsencrypt"
```

WebSocket upgrades require enabling the `webSocket` connect method on the load
balancer healthcheck if present, and the router must not swallow the `Upgrade`
header. With `traefik` docker provider, Websocket forwarding works out of the
box as long as the backend service port is correct; do **not** apply
`stripPrefix` on the `/ws` route.

---

## Deployment steps (after this PR)

1. Merge this change (frontend now builds domain-agnostic).
2. On the VPS: add this project to the Traefik network + labels (above).
3. `docker compose pull && docker compose up -d --remove-orphans` (or rerun the
   CD workflow).
4. Update DNS: keep `draw.domain.id` pointing at the VPS; `draw-api.domain.id`
   can stop being used after traffic drains.
5. Verify:
   - `curl -s https://draw.domain.id/api/health`
   - Open the site, log in, draw, and confirm AI chat + persistent save work.
   - Watch browser DevTools → Network: API/WS requests should all target
     `draw.domain.id`.

---

## Rollback

Because the frontend image no longer carries env URLs, rollback to the
two-subdomain model requires rebuilding the image with `VITE_API_URL`/
`VITE_WS_URL` re-added (previous CI/CD did this). A previously-built image
with baked-in URLs would need to be re-pushed or the build-args restored. If
you need to keep both models working simultaneously, re-enable build-args in
`cd.yml` — the same-origin fallback only applies when those vars are unset.