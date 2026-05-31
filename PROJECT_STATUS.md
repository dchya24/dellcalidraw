# Dellcalidraw - Project Status Report

**Generated:** 2026-05-31
**Branch:** dev
**Last Commit:** `f45eee0` - ci(cd): deploy to VPS via SSH after image push

---

## 📊 Executive Summary

**Overall Project Completion: ~93%**

Dellcalidraw is a real-time collaborative whiteboard application built with:
- **Frontend:** React 18 + Vite 5 + TypeScript + Excalidraw 0.18
- **Backend:** Go 1.25 + WebSocket + PostgreSQL + S3-compatible storage (MinIO/AWS)
- **Infrastructure:** Docker Compose orchestration + GitHub Actions CI/CD (lint, typecheck, build, deploy via SSH)

### Current Status
- ✅ **Real-Time Collaboration:** 90% Complete (Phases 1–7 done)
- ✅ **Persistence:** 100% Complete (Phases 8–9 done, plus per-tab cloud persistence)
- ✅ **Authentication:** 100% Complete (Phase 10 done, with password reset flow)
- ✅ **WebSocket Encryption:** Phase 11–12 complete (per-room AES-256-GCM)
- ✅ **AI Chat → Diagram:** Sprint 1, 2 & 3 complete (Mermaid, auto_layout, token tracking)
- ✅ **Guest → Cloud Sync:** Local files migrate to cloud on login
- ✅ **CI/CD:** GitHub Actions running lint/typecheck/build, CD to VPS
- 🟡 **Production Hardening:** ~70% (needs WebSocket encryption, monitoring, load testing, backups)

---

## ✅ Completed Features

### Backend (Go 1.25)

#### Phase 1–5: Core WebSocket & Collaboration (100% Complete)
- ✅ WebSocket server with `gorilla/websocket`
- ✅ HTTP router with `go-chi/chi v5`
- ✅ Room management with thread-safe operations (`sync.RWMutex`)
- ✅ Real-time element synchronization (delta updates)
- ✅ Cursor tracking and broadcasting (throttled 20/sec)
- ✅ Selection awareness
- ✅ User presence tracking
- ✅ Rate limiting (20 msg/sec, 100 msg/10sec window)
- ✅ Element validation (type, coordinates, size limits)
- ✅ Room cleanup (1-hour inactivity timeout)
- ✅ Graceful shutdown handling

#### Phase 6–7: Connection Stability (100% Complete)
- ✅ Exponential backoff reconnection (1s→30s)
- ✅ Heartbeat / Ping-Pong (10s interval, 10s timeout)
- ✅ Message queue for offline support (max 100 messages)
- ✅ Promise-based acknowledgment system (10s timeout)
- ✅ 4-state connection management (disconnected/connecting/connected/reconnecting)
- ✅ Auto re-join room on reconnect

#### Phase 8: PostgreSQL Database Integration (100% Complete)
- ✅ PostgreSQL 16 with connection pooling (25 max open, 5 idle)
- ✅ Embedded migration system (`golang-migrate` + `embed.FS`)
- ✅ Throttled persistence (3-second batch UPSERT)
- ✅ Initial scene loading from database on room join
- ✅ Graceful degradation (runs without DB if unavailable)
- ✅ Snapshot persistence on room cleanup and server shutdown
- ✅ Schema: `rooms`, `room_elements`, `room_files`, `users`, `room_permissions`, `password_reset`, `user_files`, `file_tabs`, `ai_request_logs` (8 migrations)

#### Phase 9: S3-Compatible File Storage (100% Complete)
- ✅ AWS SDK v2 client (`aws-sdk-go-v2`) — works with MinIO and AWS S3
- ✅ HTTP upload endpoint (`POST /api/rooms/{roomId}/files`)
- ✅ HTTP download endpoint (`GET /api/rooms/{roomId}/files/{fileId}`)
- ✅ File metadata stored in PostgreSQL
- ✅ Docker Compose MinIO service for development

#### Phase 10: JWT Authentication (100% Complete)
- ✅ User registration with bcrypt password hashing
- ✅ JWT access tokens (15-minute expiry)
- ✅ Refresh token rotation (7-day expiry)
- ✅ Protected API endpoints with middleware
- ✅ Logout with token invalidation
- ✅ Password reset flow (forgot / validate-token / reset)
- ✅ User session management

#### File Tabs & Guest → Cloud Sync (100% Complete)
- ✅ `file_tabs` table for per-tab canvas data (elements, appState, files)
- ✅ `POST /api/files/migrate` bulk migration endpoint
- ✅ `PUT /api/files/{fileId}/tabs` save tabs endpoint
- ✅ `GET /api/files` returns files with embedded tabs
- ✅ Local-only files preserved on login, then migrated server-side

#### AI Chat → Diagram Backend (Sprint 1 & 2 Complete)
- ✅ SSE streaming handler (`/api/ai/chat`, `/api/ai/models`, `/api/ai/health`)
- ✅ OpenAI provider (also serves Zhipu, Groq, Ollama, any OpenAI-compatible API)
- ✅ Anthropic provider (Claude models)
- ✅ AI request logging (dev-only `/api/ai/logs` endpoints)
- ✅ IP rate limiter for AI endpoints (2 req/s, burst 6)
- ✅ Model validation against provider catalog
- ✅ Environment-aware behavior (`APP_ENV`)
- ✅ 19 MCP tools: `create_rectangle`, `create_ellipse`, `create_diamond`, `create_text`, `create_arrow`, `create_line`, `create_zone`, `move_elements`, `delete_elements`, `update_element_style`, `edit_text`, `camera_update`, `get_canvas_state`, `convert_mermaid`, `auto_layout`, `create_group`, `duplicate_elements`, `resize_elements`, `align_elements`
- ✅ Token usage tracking via SSE `usage` event (OpenAI `stream_options.include_usage`, Anthropic `message_start`/`message_delta`)

### Frontend (React 18.3 + Vite 5.4)

#### Core Whiteboard (100% Complete)
- ✅ Excalidraw 0.18 integration with full drawing tools
- ✅ Multi-tab support per file (Zustand state)
- ✅ Multi-file workspace with sidebar + tab bar
- ✅ Local persistence (IndexedDB via Zustand persist)
- ✅ Tab switching with auto-save
- ✅ Theme support (light/dark)
- ✅ Full export/import/cloud options in Excalidraw MainMenu

#### Real-Time Collaboration (90% Complete)
- ✅ WebSocket service (singleton)
- ✅ Room management service (join/leave)
- ✅ Element sync service (delta updates, debounced 100ms)
- ✅ Cursor service (throttled 10/sec, 5px threshold)
- ✅ Selection service (debounced 100ms)
- ✅ Remote cursor rendering with coordinate transform
- ✅ Selection overlay with colored borders
- ✅ Collaboration panel with participant list
- ✅ Room invite dialog with auto-join from URL (`?room={roomId}`)
- ✅ Room password / permissions dialog
- ✅ Room settings panel
- ✅ Conflict resolution panel
- ✅ Connection status indicators

#### Authentication UI (100% Complete)
- ✅ Login/Register modal with validation
- ✅ Forgot password + reset password modals
- ✅ JWT token storage and refresh service
- ✅ Protected API calls
- ✅ Sign In/Sign Out in MainMenu
- ✅ Migration dialog (guest → cloud)

#### AI Chat Panel (Sprint 1, 2 & 3 Complete)
- ✅ Full chat panel UI (`AIChatPanel.tsx`)
- ✅ SSE streaming consumption
- ✅ AI Chat store (Zustand) with per-tab conversations
- ✅ Real-time element creation as tools stream
- ✅ Stop generation button (AbortController)
- ✅ Suggested prompts
- ✅ Tool call badges with batch undo button
- ✅ Show-on-canvas button (selects + scrolls to AI-generated elements)
- ✅ Markdown rendering for assistant messages (react-markdown + remark-gfm)
- ✅ Resizable panel (drag bottom-left handle, persisted to localStorage)
- ✅ Model selector dropdown (consumes `/api/ai/models` + `/api/ai/health`)
- ✅ Conversation persistence to localStorage (max 20 conversations × 100 messages)
- ✅ Per-tab conversation isolation
- ✅ Mermaid converter via `@excalidraw/mermaid-to-excalidraw` (lazy-loaded)
- ✅ Auto-layout (vertical / horizontal / grid) with binding-aware element shifts
- ✅ Token usage badge per assistant message + cumulative session total in header

---

## 🚧 Pending Work

### High Priority

#### 1. WebSocket Message Encryption (Phase 11–12) ✅ DONE 2026-05-31
Per-room AES-256-GCM on top of TLS. Defense in depth, NOT end-to-end
(server still needs plaintext for persistence, AI, conflict resolution).

- Backend: `internal/crypto/aesgcm.go` (Seal/Open), migration `000009_room_encryption_key`,
  `rooms.encryption_key` column, `GetOrCreateRoomEncryptionKey` repository method,
  `EncryptionHandshakePayload` type, `frameMessage`/`decryptInbound` in WS handler,
  `encryption_handshake` sent plaintext after `join_room`, then connection is armed.
- Frontend: `services/cryptoService.ts` (WebCrypto AES-GCM 256), `websocket.ts`
  intercepts inbound `encrypted` envelopes (serialised via promise chain to preserve
  order) and outbound encryption (heartbeats / `join_room` stay plaintext).
- Config flag: `EXCALIDRAW_WEBSOCKET_ENCRYPTION_ENABLED` (default true). Falls back
  to plaintext if no DB available.

#### 2. AI Sprint 3 — Game Changers ✅ DONE 2026-05-31
- [x] Mermaid → Excalidraw converter (`convert_mermaid` MCP tool, `@excalidraw/mermaid-to-excalidraw`)
- [x] `auto_layout` tool (vertical / horizontal / grid)
- [x] Token usage tracking + UI display

#### 3. Verify Guest → Cloud Sync end-to-end 🟡
**Status:** Code landed (`b0dded3`), needs fresh manual smoke test
**Effort:** 1 day
- Validate: guest creates files → register → files appear with cloud icon → cross-browser login

### Medium Priority

#### 4. Production Monitoring (Phase 13)
**Effort:** 2–3 days
- Prometheus metrics, Grafana dashboards
- Error tracking (Sentry)
- Detailed health checks (DB, S3, AI provider)

#### 5. Load Testing (Phase 14)
**Effort:** 2–3 days
- 50+ concurrent users, 1000+ elements, 100+ rooms
- Bottleneck identification, query optimization

#### 5. Backups & Disaster Recovery ✅ DONE 2026-05-31
Nightly Postgres dumps, gpg-encrypted (AES256), pushed to the same
S3-compatible bucket the application uses. Opt-in via Compose `backup`
profile.

- `ops/backup/` — Alpine container with postgres-client, aws-cli, gpg, dcron
- `ops/backup/backup.sh` — pg_dump → gzip → gpg → S3 + retention prune
- `ops/backup/restore.sh` — S3 → gpg → gunzip → psql, gated by RESTORE_CONFIRM=YES
- `docker-compose.yml` — `backup` service under `--profile backup`
- `docs/BACKUPS.md` — full runbook including DR checklist

Gate: requires `BACKUP_GPG_PASSPHRASE` in `.env`. Recommend pairing
with bucket-level versioning + lifecycle for layered protection.

### Low Priority Enhancements
- Idle detection (1–2 days)
- Viewport sync / "Follow User" (2–3 days)
- Advanced conflict resolution (operational transformation, 5–7 days)
- File versioning / history
- Markdown rendering in AI chat
- AI element highlighting
- Mobile / touch support
- i18n
- Additional AI tools: `create_group`, `duplicate_elements`, `resize_elements`, `align_elements`
- Gemini provider, multi-provider rotation

---

## 📁 Project Structure

```
dellcalidraw/
├── excalidraw-fe/                    # React frontend
│   └── src/
│       ├── components/
│       │   ├── ai/AIChatPanel.tsx
│       │   ├── Whiteboard.tsx
│       │   ├── Sidebar.tsx
│       │   ├── TabBar.tsx
│       │   ├── FloatingTab.tsx
│       │   ├── Toolbar.tsx
│       │   ├── CollaborationPanel.tsx
│       │   ├── RemoteCursors.tsx
│       │   ├── SelectionOverlay.tsx
│       │   ├── RoomInviteDialog.tsx
│       │   ├── RoomPasswordDialog.tsx
│       │   ├── RoomSettingsPanel.tsx
│       │   ├── ConflictResolutionPanel.tsx
│       │   ├── AuthModal.tsx
│       │   ├── ForgotPasswordModal.tsx
│       │   ├── ResetPasswordModal.tsx
│       │   ├── MigrationDialog.tsx
│       │   └── ConfirmDialog.tsx
│       ├── services/
│       │   ├── ai/
│       │   ├── api.ts
│       │   ├── websocket.ts
│       │   ├── roomService.ts
│       │   ├── roomPermissionsService.ts
│       │   ├── elementSyncService.ts
│       │   ├── cursorService.ts
│       │   ├── selectionService.ts
│       │   ├── fileService.ts
│       │   ├── exportImportService.ts
│       │   └── tokenRefreshService.ts
│       ├── store/
│       │   ├── useWhiteboardStore.ts
│       │   ├── useAuthStore.ts
│       │   ├── useThemeStore.ts
│       │   └── useAIChatStore.ts
│       └── types/{ai,auth,roomPermissions,websocket}.ts
│
├── excalidraw-be/                    # Go backend
│   ├── cmd/server/
│   │   ├── main.go
│   │   ├── auth_handlers.go
│   │   ├── canvas_handlers.go
│   │   ├── file_handlers.go
│   │   └── file_management_handlers.go
│   └── internal/
│       ├── ai/{handler,openai,anthropic,provider}.go
│       ├── auth/
│       ├── config/
│       ├── database/
│       │   └── migrations/000001..000008_*.{up,down}.sql
│       ├── middleware/{logging,ratelimit}.go
│       ├── room/
│       ├── storage/
│       └── websocket/
│
├── docs/
│   ├── be/{PROGRESS,DEVELOPMENT_PHASES,BACKEND_REQUIREMENTS}.md
│   ├── fe/...
│   ├── plans/2026-05-19-guest-to-cloud-sync.md
│   ├── AI_CHAT_DIAGRAM.md
│   ├── ai_implementation_todo_160526.md
│   └── ...
│
├── docker-compose.yml / docker-compose.dev.yml
├── Makefile
├── DEPLOYMENT.md
├── .github/workflows/{ci,cd}.yml
└── AGENTS.md
```

---

## 🔧 Technology Stack

### Frontend
- React 18.3.1, Vite 5.4.11, TypeScript 5.6
- Zustand 5 (state + persist)
- @excalidraw/excalidraw 0.18
- Tailwind CSS 3, lucide-react

### Backend
- Go 1.25
- go-chi/chi v5, gorilla/websocket v1.5
- PostgreSQL 16, lib/pq, golang-migrate
- aws-sdk-go-v2 (S3-compatible storage)
- zap, viper
- golang-jwt/jwt v5, bcrypt
- AI providers: OpenAI-compatible (OpenAI, Zhipu, Groq, Ollama), Anthropic

### Infrastructure
- Docker + Docker Compose (dev + prod)
- GitHub Actions CI (lint, typecheck, build artifacts) + CD (image push + SSH deploy)
- MinIO (dev), AWS S3 or compatible (prod)
- Nginx reverse proxy (production)

---

## 🚀 Quick Start

```bash
# Development
make dev          # all services
make logs         # tail logs
make logs-fe
make logs-be
make down

# Production
make prod-up
make prod-down

# Frontend only
cd excalidraw-fe && npm install && npm run dev

# Backend only
cd excalidraw-be && go mod download && make run
```

See `DEPLOYMENT.md` for VPS deploy specifics.

---

## 🧪 Testing Status

### Manual
- ✅ WebSocket connect / reconnect, room join/leave
- ✅ Element sync, cursor tracking, selection awareness
- ✅ Room invite + auto-join
- ✅ Database persistence across restarts
- ✅ File upload / download
- ✅ Auth: register, login, logout, refresh, password reset
- ✅ AI chat: streaming, model switch, batch undo, conversation persistence
- 🟡 Guest → Cloud sync (code landed, end-to-end re-verification recommended)

### Automated
- ✅ CI lint + typecheck + build (frontend)
- ✅ CI go fmt + go test -race + coverage (backend)
- ✅ Frontend Vitest unit tests: cryptoService, aiService SSE parser
- ✅ Backend `go test` unit tests:
  - `internal/crypto` (AES-GCM round-trip, tamper, wire format)
  - `internal/auth` (bcrypt, JWT lifecycle, refresh randomness, tamper detection)
  - `internal/config` (defaults, env overrides, CORS parsing)
  - `internal/middleware` (security headers, rate limiter behavior)
  - `internal/ai` (system prompt, 19-tool registry integrity, schema sanity)
- ❌ HTTP integration tests (handlers coupled to `*PostgresClient`, deferred)
- ❌ WebSocket / collab tests (deferred per focus)
- ❌ E2E browser tests (Playwright/Cypress) — deferred
- ❌ Load tests (deferred per focus)

See `docs/TESTING.md` for the running guide and what's still uncovered.

---

## 🔐 Security Status

### Implemented
- ✅ JWT auth with refresh rotation
- ✅ Bcrypt password hashing, password reset flow
- ✅ Rate limiting on WebSocket
- ✅ Rate limiting on AI HTTP endpoints (2 req/s, burst 6)
- ✅ Rate limiting on auth HTTP endpoints (1 req/s, burst 5)
- ✅ Element validation/sanitization
- ✅ CORS configuration
- ✅ Parameterized SQL queries
- ✅ Per-room AES-256-GCM WebSocket message encryption (Phase 11–12)
- ✅ Encrypted Postgres backups to S3 (gpg AES256)
- ✅ Security headers (CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy, COOP; HSTS opt-in for production)

### Missing
- ❌ File encryption at rest
- ❌ HTTPS/TLS terminated by app (relies on reverse proxy)
- ❌ CSRF protection (mitigated for now via CORS + bearer tokens)

---

## 📋 Recommended Next Steps

**Active focus (this iteration):**
1. ✅ Documentation sweep
2. ✅ AI Sprint 3 (Mermaid, auto_layout, token tracking)
3. ✅ WebSocket encryption (Phase 11–12)
4. ✅ Backups (Postgres + S3)
5. ✅ Security headers + auth rate limiting
6. ✅ AI tools tambahan (group / duplicate / resize / align)
7. ✅ Markdown chat + element highlighting + resizable panel
8. ✅ OT design plan (`docs/plans/2026-05-31-operational-transformation.md`)
9. ✅ Test scaffolding (Vitest FE) + pure-logic tests both stacks
10. ✅ Bug fix: refresh tokens now opaque random 256-bit (was deterministic JWT)

**Following:**
11. HTTP integration tests (would unblock guest→cloud sync verification)
12. Production monitoring (Prometheus/Grafana/Sentry)
13. File encryption at rest
14. OT implementation (per the plan, after monitoring + replay harness)

---

## 📞 Contact & Resources

### Repository
- **Branch:** dev
- **Last Updated:** 2026-05-31

### Documentation
- **Backend Progress:** `docs/be/PROGRESS.md`
- **Frontend Integration:** `docs/fe/INTEGRATION_COMPLETE.md`
- **AI feature plan:** `docs/AI_CHAT_DIAGRAM.md`, `docs/ai_implementation_todo_160526.md`
- **Guest sync plan:** `docs/plans/2026-05-19-guest-to-cloud-sync.md`
- **Deployment:** `DEPLOYMENT.md`
- **Agent Guide:** `AGENTS.md`

---

**Status:** 🟢 Active Development | 🎯 ~82% Complete | 🚀 Production-grade for core features, hardening in progress
