# Agentic AI - Chat to Diagram Feature

**Created:** 2026-05-13
**Status:** 📋 Planning
**Goal:** User bisa chat dengan AI untuk generate/edit diagram langsung di canvas

---

## Overview

Chat panel yang memungkinkan user berinteraksi dengan AI untuk:
- Generate diagram baru dari deskripsi natural language
- Edit/modify diagram yang sudah ada di canvas
- AI bisa "melihat" current canvas state sebagai context

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Frontend                                    │
│                                                                       │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────────────────┐ │
│  │  Chat Panel  │───►│  AI Service  │───►│  excalidrawAPI         │ │
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
│  │  /api/ai/chat    │───►│  MCP Server (Excalidraw Tools)      │    │
│  │  (proxy + tools) │    │                                     │    │
│  └────────┬─────────┘    │  Tools:                             │    │
│           │               │  - create_rectangle                 │    │
│           │               │  - create_diamond                   │    │
│           ▼               │  - create_ellipse                   │    │
│  ┌──────────────────┐    │  - create_text                      │    │
│  │  LLM Provider    │    │  - create_arrow                     │    │
│  │  (OpenAI /       │    │  - create_line                      │    │
│  │   Anthropic)     │    │  - delete_elements                  │    │
│  └──────────────────┘    │  - move_elements                    │    │
│                           │  - update_element_style             │    │
│                           │  - layout_flowchart                 │    │
│                           │  - layout_grid                      │    │
│                           │  - get_canvas_state                 │    │
│                           │  - mermaid_to_excalidraw            │    │
│                           └─────────────────────────────────────┘    │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## MCP (Model Context Protocol) Approach

Menggunakan MCP sebagai interface antara LLM dan Excalidraw canvas.

### Why MCP?

1. **Standardized** — Protocol yang sudah diadopsi luas untuk tool-calling
2. **Provider agnostic** — Bisa pakai OpenAI, Anthropic, atau model lain
3. **Existing packages** — Ada `excalidraw-mcp`, `@excalidraw/mermaid-to-excalidraw`
4. **Extensible** — Mudah tambah tools baru tanpa ubah LLM prompt
5. **Streaming support** — MCP mendukung streaming responses

### MCP Tools Definition

```typescript
// Core drawing tools
interface MCPTools {
  // Create elements
  create_rectangle: (params: {
    x: number; y: number;
    width: number; height: number;
    label?: string;
    backgroundColor?: string;
    strokeColor?: string;
  }) => ExcalidrawElement;

  create_diamond: (params: {
    x: number; y: number;
    width: number; height: number;
    label?: string;
  }) => ExcalidrawElement;

  create_ellipse: (params: {
    x: number; y: number;
    width: number; height: number;
    label?: string;
  }) => ExcalidrawElement;

  create_text: (params: {
    x: number; y: number;
    text: string;
    fontSize?: number;
  }) => ExcalidrawElement;

  create_arrow: (params: {
    startElementId?: string;
    endElementId?: string;
    startX?: number; startY?: number;
    endX?: number; endY?: number;
    label?: string;
  }) => ExcalidrawElement;

  create_line: (params: {
    points: [number, number][];
    strokeColor?: string;
  }) => ExcalidrawElement;

  // Modify elements
  move_elements: (params: {
    elementIds: string[];
    deltaX: number; deltaY: number;
  }) => void;

  delete_elements: (params: {
    elementIds: string[];
  }) => void;

  update_element_style: (params: {
    elementIds: string[];
    backgroundColor?: string;
    strokeColor?: string;
    strokeWidth?: number;
    fontSize?: number;
  }) => void;

  // Layout helpers
  layout_flowchart: (params: {
    direction: "TB" | "LR";
    nodes: { id: string; label: string; type: "rectangle" | "diamond" | "ellipse" }[];
    edges: { from: string; to: string; label?: string }[];
  }) => ExcalidrawElement[];

  layout_grid: (params: {
    elements: { label: string; type: string }[];
    columns: number;
    spacing?: number;
    startX?: number; startY?: number;
  }) => ExcalidrawElement[];

  // Context
  get_canvas_state: () => {
    elements: ExcalidrawElement[];
    elementCount: number;
    boundingBox: { x: number; y: number; width: number; height: number };
  };

  // Mermaid integration
  mermaid_to_excalidraw: (params: {
    syntax: string; // Mermaid diagram syntax
  }) => ExcalidrawElement[];
}
```

---

## LLM Provider Configuration

Backend sebagai proxy, support multiple providers:

```go
// Backend config
type AIConfig struct {
    Provider    string // "openai" | "anthropic"
    APIKey      string
    Model       string // "gpt-4o" | "claude-sonnet-4-20250514"
    BaseURL     string // Custom endpoint (OpenAI compatible)
    MaxTokens   int
    Temperature float64
}
```

### OpenAI Compatible
- OpenAI (gpt-4o, gpt-4o-mini)
- Azure OpenAI
- Local models via Ollama/vLLM (OpenAI compatible API)
- Groq, Together AI, etc.

### Anthropic Compatible
- Claude Sonnet, Opus, Haiku
- Via Anthropic API directly

---

## Streaming Flow

```
User: "Buatkan flowchart login"
    │
    ▼ POST /api/ai/chat (SSE stream)
    │
Backend:
    │── Send prompt + tools + canvas context to LLM
    │── LLM streams response:
    │     ├── text: "Saya akan membuat flowchart login..."
    │     ├── tool_call: create_rectangle("User Input", ...)
    │     ├── tool_call: create_rectangle("Validate", ...)
    │     ├── tool_call: create_diamond("Valid?", ...)
    │     ├── tool_call: create_arrow(...)
    │     └── text: "Flowchart login sudah dibuat dengan 5 nodes."
    │
    ▼ SSE events to frontend
    │
Frontend:
    ├── Show text in chat (streaming)
    ├── Execute tool results → updateScene() (elements appear one by one)
    └── Final message in chat
```

### SSE Event Format

```typescript
// Text chunk
{ type: "text", content: "Saya akan membuat..." }

// Tool call (element created)
{ type: "tool_result", tool: "create_rectangle", result: { element: ExcalidrawElement } }

// Batch elements (for layout tools)
{ type: "tool_result", tool: "layout_flowchart", result: { elements: ExcalidrawElement[] } }

// Done
{ type: "done", summary: "Created 5 elements" }

// Error
{ type: "error", message: "Failed to generate" }
```

---

## Frontend Components

### Chat Panel

```
┌─────────────────────────────┐
│  AI Assistant            [x] │
├─────────────────────────────┤
│                              │
│  🤖 Halo! Saya bisa bantu   │
│     membuat diagram.         │
│                              │
│  👤 Buatkan flowchart login  │
│     dengan validasi email    │
│                              │
│  🤖 Saya akan membuat...     │
│     ✅ Created "User Input"  │
│     ✅ Created "Validate"    │
│     ✅ Created "Valid?"      │
│     ✅ Created arrows        │
│     Done! 5 elements added.  │
│                              │
├─────────────────────────────┤
│  [Type message...]    [Send] │
└─────────────────────────────┘
```

### File Structure (FE)

```
src/
├── components/
│   └── AIChatPanel.tsx          # Chat UI component
├── services/
│   └── aiService.ts             # AI API client + SSE handling
├── store/
│   └── useAIChatStore.ts        # Chat history, loading state
└── types/
    └── ai.ts                    # AI-related types
```

### File Structure (BE)

```
excalidraw-be/
├── cmd/server/
│   └── ai_handlers.go          # HTTP handlers for /api/ai/*
├── internal/
│   ├── ai/
│   │   ├── provider.go         # LLM provider interface
│   │   ├── openai.go           # OpenAI implementation
│   │   ├── anthropic.go        # Anthropic implementation
│   │   ├── tools.go            # MCP tool definitions
│   │   └── stream.go           # SSE streaming logic
│   └── mcp/
│       ├── server.go           # MCP server implementation
│       ├── tools_draw.go       # Drawing tools
│       ├── tools_layout.go     # Layout tools
│       ├── tools_modify.go     # Modify/delete tools
│       └── tools_context.go    # Canvas context tools
```

---

## API Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/ai/chat` | ✅ | Send message, get SSE stream response |
| GET | `/api/ai/models` | ✅ | List available models |
| POST | `/api/ai/config` | ✅ | Update AI provider config (admin) |

### POST /api/ai/chat

**Request:**
```json
{
  "message": "Buatkan flowchart login",
  "canvasContext": {
    "elements": [...],
    "activeFileId": "...",
    "activeTabId": "..."
  },
  "conversationId": "conv-123",
  "model": "gpt-4o"
}
```

**Response:** SSE stream (see event format above)

---

## Context Awareness

AI bisa "melihat" canvas melalui:

1. **Canvas state dikirim di request** — FE kirim current elements sebagai context
2. **`get_canvas_state` tool** — LLM bisa panggil tool ini untuk inspect canvas
3. **Element references** — LLM bisa refer ke element by ID untuk modify

### Context Optimization

Canvas bisa besar (1000+ elements). Strategi:
- Kirim summary saja (element count, types, bounding box)
- LLM panggil `get_canvas_state` jika perlu detail
- Limit context: hanya kirim visible viewport elements
- Atau kirim element labels/text saja tanpa full coordinates

---

## Mermaid Integration

Leverage `@excalidraw/mermaid-to-excalidraw` untuk complex diagrams:

```
User: "Buatkan sequence diagram API call"

AI → tool_call: mermaid_to_excalidraw({
  syntax: `
    sequenceDiagram
      Client->>Server: POST /api/login
      Server->>DB: Query user
      DB-->>Server: User data
      Server-->>Client: JWT token
  `
})

→ Excalidraw elements rendered on canvas
```

Ini powerful karena:
- Mermaid syntax lebih mudah di-generate LLM
- Support: flowchart, sequence, class, ER, gantt, pie, mindmap
- `@excalidraw/mermaid-to-excalidraw` sudah handle layout

---

## Implementation Phases

### Phase 1: Basic Chat + Simple Generation (MVP)
- [ ] Chat panel UI (FE)
- [ ] AI service + SSE client (FE)
- [ ] Backend proxy endpoint (`/api/ai/chat`)
- [ ] OpenAI provider implementation
- [ ] Basic tools: `create_rectangle`, `create_text`, `create_arrow`
- [ ] Canvas context in request

### Phase 2: Full Tool Set + Streaming
- [ ] All drawing tools (diamond, ellipse, line)
- [ ] Layout tools (flowchart, grid)
- [ ] Streaming elements to canvas (appear one by one)
- [ ] Modify tools (move, delete, style)
- [ ] Anthropic provider

### Phase 3: Mermaid + Advanced
- [ ] Mermaid-to-Excalidraw integration
- [ ] Complex diagram generation (sequence, ER, class)
- [ ] Edit existing diagrams via chat
- [ ] Conversation history persistence
- [ ] Model selection UI

### Phase 4: Polish
- [ ] Undo AI changes (revert last AI action)
- [ ] Suggested prompts / templates
- [ ] Cost tracking / rate limiting
- [ ] Multi-language support
- [ ] Keyboard shortcut to open chat (Ctrl+Shift+A)

---

## Environment Variables

```env
# Backend (.env)
AI_PROVIDER=openai              # openai | anthropic
AI_API_KEY=sk-...               # Provider API key
AI_MODEL=gpt-4o                 # Default model
AI_BASE_URL=                    # Custom endpoint (optional, for OpenAI compatible)
AI_MAX_TOKENS=4096
AI_TEMPERATURE=0.7

# Optional: Anthropic
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_MODEL=claude-sonnet-4-20250514
```

---

## Security Considerations

- API key disimpan di backend, tidak exposed ke FE
- Rate limiting per user (misal 20 requests/minute)
- Max canvas context size (prevent abuse)
- Input sanitization (prevent prompt injection via element labels)
- Cost monitoring per user/org

---

## Related Docs

- [FILE_SHEET_ARCHITECTURE.md](./FILE_SHEET_ARCHITECTURE.md) — File/sheet structure
- [ERD.md](./erd/ERD.md) — Database schema
- [Excalidraw MCP packages](https://www.npmjs.com/search?q=excalidraw%20mcp) — NPM ecosystem

---

## Open Questions

1. **Conversation persistence** — Simpan chat history di DB atau localStorage saja? Answer: Localstorage
2. **Per-file or global chat?** — Chat terikat ke file/sheet atau global? Answer: persheets 
3. **Collaborative AI** — Jika multi-user di room, siapa yang bisa trigger AI? Answer: All users 
4. **Cost model** — Free tier? Token limit per user? Answer: Currently no tier 
5. **Offline fallback** — Jika AI unavailable, show graceful error? Answer: Yes
