# Dellcalidraw - AI Agent Guide

This is a **dual-project repository** containing a React frontend and Go backend service orchestrated via Docker Compose.

## Project Structure
- **Frontend**: `excalidraw-fe/` - React 19 + Vite + TypeScript + Excalidraw
- **Backend**: `excalidraw-be/` - Go 1.24 + WebSocket + go-chi/chi
- **Sub-package AGENTS.md**: Each package has detailed documentation (see below)

## Root Setup Commands

```bash
# Start both services in development mode
make dev

# Start production environment
make prod-up

# View logs
make logs           # All services
make logs-fe        # Frontend only
make logs-be        # Backend only

# Stop all services
make down

# Build individual services (for debugging)
cd excalidraw-fe && npm run build
cd excalidraw-be && make build
```

## Universal Conventions

- **Frontend**: Use functional components, Zustand for state, service layer for WebSocket/room logic
- **Backend**: Follow standard Go layout (cmd/internal), use mutex for concurrent access, JSON WebSocket protocol
- **Commit format**: Conventional commits - `feat:`, `fix:`, `refactor:`, `config:`
- **TypeScript strict mode** enabled
- **Code style**: ESLint (frontend), golangci-lint (backend)

## Security & Secrets

- **NEVER** commit `.env` files - use `.env.example` as template
- Frontend secrets: Place in `excalidraw-fe/.env` (gitignored)
- Backend secrets: Place in `excalidraw-be/.env` (gitignored)
- No PII in logs or error messages

## JIT Index (what to open, not what to paste)

### Package Structure
- **Frontend (React)**: `excalidraw-fe/` → [see excalidraw-fe/AGENTS.md](excalidraw-fe/AGENTS.md)
- **Backend (Go)**: `excalidraw-be/` → [see excalidraw-be/AGENTS.md](excalidraw-be/AGENTS.md)
- **Documentation**: `docs/` - Project roadmap, phases, and integration guides
  - Backend phases: `docs/be/DEVELOPMENT_PHASES.md`
  - Frontend integration: `docs/fe/FRONTEND_INTEGRATION.md` (Phase 3 & 4 complete)
  - Phase summary: `docs/fe/PHASE_SUMMARY.md`

### Quick Find Commands

```bash
# Find a React component
rg -n "export (default )?function" excalidraw-fe/src/components

# Find a Zustand store
rg -n "create\(" excalidraw-fe/src/store

# Find a frontend service
rg -n "class.*Service" excalidraw-fe/src/services

# Find a Go struct
rg -n "type.*struct" excalidraw-be/internal

# Find WebSocket message handlers
rg -n "case.*:" excalidraw-be/internal/websocket
```

### Critical Entry Points
- Frontend app: `excalidraw-fe/src/App.tsx`
- Frontend stores: `excalidraw-fe/src/store/useWhiteboardStore.ts`, `useThemeStore.ts`
- Backend entry: `excalidraw-be/cmd/server/main.go`
- Backend room logic: `excalidraw-be/internal/room/room.go`

## Definition of Done

Before creating a PR:
- [ ] Frontend: `cd excalidraw-fe && npm run lint && npm run build`
- [ ] Backend: `cd excalidraw-be && make fmt && make lint && make test`
- [ ] Both services build successfully
- [ ] No TypeScript errors in frontend
- [ ] Docker containers start without errors: `make dev`
- [ ] Documentation updated if adding new features
