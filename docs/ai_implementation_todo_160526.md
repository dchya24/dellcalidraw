# AI Implementation TODO

**Created:** 2026-05-16
**Last Updated:** 2026-05-31
**Source:** Analysis of `AI_CHAT_DIAGRAM.md`, `excalidraw-mcp.md`, `excalidraw_readme.md`

---

## Legend

| Symbol | Status | Description |
|--------|--------|-------------|
| ✅ | **Done** | Fully implemented, tested, and working |
| 🔧 | **Working** | Partially implemented or needs refinement |
| ❌ | **Not Started** | No implementation exists yet |
| 📝 | **Docs Only** | Type definitions or documentation exist, no working code |

---

## 1. Backend — AI Chat Core

| # | Feature | Status | File(s) | Notes |
|---|---------|--------|---------|-------|
| 1.1 | SSE streaming handler | ✅ | `internal/ai/handler.go` | `HandleChat`, `HandleModels`, `HandleHealth` |
| 1.2 | OpenAI provider (chat + stream) | ✅ | `internal/ai/openai.go` | Supports OpenAI, Zhipu, and compatible APIs |
| 1.3 | Anthropic provider (chat + stream) | ✅ | `internal/ai/anthropic.go` | Claude models supported |
| 1.4 | Route registration (`/api/ai/*`) | ✅ | `cmd/server/main.go` | Bug fix applied on 2026-05-14 |
| 1.5 | AI config from env vars | ✅ | `internal/config/config.go` | Provider, key, model, base URL, tokens, temp |
| 1.6 | Graceful fallback (AI disabled) | ✅ | `cmd/server/main.go` | `if cfg.AI.APIKey != ""` guard |
| 1.7 | Gemini / Google provider | ❌ | — | Not implemented |
| 1.8 | Multi-provider rotation / fallback | ❌ | — | Failover between providers |

---

## 2. Backend — MCP Tools

| # | Tool Name | Status | Notes |
|---|-----------|--------|-------|
| 2.1 | `create_rectangle` | ✅ | |
| 2.2 | `create_ellipse` | ✅ | |
| 2.3 | `create_diamond` | ✅ | |
| 2.4 | `create_text` | ✅ | |
| 2.5 | `create_arrow` | ✅ | With bindings support |
| 2.6 | `create_line` | ✅ | |
| 2.7 | `create_zone` | ✅ | Grouping/background zone |
| 2.8 | `move_elements` | ✅ | |
| 2.9 | `delete_elements` | ✅ | |
| 2.10 | `update_element_style` | ✅ | |
| 2.11 | `camera_update` | ✅ | Viewport control |
| 2.12 | `get_canvas_state` | ✅ | Returns element count, types, bounds |
| 2.13 | `create_group` | ❌ | Grouping elements together |
| 2.14 | `duplicate_elements` | ❌ | Clone existing elements |
| 2.15 | `resize_elements` | ❌ | Change width/height of existing elements |
| 2.16 | `edit_text` | ✅ | `internal/ai/provider.go` — changes text content of existing text/labeled element (Sprint 2) |
| 2.17 | `align_elements` | ❌ | Auto-align (left, center, right, top, bottom) |
| 2.18 | `auto_layout` | ✅ | `internal/ai/provider.go` (tool def) + `AIChatPanel.tsx::applyAutoLayout` — vertical / horizontal / grid (Sprint 3) |

---

## 3. Frontend — AI Chat Panel

| # | Feature | Status | File(s) | Notes |
|---|---------|--------|---------|-------|
| 3.1 | Chat panel UI | ✅ | `components/ai/AIChatPanel.tsx` | 31KB, full panel |
| 3.2 | SSE streaming consumption | ✅ | `services/ai/aiService.ts` | EventSource-based |
| 3.3 | AI Chat store (Zustand) | ✅ | `store/useAIChatStore.ts` | Conversations per tab |
| 3.4 | Real-time element creation | ✅ | `AIChatPanel.tsx` | Elements added on tool_call events |
| 3.5 | Stop generation button | ✅ | `AIChatPanel.tsx` | AbortController |
| 3.6 | Suggested prompts | ✅ | `AIChatPanel.tsx` | Pre-built prompt suggestions |
| 3.7 | Tool call badges & summary | ✅ | `AIChatPanel.tsx` | Visual feedback for executed tools |
| 3.8 | AI type definitions | ✅ | `types/ai.ts` | 140 lines |
| 3.9 | Conversation history persistence | ✅ | `store/useAIChatStore.ts` | localStorage via Zustand persist (Sprint 2). Pruned: max 20 conversations, 100 messages each |
| 3.10 | Multi-tab conversation support | ✅ | `useAIChatStore.ts` | Conversations keyed per tab, validated in Sprint 2 |
| 3.11 | Chat panel resize / draggable | ❌ | — | Fixed panel size |
| 3.12 | Markdown rendering in chat | ❌ | — | Plain text only |

---

## 4. Frontend — AI Model & Provider Settings

| # | Feature | Status | File(s) | Notes |
|---|---------|--------|---------|-------|
| 4.1 | Model selector UI | ✅ | `components/ai/AIChatPanel.tsx` | Dropdown consumes `/api/ai/models` + `/api/ai/health` (Sprint 1) |
| 4.2 | Provider selector UI | ❌ | — | Provider tied to backend env; no runtime switch |
| 4.3 | Temperature slider UI | ❌ | — | Config via env only |
| 4.4 | API key input UI | ❌ | — | Config via env only (server-managed, intentional) |
| 4.5 | Custom base URL UI | ❌ | — | Config via env only |
| 4.6 | Model list endpoint | ✅ | `GET /api/ai/models` | Consumed by FE model selector |

---

## 5. Mermaid Integration

| # | Feature | Status | File(s) | Notes |
|---|---------|--------|---------|-------|
| 5.1 | Type definitions | ✅ | `types/ai.ts` | `MermaidToExcalidrawParams` interface |
| 5.2 | `@excalidraw/mermaid-to-excalidraw` integration | ✅ | `package.json`, `AIChatPanel.tsx::applyConvertMermaid` | Lazy-loaded dynamic import (Sprint 3) |
| 5.3 | Mermaid parsing tool (backend) | ✅ | `internal/ai/provider.go` — `convert_mermaid` tool definition (Sprint 3) |
| 5.4 | Mermaid import UI | 🔧 | — | AI invokes the tool via prompt; no dedicated paste-Mermaid dialog yet |

---

## 6. Excalidraw MCP App Integration

> Ref: `docs/excalidraw-mcp.md` — external project reference

| # | Feature | Status | File(s) | Notes |
|---|---------|--------|---------|-------|
| 6.1 | MCP Server mode (remote/local) | ❌ | — | App doesn't expose MCP protocol |
| 6.2 | MCP App iframe rendering | ❌ | — | No iframe-based diagram rendering |
| 6.3 | `create_view` tool | ❌ | — | Different from local tools; streams JSON to MCP client |
| 6.4 | Checkpoint/restore system | ❌ | — | `restoreCheckpoint` not implemented |
| 6.5 | Progressive camera streaming | 🔧 | `provider.go` | `camera_update` adapted from this concept |
| 6.6 | Element format compliance | ✅ | `provider.go` | Follows spec from `excalidraw_readme.md` |

---

## 7. Excalidraw Element Format Spec (from `excalidraw_readme.md`)

| # | Spec Feature | Status | Notes |
|---|-------------|--------|-------|
| 7.1 | Basic elements (rect, ellipse, diamond, text, arrow) | ✅ | All supported |
| 7.2 | Labeled shapes (auto-center text) | ✅ | Via `label` property in tools |
| 7.3 | Arrow bindings (`fixedPoint`) | ✅ | In `create_arrow` tool |
| 7.4 | Color palette (primary + fills) | ✅ | Hardcoded in tool descriptions |
| 7.5 | `cameraUpdate` pseudo-element | ✅ | `camera_update` tool |
| 7.6 | `delete` pseudo-element | ✅ | `delete_elements` tool |
| 7.7 | Drawing order (progressive emit) | ✅ | Instructed in system prompt |
| 7.8 | `restoreCheckpoint` | ❌ | No checkpoint system |
| 7.9 | Animation mode (delete + recreate in-place) | ❌ | No automatic animation support |
| 7.10 | Dark mode diagrams | ❌ | No dark background zone tool |
| 7.11 | Font size rules (min 14-16) | 🔧 | In prompt instructions, not enforced |
| 7.12 | 4:3 camera aspect ratio enforcement | 🔧 | In prompt instructions, not validated |

---

## 8. Enhancements & Quality of Life

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| 8.1 | Conversation export (JSON/Markdown) | ❌ | |
| 8.2 | Chat history to database persistence | ❌ | In-memory only |
| 8.3 | Undo AI-generated elements (batch undo) | ✅ | `AIChatPanel.tsx` — "Undo" button on assistant message removes batch via tracked `createdElementIds` (Sprint 2) |
| 8.4 | AI-generated element highlighting | ❌ | No visual distinction from user elements |
| 8.5 | Image generation & embed | ❌ | |
| 8.6 | Multi-language prompt support | 🔧 | System prompt partially in English/Indonesian |
| 8.7 | Rate limiting on AI endpoints | ✅ | `internal/middleware/ratelimit.go` — IP-based, 2 req/s, burst 6 (Sprint 1) |
| 8.8 | Token usage tracking & display | ✅ | OpenAI: `stream_options.include_usage`. Anthropic: `message_start`/`message_delta`. Backend emits `usage` SSE event, FE displays per-message badge + session total in panel header (Sprint 3) |
| 8.9 | Error recovery (retry failed generations) | ❌ | |
| 8.10 | Context window management | ❌ | No summarization of long conversations |

---

## 9. Documentation Updates Needed

| # | Task | Status | Notes |
|---|------|--------|-------|
| 9.1 | Update `AI_CHAT_DIAGRAM.md` tools list | ✅ | Synced 2026-05-31: now lists all 13 tools incl. `edit_text`, `create_zone`, `camera_update` |
| 9.2 | Add MCP App integration guide | ❌ | `excalidraw-mcp.md` is external README only |
| 9.3 | Document `excalidraw_readme.md` adaptation status | ❌ | No mapping of spec → implementation |
| 9.4 | Add AI troubleshooting guide | ❌ | Common errors, debug tips |
| 9.5 | Add AI feature user guide | ❌ | End-user documentation |

---

## Priority Recommendations

### 🔴 High Priority (Core Experience)
1. **#2.13-2.17** — Additional MCP tools (group, duplicate, resize, align)
2. **#3.12** — Markdown rendering in chat panel

### 🟡 Medium Priority (Usability)
3. **#7.8-7.10** — Checkpoint, animation, dark mode support

### 🟢 Low Priority (Nice to Have)
4. **#6.1-6.4** — Full MCP App server mode
5. **#1.7-1.8** — Additional providers (Gemini, rotation)
6. **#8.5** — Image generation
7. **#3.11** — Panel resize / draggable
8. **#4.2-4.5** — In-app provider/temperature/base URL UI (server-managed today)

---

## Sprint Plan (Impact vs Effort)

> Pendekatan: maksimalkan value dengan effort minimal. Sprint diurutkan dari quick wins ke game changers.

### Sprint 1 — Quick Wins (est. ~1 hari)

| # | Item | Effort | Impact | Alasan |
|---|------|--------|--------|--------|
| 9.1 | Update docs tools list (10 → 12) | 30 min | Tinggi | Doc salah saat ini, risk bikin developer lain bingung |
| 8.7 | Rate limiting AI endpoints | 30 min | Tinggi | Chi middleware standar. Tanpa ini, 1 user bisa burn seluruh API quota |
| 4.1 | Model selector dropdown UI | 2 jam | Tinggi | Backend `GET /api/ai/models` **sudah jadi**, cuma butuh dropdown di FE |

**Deliverables:**
- [x] `AI_CHAT_DIAGRAM.md` updated dengan 12 tools
- [x] Rate limiter middleware di `internal/middleware/ratelimit.go` + dipasang di `cmd/server/main.go` (2 req/s, burst 6 per IP)
- [x] Model selector dropdown di `AIChatPanel.tsx` yang consume `/api/ai/models` + `/api/ai/health`
- [x] Backend `HandleChat` sekarang menerima `model` dari request (validasi terhadap provider models)
- [x] Frontend `aiService.ts` mengirim `model` ke backend

---

### Sprint 2 — Medium ROI (est. ~1.5 hari)

| # | Item | Effort | Impact | Alasan |
|---|------|--------|--------|--------|
| 2.16 | `edit_text` MCP tool | 1 jam | Tinggi | Tool paling sering diminta ("ubah label ini", "ganti teks"). Mirip `update_element_style`, ~50 lines di `provider.go` |
| 3.9 | Conversation persistence (localStorage) | 1 jam | Tinggi | Refresh = hilang semua chat. Persist ke localStorage dulu tanpa DB |
| 8.3 | Batch undo AI elements | 3 jam | Tinggi | Saat AI generate 15 elements dan jelek, user undo 15x. Satu tombol hapus batch. Data `toolCalls` sudah ada di store |

**Deliverables:**
- [x] Tool `edit_text` di `internal/ai/provider.go` + handler `applyEditText` di `AIChatPanel.tsx`
- [x] Type `EditTextParams` di `types/ai.ts` + `edit_text` di `TOOL_META`
- [x] `createdElementIds` field di `ChatMessage` type + tracking di tool_call handler
- [x] Tombol "Undo" di tool badges assistant message yang hapus batch elemen
- [x] localStorage persistence pruning (max 20 conversations, max 100 messages/conversation)

---

### Sprint 3 — Game Changers (est. ~3-4 hari) — **DONE 2026-05-31**

| # | Item | Effort | Impact | Alasan |
|---|------|--------|--------|--------|
| 5.2-5.3 | Mermaid → Excalidraw converter | 1.5 hari | Sangat tinggi | Package `@excalidraw/mermaid-to-excalidraw` sudah ada di npm. Unlock: paste Mermaid → langsung jadi diagram |
| 2.18 | `auto_layout` tool | 1.5 hari | Tinggi | AI sering generate posisi berantakan. Auto-arrange jadi grid/flowchart layout via topological sort |
| 8.8 | Token usage tracking & display | 0.5 hari | Medium | Counter di SSE response + UI display. Penting untuk user dengan API berbayar |

**Deliverables:**
- [x] `@excalidraw/mermaid-to-excalidraw@2.2.2` installed (lazy-loaded), `convert_mermaid` MCP tool di backend, `applyConvertMermaid` di FE
- [x] Tool `auto_layout` di `provider.go` (vertical / horizontal / grid) + `applyAutoLayout` di FE yang memindahkan elemen sambil menjaga binding
- [x] Token counter di SSE stream (`usage` event) untuk OpenAI (`stream_options.include_usage`) dan Anthropic (`message_start` + `message_delta`). FE menampilkan badge per-pesan dan total per session di header.

---

### Explicitly Deferred (effort tinggi, value belum sebanding)

| Item | Kenapa skip dulu |
|------|------------------|
| MCP App server mode (#6.1-6.4) | Butuh arsitektur baru (iframe, MCP protocol). Belum ada use case jelas |
| Gemini provider (#1.7) | OpenAI-compatible sudah cover Zhipu, Groq, Ollama — cukup |
| Dark mode diagrams (#7.10) | Nice to have, minor. User bisa manual set background |
| Animation mode (#7.9) | Kompleks, value utamanya cuma demo/showcase |
| Chat panel draggable (#3.11) | UX polish, bukan core feature |
| Multi-provider rotation (#1.8) | Butuh abstraction layer lebih dalam |
| Checkpoint/restore (#7.8) | Depends on MCP App integration (#6) |
| Image generation (#8.5) | Butuh integration dengan image API (DALL-E, dll) |
| Conversation export (#8.1) | Nice to have, bisa pakai copy-paste dulu |
