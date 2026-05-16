# Agentic AI - Chat to Diagram Feature

**Created:** 2026-05-13  
**Status:** ✅ **IMPLEMENTED & FIXED** (2026-05-14)

---

## Overview

Chat panel yang memungkinkan user berinteraksi dengan AI untuk:
- Generate diagram baru dari deskripsi natural language
- Edit/modify diagram yang sudah ada di canvas
- AI bisa "melihat" current canvas state sebagai context

---

## 🔧 Bug Fix Applied (2026-05-14)

**Issue:** The `/api/ai/*` routes were not being registered properly.

**Root Cause:** The `r.Route("/api/ai", ...)` pattern wasn't matching as expected with chi router when using Group inside RegisterRoutes.

**Fix:** Changed the route registration in `cmd/server/main.go` from:
```go
aiHandler.RegisterRoutes(r)  // Routes were: /chat, /models, /health
```

To:
```go
r.Route("/api/ai", func(r chi.Router) {
    aiHandler.RegisterRoutes(r)
})
```

**Files Modified:**
- `excalidraw-be/cmd/server/main.go` - Fixed route registration

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Frontend                                    │
│                                                                       │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────────────────┐ │
│  │  AIChatPanel │───►│  aiService   │───►│  excalidrawAPI         │ │
│  │  (React)     │    │  (FE client) │    │  .updateScene()        │ │
│  └──────────────┘    └──────┬───────┘    └────────────────────────┘ │
│                              │                                        │
└──────────────────────────────┼────────────────────────────────────────┘
                               │ HTTP/SSE (streaming)
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           Backend (Go)                                │
│                                                                       │
│  ┌──────────────────┐    ┌─────────────────────────────────────┐    │
│  │  /api/ai/chat    │───►│  AI Provider (OpenAI/Zhipu)          │    │
│  │  (SSE stream)    │    │  + MCP Tools                         │    │
│  └────────┬─────────┘    └─────────────────────────────────────┘    │
│           │                                                         │
│           ▼                                                         │
│  ┌──────────────────┐                                              │
│  │  LLM Provider    │                                              │
│  │  (OpenAI /       │                                              │
│  │   Anthropic /    │                                              │
│  │   Zhipu)         │                                              │
│  └──────────────────┘                                              │
└─────────────────────────────────────────────────────────────────────┘
```

---

## ✅ Implemented Features

### Backend (Go)

| Feature | File | Status |
|---------|------|--------|
| SSE streaming handler | `internal/ai/handler.go` | ✅ |
| OpenAI provider | `internal/ai/openai.go` | ✅ |
| Anthropic provider | `internal/ai/anthropic.go` | ✅ |
| MCP tool definitions | `internal/ai/provider.go` | ✅ |
| Canvas context | `internal/ai/handler.go` | ✅ |
| Tool execution | `AIChatPanel.tsx` | ✅ |
| Configuration | `internal/config/config.go` | ✅ |
| Route registration | `cmd/server/main.go` | ✅ (fixed) |

### Frontend (React)

| Feature | File | Status |
|---------|------|--------|
| Chat Panel UI | `src/components/ai/AIChatPanel.tsx` | ✅ |
| AI Service (SSE) | `src/services/ai/aiService.ts` | ✅ |
| AI Store (Zustand) | `src/store/useAIChatStore.ts` | ✅ |
| Element generators | `AIChatPanel.tsx` | ✅ |
| Streaming text | `AIChatPanel.tsx` | ✅ |
| Stop generation | `AIChatPanel.tsx` | ✅ |
| Suggested prompts | `AIChatPanel.tsx` | ✅ |

---

## MCP Tools Implemented

### Shape Creation
```typescript
create_rectangle  // Args: x, y, width, height, label?, strokeColor?, backgroundColor?, fillStyle?, strokeWidth?, roughness?, opacity?, roundness?
create_ellipse     // Args: x, y, width, height, label?, strokeColor?, backgroundColor?, fillStyle?, strokeWidth?, roughness?, opacity?
create_diamond     // Args: x, y, width, height, label?, strokeColor?, backgroundColor?, fillStyle?, strokeWidth?, roughness?, opacity?
create_text        // Args: x, y, text, fontSize?, strokeColor?
create_arrow       // Args: startX, startY, endX, endY, label?, strokeColor?, strokeWidth?, strokeStyle?, startArrowhead?, endArrowhead?, startBinding?, endBinding?
create_line        // Args: points, strokeColor?, strokeWidth?, strokeStyle?, startArrowhead?, endArrowhead?
```

### Zones & Layout
```typescript
create_zone        // Args: x, y, width, height, label?, strokeColor?, backgroundColor?, opacity?
camera_update      // Args: x, y, width, height — controls the viewport camera
```

### Element Modification
```typescript
move_elements      // Args: elementIds, deltaX, deltaY
delete_elements    // Args: elementIds
update_element_style // Args: elementIds, backgroundColor?, strokeColor?, strokeWidth?, opacity?
```

### Context
```typescript
get_canvas_state   // Returns: elementCount, types, bounding box
```

---

## Configuration

### Environment Variables (`excalidraw-be/.env`)

```bash
# AI Configuration
EXCALIDRAW_AI_PROVIDER=openai          # "openai" | "anthropic"
EXCALIDRAW_AI_API_KEY=sk-...           # Your API key
EXCALIDRAW_AI_MODEL=glm-4.7           # Model (gpt-4o, glm-4.7, claude-sonnet-4-20250514, etc.)
EXCALIDRAW_AI_BASE_URL=                # Optional: Custom endpoint for OpenAI-compatible APIs
EXCALIDRAW_AI_MAX_TOKENS=100000        # Max response tokens
EXCALIDRAW_AI_TEMPERATURE=0.7          # Creativity (0-1)
```

### Supported Models

| Provider | Models |
|----------|--------|
| OpenAI | `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `gpt-4` |
| Anthropic | `claude-sonnet-4-20250514`, `claude-3-5-sonnet-20241022`, `claude-3-5-haiku-20241022` |
| Zhipu (GLM) | `glm-4.7`, `glm-4`, `glm-3` |
| OpenAI Compatible | Ollama, vLLM, Azure OpenAI, Groq, Together AI |

---

## API Endpoints

### POST `/api/ai/chat`
SSE streaming endpoint for AI chat.

**Request:**
```json
{
  "message": "Buatkan flowchart login sederhana",
  "canvasContext": {
    "elements": [...],
    "activeFileId": "...",
    "activeTabId": "...",
    "roomId": "..."
  },
  "model": "glm-4.7"
}
```

**Response (SSE Events):**
```json
// Text streaming
{"type": "text", "content": "Saya akan membuat flowchart..."}

// Tool call
{"type": "tool_call", "id": "...", "name": "create_rectangle", "arguments": {...}}

// Done
{"type": "done", "summary": "..."}
```

### GET `/api/ai/models`
Returns available models for the configured provider.

**Response:**
```json
{
  "models": ["gpt-4o", "gpt-4o-mini", "claude-sonnet-4-20250514"]
}
```

### GET `/api/ai/health`
Health check for AI service.

**Response:**
```json
{
  "status": "ok",
  "models": ["gpt-4o", "gpt-4o-mini", "claude-sonnet-4-20250514"]
}
```

---

## Usage Flow

1. **User clicks AI Assistant button** (bottom-right floating button)
2. **Chat Panel opens** with suggested prompts
3. **User types message** (e.g., "Buatkan flowchart login")
4. **Backend streams response** via SSE:
   - Text messages
   - Tool calls → Elements added to canvas in real-time
5. **Canvas updates** as AI creates elements
6. **User can stop** generation mid-stream

---

## Build Verification

- ✅ Backend: `go build ./cmd/server` passes
- ✅ Frontend: `npm run build` passes
- ✅ No TypeScript errors
- ✅ No Go compilation errors

---

## Next Steps (Optional Enhancements)

- [ ] **Mermaid to Excalidraw** - Convert Mermaid diagrams
- [ ] **Layout helpers** - Auto-layout flowchart/grid
- [x] **AI model selector** - UI to switch models ✅ (Sprint 1)
- [ ] **Conversation export** - Save chat history
- [ ] **Image generation** - Generate and embed images
- [ ] **Text-to-diagram** - Better diagram interpretation
- [ ] **Rate limiting** - Protect AI endpoints from abuse ✅ (Sprint 1)

---

## Related Documents

- [Excalidraw MCP App](https://github.com/antonpk1/excalidraw-mcp-app) - Reference implementation
- [@excalidraw/mermaid-to-excalidraw](https://www.npmjs.com/package/@excalidraw/mermaid-to-excalidraw) - Mermaid conversion