# Pending Features Backlog

> **Generated:** 2026-05-31
> **Branch:** `dev`
> **Scope:** Single source of truth for outstanding work outside the
> three areas the user has explicitly steered away from: production
> monitoring, automated testing expansion, and encryption-related work.

For roadmap context (overall progress, completed phases) see
`PROJECT_STATUS.md`. This file lists what's left and groups it by
where the value lives.

---

## Quick reference

| Track                       | Items | Suggested next |
| --------------------------- | :---: | :------------- |
| Persistence & Data          | 4     | Chat history → DB |
| AI Feature                  | 8     | Conversation export |
| Auth & User Management      | 4     | Password reset email delivery |
| Files & Sharing             | 1     | Per-file sharing UI |
| Collaboration polish        | 3     | Idle detection (deferred) |
| MCP App integration         | 4     | (low priority) |
| HTTP & Security follow-ups  | 2     | CSRF protection |

---

## Persistence & Data

### 1. Chat history persistence to database 🟡
**Why:** AI conversations live in `localStorage` only (max 20
conversations × 100 messages). Users lose history on device switch
or browser clear. Cross-device continuity is a common ask.

**Effort:** 1–2 days
**Files:**
- BE: new migration `ai_conversations` (id, user_id, tab_id, created_at, messages JSONB)
- BE: handlers under `/api/ai/conversations` (list / get / save / delete)
- FE: `useAIChatStore` checks auth, falls back to localStorage when guest
- FE: sync local conversations on login (similar to file tabs migration)

**Notes:** Should respect the 20×100 limit when persisting. Consider
encryption at rest for AI logs if turning this on (follow-up to file
encryption track which the user has parked).

### 2. File versioning / history 🟡
**Why:** Save = overwrite today. No way to recover an earlier state of
a file. Common feature in design tools.

**Effort:** 2–3 days
**Files:**
- BE: new migration `file_versions` (id, file_id, version, snapshot JSONB, created_at, label)
- BE: handlers `GET /api/files/{fileId}/versions`, `POST /api/files/{fileId}/versions/{versionId}/restore`
- BE: snapshot strategy — every N saves OR every M minutes, configurable; trim to last 50
- FE: "History" panel in MainMenu showing versions, click-to-restore
- FE: Optional version label / autosave indicator

**Notes:** Storage cost grows linearly with edits. Throttle snapshots
and prune. Pair with backups (already done) for full recovery story.

### 3. Account deletion / data export (GDPR) 🟢
**Why:** Right-to-be-forgotten. Required if EU users in scope.

**Effort:** 1–1.5 days
**Files:**
- BE: `DELETE /api/account` cascades through user_files, file_tabs,
  ai_request_logs, refresh_tokens, room_members; soft-delete or hard-delete TBD
- BE: `GET /api/account/export` returns a JSON archive of all user data
- FE: Settings → "Delete account" + "Export my data" buttons with confirmations

**Notes:** Soft delete preferred initially (keeps room collaboration
attribution sane); hard delete via cron after 30-day grace period.

### 4. HTTP rate limiting on remaining endpoints 🟢
**Why:** AI and auth endpoints are rate-limited; file management,
canvas save/load, and rooms still aren't. Spam-protection gap.

**Effort:** 0.5 day
**Files:**
- BE: extend `cmd/server/main.go` with a general-purpose limiter on
  authenticated `/api/files/*` and `/api/rooms/*/canvas/*`
- BE: per-user limit (not per-IP) for authenticated routes — needs
  small middleware variant pulling user ID from JWT context

**Notes:** Token bucket already exists. Tune limits per route based
on expected behavior (e.g. rooms canvas save can burst during paste
operations).

---

## AI Feature

> Source: `docs/ai_implementation_todo_160526.md` items not yet ✅.

### 5. Conversation export (JSON / Markdown) 🟢
**Why:** Users currently rely on copy-paste. Common request.
**Effort:** 0.5 day
**Files:** `AIChatPanel.tsx` — export button → `Blob` download.
Markdown serializer maps tool calls back to rendered diagram description.

### 6. Image generation & embed in diagrams 🟢
**Why:** AI generates diagrams from text but can't generate or embed
raster images. Adds icon/illustration use cases.
**Effort:** 2–3 days
**Files:**
- BE: new `generate_image` tool → DALL-E / Stable Diffusion API
- BE: store result in S3 (use existing storage layer)
- FE: handle as `image` element with the returned blob URL

**Risk:** Cost. Add per-user image-generation rate limit.

### 7. Error recovery / retry failed AI generations 🟢
**Why:** Network blip mid-stream loses the response. User has to
retype prompt.
**Effort:** 0.5 day
**Files:** `aiService.ts` — exponential backoff retry on 5xx /
network errors, capped at 3 attempts. Keep AbortController so user
can cancel.

### 8. Context window management (long conversation summarization) 🟢
**Why:** Long sessions hit model context limit. The current pruning
(last 100 messages) is blunt.
**Effort:** 1–1.5 days
**Files:**
- BE: when message count > N, summarize older messages with a
  cheaper model (e.g. gpt-4o-mini) and replace them with a system
  message containing the summary
- FE: hint indicator that conversation has been summarized

### 9. Checkpoint / restore system 🟢
**Why:** Spec'd in `docs/excalidraw_readme.md`. Lets AI bookmark
states and revert if its changes are unwanted.
**Effort:** 1 day
**Files:** new `create_checkpoint` and `restore_checkpoint` tools;
in-memory or per-tab persistent storage.

### 10. Animation mode (delete + recreate in-place) 🟢
**Why:** Diagram-as-presentation showcase. Low impact for everyday
use, but visually striking.
**Effort:** 1 day
**Files:** new `animate` tool that pulses, fades, or rebuilds
elements over a timeline.

### 11. Dark-mode-aware diagrams 🟢
**Why:** AI doesn't currently consider whether the canvas is dark
when picking colors.
**Effort:** 0.5 day
**Files:** `BuildSystemPrompt` — include current theme in canvas
context; document dark palette in the prompt.

### 12. AI provider rotation / fallback 🟢
**Why:** Single provider down = AI offline. Multi-provider failover
adds resilience.
**Effort:** 1 day
**Files:** new wrapper `LLMProvider` that holds a primary +
secondary and retries on failure. Out of scope: Gemini support
itself (user said current providers are enough).

---

## Auth & User Management

### 13. Password reset email delivery 🟡 ✅ DONE 2026-05-31
**Why:** Backend creates the reset token and `slog.Info`s the URL —
no email is actually sent. Production-blocker for password reset.

**Effort:** 1 day
**Files:**
- BE: `internal/email/` package with sender interface, SMTP impl
  (gomail) and SES impl
- BE: config `EMAIL_PROVIDER`, `EMAIL_FROM`, `SMTP_HOST`, etc.
- BE: wire into `ForgotPassword` handler at `auth_handlers.go:402`
- BE: HTML + plain text templates

**Notes:** Defer to dev-mode log fallback when no provider is
configured (current behavior). Same email path will be reused for
invitation emails (#15) and account deletion confirmations.

**Resolution:** Landed in `feat(email): SMTP-based transactional email
for password reset`. New `internal/email/` package with `Sender`
interface, `LogSender` (dev fallback), and `SMTPSender` (net/smtp
+ STARTTLS). Templates for password reset and room invitations
(reused for #15) live in `templates.go`. Config under
`EXCALIDRAW_EMAIL_*`; `auth_handlers.go::ForgotPassword` now sends
actual email. Compose + .env.example updated. 12 unit tests cover
fallback selection, MIME shape, address parsing, template content,
and HTML escaping.

### 14. Two-factor authentication (TOTP) 🟢
**Why:** Account hardening. Standard expectation for SaaS.
**Effort:** 2 days
**Files:**
- BE: `internal/auth` — TOTP secret generation, verification,
  backup codes
- BE: handlers `/api/auth/2fa/{enable,verify,disable}`
- BE: migration adding `users.totp_secret`, `users.totp_enabled`,
  `users.backup_codes_hash`
- FE: settings panel with QR enrollment, verify-on-login flow

### 15. Email-based room invitations (delivery) 🟡 partial — sender ready
**Why:** `room_invitations` table exists, `CreateInvitation` returns
an invite URL, but no email is sent. Same gap as #13.
**Effort:** 0.5 day (after #13)
**Files:** plug `internal/email` into the invitation handler.
**Status:** `email.RoomInvitationMessage` template is shipped; just
needs the WS / HTTP invitation handler to call `email.Send` with it.

### 16. Profile management UI polish 🟢
**Why:** Backend has `UpdateUserProfile` (username + avatar) but I
haven't seen a settings page in the FE. Worth checking and shipping
if missing.
**Effort:** 0.5 day to verify, 1 day to build if absent
**Files:** FE settings modal with username/avatar edit, password
change form.

---

## Files & Sharing

### 17. Per-file sharing UI 🟡
**Why:** Backend has `room_permissions` table and handlers, plus
`RoomSettingsPanel.tsx` exists for room-level sharing. But user
files (the multi-tab files in `user_files` / `file_tabs`) don't
have a sharing UI — only realtime collaboration rooms do.

**Effort:** 1.5 days
**Files:**
- BE: new `file_collaborators` migration mirroring `room_members`
- BE: handlers `POST /api/files/{fileId}/share`, list/revoke
- BE: `JWTMiddleware` extension to also accept `?shareToken=` for
  read-only public links
- FE: "Share" entry in MainMenu (per file), invite-by-email dialog,
  public-link toggle

**Notes:** Rooms ≠ files in this app. Worth confirming with the
user whether per-file sharing is desired or whether
"open file → click Share to spawn a room" is the intended UX.

---

## Collaboration polish (deferred per current focus)

### 18. Idle detection 🟢
**Why:** Users appear "online" forever once connected. Idle
indicator (e.g. 5min no activity) gives others useful signal.
**Effort:** 1–2 days
**Files:** `cursorService` tracks last-activity timestamp,
broadcasts `idle: true` on threshold; FE renders dimmed cursor /
participant entry.

### 19. Viewport sync / "Follow User" mode 🟢
**Why:** Walkthroughs and presentations. One user "drives", others'
viewports follow.
**Effort:** 2–3 days
**Files:**
- New WS message `follow_request` / `viewport_update`
- FE menu "Follow {user}" toggle, applies the followed user's
  viewport on every frame

### 20. Operational transformation 🔴
**Why:** Last-write-wins drops simultaneous edits to the same
element silently.
**Effort:** 2 weeks calendar
**Status:** Detailed plan at
`docs/plans/2026-05-31-operational-transformation.md`. Plan
recommends parking until a replay harness, observability, and a Yjs
spike land first.

---

## MCP App integration (low priority)

> Source: `docs/excalidraw-mcp.md` (external app reference).

### 21. MCP Server mode (expose this app over MCP) 🟢
**Why:** Lets Claude Desktop / Cursor drive the canvas directly
without going through the chat panel.
**Effort:** 1 week
**Files:** new `internal/mcp/` package implementing the MCP
protocol; tool surface mirrors current AI tools but as MCP commands.

### 22. Iframe rendering for MCP App 🟢
**Effort:** 0.5 day
**Files:** add an `?embed=1` query param that hides chrome and
exposes a postMessage API for parent frames.

### 23. Progressive camera streaming 🟢
**Why:** Currently `camera_update` is one-shot. Streaming would
allow smooth pans during AI generation.
**Effort:** 1 day
**Files:** FE animates camera between successive `camera_update`
events instead of jumping.

### 24. View-only public link 🟢
**Why:** Share a snapshot URL that doesn't require auth.
**Effort:** 1 day
**Files:** `GET /api/files/public/{token}` returns elements + appState
without any mutation; FE renders read-only canvas.

---

## HTTP & Security follow-ups

### 25. CSRF protection 🟢
**Why:** Currently mitigated by CORS allow-list + bearer token in
`Authorization` header (so a CSRF attempt can't read the token).
But cookie-based flows would still be vulnerable if added later.
**Effort:** 0.5 day
**Files:** chi `csrf` middleware on state-changing endpoints; FE
fetches a token from a `/csrf` endpoint and echoes in `X-CSRF-Token`.

### 26. CORS allow-list narrowing 🟢
**Why:** Current default `EXCALIDRAW_CORS_ALLOWED_ORIGINS` allows
both `localhost:3000` and `localhost:5173`. Production should
narrow to the actual domain.
**Effort:** 15 minutes (config doc + DEPLOYMENT.md note).

---

## Out of scope (per user direction)

The following are explicitly parked or de-prioritised:

- Mobile / touch support
- Internationalization (multi-language)
- Production monitoring (Prometheus / Grafana / Sentry)
- File encryption at rest
- Additional automated testing (HTTP integration, E2E, load)
- WebSocket / collaboration end-to-end tests
- Gemini provider, additional LLM providers beyond OpenAI-compat + Anthropic

---

## Priority recommendation

Looking at this backlog with fresh eyes, my picks for "do next" — in
order of value-per-effort, not touching the user's parked tracks:

1. **#13 Password reset email** (1 day) — currently printf-debug, blocks
   any production use of the password reset flow.
2. **#5 Conversation export** (0.5 day) — quick win, frequently asked.
3. **#1 Chat history → DB** (1–2 days) — user-visible, isolates AI work
   from collaboration changes.
4. **#17 Per-file sharing** (1.5 days) — closes the gap between rooms
   (realtime) and files (persistent), confusing today.
5. **#2 File versioning** (2–3 days) — significant feature, pairs well
   with the backups work already shipped.

After these, the rest of this list is a coherent backlog you can pick
from based on user priority or available capacity.
