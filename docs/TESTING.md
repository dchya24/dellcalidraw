# Testing Guide

## What's covered

| Layer                  | Status | Tests                                                                                  |
| ---------------------- | :----: | -------------------------------------------------------------------------------------- |
| BE — `internal/crypto` | ✅     | AES-256-GCM seal/open round-trip, tamper detection, wire format                         |
| BE — `internal/auth`   | ✅     | Bcrypt hash/check, JWT generate/validate, expiry, tamper, refresh randomness            |
| BE — `internal/config` | ✅     | Defaults, env overrides, `IsDevelopment`, CORS list parsing                             |
| BE — `internal/middleware` | ✅ | Security headers (HSTS opt-in, CSP, etc.), per-IP rate limit (burst, isolation, replenish) |
| BE — `internal/ai`     | ✅     | System prompt content, 19-tool registry integrity, schema sanity                        |
| FE — `cryptoService`   | ✅     | WebCrypto round-trip with BE format, tamper detection, IV freshness                     |
| FE — `aiService`       | ✅     | SSE event parser for `text` / `tool_call` / `tool_result` / `usage` / `done` / `error`  |

## What's not covered (yet)

These require either a real DB / S3 (`testcontainers-go`) or refactoring
the handlers to depend on interfaces. Tracked but deferred:

- HTTP integration tests for `cmd/server` handlers (auth, file management,
  file tabs migration, AI chat).
- WebSocket end-to-end tests (collab feature, deferred per current focus).
- Frontend component tests for `AIChatPanel` (heavy DOM + Excalidraw API).
- E2E browser tests (Playwright/Cypress).

The pure-logic tests above cover the building blocks the handlers
compose, so a lot of the bug surface is already trapped — but they
won't catch wiring mistakes in `main.go` or the handler glue. Add
integration tests when scope allows.

## Running tests

### Backend

```bash
cd excalidraw-be

# All tests with race detector + coverage profile
go test -race -coverprofile=coverage.out ./...

# Single package
go test -v ./internal/crypto/...

# Coverage report (HTML)
go tool cover -html=coverage.out
```

### Frontend

```bash
cd excalidraw-fe

# CI mode (single run)
npm test

# Watch mode
npm run test:watch

# UI mode
npm run test:ui
```

## Adding tests

### Backend
- Place tests next to the code under test (`*_test.go` in the same package).
- Prefer table-driven tests for multi-case logic.
- Use `t.Setenv` to scope env mutations to a single test.
- Don't depend on the network or real services — those go in a
  separate `internal/<pkg>/integration` build tag if/when added.

### Frontend
- Tests live in `src/<area>/__tests__/<name>.test.ts` next to the code.
- `src/test/setup.ts` polyfills WebCrypto and registers
  `@testing-library/jest-dom` matchers.
- Prefer `vitest` globals (`describe`, `it`, `expect`) — already
  enabled in `tsconfig.app.json` types.
- For React components, use `@testing-library/react` (already a
  devDependency); we don't have component tests yet because the AI
  panel touches Excalidraw imperatively.

## CI

`.github/workflows/ci.yml` runs:

- Frontend: `npm run lint` → `tsc --noEmit` → `npm test` → `npm run build`
- Backend: `go fmt` (must be clean) → `go build` → `go test -race -coverprofile`

Coverage profile is uploaded as a GitHub Actions artifact (7-day
retention) for inspection.
