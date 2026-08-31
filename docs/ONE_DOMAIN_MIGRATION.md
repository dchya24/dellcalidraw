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

Traefik runs in its own compose project — network `traefik_default` contains
only the Traefik container itself; application containers never join it.
Routes are declared through the **file provider** (dynamic config), pointing
at the host ports this project publishes. This is the same mechanism the
other services on the VPS already use, and it needs **zero repo changes**:
`docker-compose.yml` already publishes `${FRONTEND_PORT:-3000}:80` and
`${BACKEND_PORT:-8080}:8080`.

### Dynamic config (file provider)

Add a router/service set to Traefik's dynamic config directory (the same
place the routers for the existing services are declared — the directory
Traefik watches via `providers.file.directory`):

```yaml
http:
  routers:
    draw-api:
      rule: "Host(`draw.domain.id`) && PathPrefix(`/api`)"
      entryPoints: [websecure]
      service: draw-backend
      tls:
        certResolver: letsencrypt
    draw-ws:
      rule: "Host(`draw.domain.id`) && Path(`/ws`)"
      entryPoints: [websecure]
      service: draw-backend
      tls:
        certResolver: letsencrypt
    draw-fe:
      rule: "Host(`draw.domain.id`)"
      entryPoints: [websecure]
      service: draw-frontend
      tls:
        certResolver: letsencrypt
  services:
    draw-backend:
      loadBalancer:
        servers:
          - url: "http://<host>:8080"
    draw-frontend:
      loadBalancer:
        servers:
          - url: "http://<host>:3000"
```

Notes:
- Longer rules rank higher in Traefik, so the `/api` and `/ws` routers take
  precedence over the catch-all `Host` router — no explicit `priority` needed.
- **No `stripPrefix`**: forward paths unchanged (the backend registers
  `/api/...` and `/ws` itself).
- WebSocket works out of the box on Traefik v2+ (Upgrade headers are
  forwarded as-is).
- `<host>` must be the address the Traefik container can use to reach the
  host-published ports. Copy the pattern from the existing services' config —
  common options: Traefik with `network_mode: host`,
  `host.docker.internal` + `extra_hosts: ["host-gateway:172.17.0.1"]`, or the
  docker bridge gateway IP.
- Keep the host port binds in `docker-compose.yml` — under this model they
  are the routing path, not a debug affordance.

---

## Deployment steps

1. Merge this change (frontend now builds domain-agnostic).
2. On the VPS: add the routers/services above to Traefik's dynamic config
   (file provider), matching the entrypoint/cert-resolver names used by the
   existing services. Traefik reloads dynamic config automatically on file
   change.
3. Remove the old `draw-api.domain.id` router from the same config once the
   new one is verified (keep `draw.domain.id` DNS as-is).
4. Verify:
   - `curl -s https://draw.domain.id/api/stats` — exercises the `/api`
     router end to end. (Backend `/health` is registered at the root path and
     is intentionally not routed publicly.)
   - Open the site, log in, draw, and confirm AI chat + persistent save work.
   - Watch browser DevTools → Network: API/WS requests should all target
     `draw.domain.id` (no CORS preflights).

No repo changes are required for the routing itself: `docker-compose.yml`,
`cd.yml`, and `.env.example` already publish the host ports the Traefik
config points at, and the CD deploy flow is untouched.

---

## Rollback

Under the file-provider model, rollback is Traefik-config-only: re-add (or
un-comment) the `draw-api.domain.id` router pointing at the backend host
port, and the two-subdomain setup works again. No image rebuild is needed —
the frontend image is domain-agnostic, but the served page only uses
same-origin URLs; the browser clients already deployed keep calling
`draw.domain.id/api/...`, which the `/api` router continues to serve either
way.
