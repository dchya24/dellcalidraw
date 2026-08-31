# AI Agent Loop & Persistent Memory — Design

**Status:** Draft, pending user review
**Date:** 2026-08-28
**Scope:** Replace the current one-shot "stream tool_calls, done" flow with a multi-step agent loop and add persistent memory across conversations.

## Problem

Today's AI service has two gaps:

1. **No conversation memory.** The frontend keeps `conversations[tabId]` in `useAIChatStore` but only the last `userMessage.content` is sent to the backend. The backend builds `messages` as `system + user` and never sees prior turns. The model cannot follow up on its own previous tool calls (e.g. "make the previous one red") because it has no record of them.

2. **No tool round-trip.** Tool calls emitted by the LLM are streamed to the browser and executed locally. The result is never sent back to the model. The model cannot validate (`get_canvas_state`) or iterate (`convert_mermaid` then fix layout) — it issues a fixed sequence and hopes for the best.

## Goals

- The model can hold a real conversation: every turn, the full transcript (system prompt + previous user/assistant/tool messages) is sent.
- The model can iterate: after each batch of tool calls, the browser returns results; the model can call tools again. The loop is bounded.
- The model can remember across sessions and tabs (per user, per room).
- Backward compatible: existing single-turn behavior keeps working when the model emits no tool calls.

## Non-Goals (Iteration 1)

- UI for viewing/editing memory (memory is internal-only).
- Memory compression / pruning policies.
- Cross-tab memory diff or conflict resolution.
- Configurable agent loop depth per request (depth is fixed at 20).
- Multi-user shared memory beyond "per room".
- Tool execution server-side. Tools still run in the browser.

## Decisions Locked

- **Tool loop depth:** 20 tool-calls per request (raised from initial 8).
- **Loop end behavior (max steps):** soft stop with a "wrap-up" turn. When the limit is reached, the agent loop injects a system-reminder telling the model to stop calling tools and emit a final text summary, then makes one more LLM call **without tools** so the model produces a natural `finish_reason: stop` and a user-facing summary. No forced `stop`. See "Agent Loop" below.
- **Memory shape:** raw messages are stored verbatim for "show history" purposes, plus an LLM-generated summary with a vector embedding used for retrieval.
- **Memory scope:** per-user (always) and per-room (always). Per-tab filtering is supported in the schema but is optional at retrieval time.
- **Tool execution:** still browser-side. Backend becomes the round-trip broker.

## Architecture

Three concerns are split into three layers in the backend:

1. **Inference plane** (existing). `internal/ai/handler.go`, `internal/ai/provider.go`, and the provider adapters in `openai.go` / `anthropic.go`. Continues to format prompts, talk to OpenAI/Anthropic, and stream SSE.
2. **Agent control plane** (new). `internal/ai/agent/`. Owns the multi-step loop: how the backend waits for tool results from the browser, how it composes messages for the next LLM call, how it enforces the depth cap, how it emits loop-control SSE events.
3. **Memory plane** (new). `internal/ai/memory/`. Owns the DB schema, ingestion (raw → summary → embedding), retrieval per request, and the helper that injects memory into the system prompt.

Frontend changes are minimal: send the full transcript, receive SSE control events, and POST tool results back as they become available.

### High-level request flow

```
+----------+      1. POST /api/ai/chat (full transcript)     +-----------------+
| Frontend | ----------------------------------------------> | Handler         |
|          | <------ 200 + SSE start (requestId, maxSteps)  |                 |
|          |                                                 |                 |
|          |  2. SSE: text, tool_call                        |                 |
|          | <---------------------------------------------- |  3. call LLM    |
|          |                                                 |     iter 1..20  |
|          |  4. POST /api/ai/tool-result                    |                 |
|          | ----------------------------------------------> |  5. append tool |
|          | <---- SSE: text, more tool_call, agent_iteration|     message     |
|          |                                                 |     call LLM    |
|          |                                                 |     next iter   |
|          |  6. SSE: done, agent_final                      |                 |
|          | <---------------------------------------------- |                 |
+----------+                                                 +-----------------+
```

## Agent Loop (20 steps)

### Rules

- **Depth:** 20 tool-calls per request. Configurable via a constant in `internal/ai/agent/loop.go`; not exposed to clients in iteration 1.
- **Iteration = 1 LLM call.** Each iteration is one round-trip to the provider.
- **Tool execution boundary:** an iteration ends when the provider returns a `finish_reason` of either:
  - `stop` (text only) → loop ends normally.
  - `tool_calls` (OpenAI) or `tool_use` (Anthropic) → backend streams each tool call to the browser, then waits for `POST /api/ai/tool-result` to return.
- **Per-iteration timeout:** the request context deadline is shared, not reset per iteration. The total request budget is whatever the HTTP server allows.
- **Loop state lifetime:** per `requestId`, held in process memory (`sync.Map` of `requestId → *LoopState`). GC'd when the loop finishes or the request context is canceled. No DB persistence of loop state.

### Soft stop at max steps

When the cap of 20 tool-calls is hit:

1. The loop appends a `system`-role reminder to the messages list:

   ```
   You have used 20 tool-calls for this request and reached the iteration
   limit. You may not call any more tools. Respond to the user with a final
   text summary of what you created, what is missing, and any caveats.
   Do not call any additional tools.
   ```

2. The loop calls the provider **one more time** with `tools: nil` (so the provider physically cannot emit a tool call). The model emits a final text response and `finish_reason: stop`.

3. The loop emits `agent_final` with `reason: "max_steps"` and then `done`.

The model is never forced to emit `stop` against its will mid-iteration. The wrap-up turn gives the user a real explanation of what happened, instead of a silent cutoff.

### Loop-control SSE events

Three new event types extend the existing `SSEEvent` union:

| Event | Direction | When | Purpose |
| --- | --- | --- | --- |
| `start` | server → client | First event of every chat response | Carries `requestId` and `maxSteps` so the client can address the tool-result endpoint. |
| `agent_iteration` | server → client | Before each LLM call from step 2 onward | Carries `step` (current count) and `maxSteps`. Lets the UI show "AI • iter 3/20". |
| `agent_final` | server → client | After the last LLM call, before `done` | Carries `reason: "stop" \| "max_steps" \| "error"`. Lets the UI display a final state. |

Existing `text`, `tool_call`, `usage`, `done`, `error` events keep their shape.

### `start` event payload

```json
{
  "type": "start",
  "requestId": "5b1d3c7e-...-uuid",
  "maxSteps": 20
}
```

`requestId` is unique per `POST /api/ai/chat` call. The client must echo it back in `POST /api/ai/tool-result`.

### Loop failure handling

| Failure | Behavior |
| --- | --- |
| LLM error in iteration > 1 | Emit SSE `error`, then `agent_final: error`, then `done`. Loop state released. |
| Browser disconnect mid-iteration | `ctx.Done()` fires on the loop. Emit SSE `error: cancelled`, `agent_final: error`. |
| Tool-result POST has unknown `requestId` | Respond `404`. (No SSE channel to write to.) |
| Tool-result POST arrives after loop has already ended (e.g. wrap-up) | Log warn, respond `409`. (Late tool results are dropped.) |

## Tool Round-Trip Contract

### New endpoint

```
POST /api/ai/tool-result
Content-Type: application/json
Authorization: Bearer <access-token>

{
  "requestId": "uuid",
  "results": [
    {
      "callId": "tool_call_xyz",
      "name": "create_rectangle",
      "success": true,
      "result": { "id": "abc123" },
      "error": null
    }
  ]
}
```

Response:
- `200 OK` on accept.
- `404 Not Found` if `requestId` is unknown.
- `409 Conflict` if the loop has already ended.

### Provider-specific tool message format

When the loop receives a tool result, it appends a message to the transcript in the right format for the provider:

- **OpenAI**: `{"role": "tool", "tool_call_id": "xyz", "content": "<json-string>"}`.
- **Anthropic**: appends to the `user` content array as `{"type": "tool_result", "tool_use_id": "xyz", "content": "..."}`.

Provider adapters in `openai.go` and `anthropic.go` are extended to accept an `appendMessages []Message` parameter (or the loop passes the full messages list each time, which is simpler and what we will do).

### Browser-side result shape

For each tool the browser executes, it returns a `ToolResult` with the following convention:

| Tool family | `result` shape | `error` shape |
| --- | --- | --- |
| `create_*` | `{ "id": "<newElementId>" }` | `{ "error": "..." }` on failure |
| `move_elements`, `delete_elements`, `update_element_style`, `edit_text` | `{ "affectedCount": <n> }` | same |
| `auto_layout`, `align_elements`, `resize_elements` | `{ "affectedCount": <n> }` | same |
| `duplicate_elements` | `{ "createdIds": ["..."] }` | same |
| `create_group` | `{ "groupId": "..." }` | same |
| `convert_mermaid` | `{ "elementCount": <n> }` | same |
| `camera_update` | `{}` (no-op) | same |
| `get_canvas_state` | `{ "elementCount": ..., "types": [...], "boundingBox": {...} }` | same |

Failures are still tool-call-side errors, not network errors. The browser returns `success: false` for things like "element not found" or "Mermaid parse failed".

## Conversation Memory

### Schema (migration `000010_ai_memory`)

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE ai_memory_entries (
    id           UUID PRIMARY KEY,
    owner_type   TEXT NOT NULL,            -- 'user' | 'room'
    owner_id     TEXT NOT NULL,            -- user_id or room_id (strings, matching file_tabs.room_id and auth claims)
    tab_id       UUID,                     -- nullable: memory specific to one tab
    kind         TEXT NOT NULL,            -- 'summary' | 'raw'
    content      TEXT NOT NULL,            -- summary text or raw message JSON
    embedding    VECTOR(1536),             -- pgvector
    metadata     JSONB,                    -- {request_id, ts, model, role, ...}
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ai_memory_owner_idx ON ai_memory_entries (owner_type, owner_id, created_at DESC);
CREATE INDEX ai_memory_embedding_idx ON ai_memory_entries USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX ai_memory_tab_idx ON ai_memory_entries (tab_id) WHERE tab_id IS NOT NULL;
```

`pgvector` is a new runtime dependency for the database. The migration must be applied via the same migration runner that already handles `000001`–`000009`.

### Ingestion pipeline

Triggered on SSE `done` (success only). Runs as a background goroutine, not in the request hot path.

1. Collect all messages from the just-finished request that have not yet been ingested (use `request_id` as the dedup key in `metadata`).
2. Call a **summarization LLM** (same provider, but the cheapest available model) with a fixed prompt that returns 1–3 paragraphs covering: topic, key decisions, diagram parameters used, style choices, and any concrete element IDs the user cares about.
3. Embed the summary with `text-embedding-3-small` (default; configurable later).
4. Insert one `ai_memory_entries` row with `kind = 'summary'`, `owner_type = 'user'`, `owner_id = <user>`, `tab_id = <tab>`, `embedding = <vec>`, `metadata = {request_id, ts, model}`.
5. Insert a second `ai_memory_entries` row with `kind = 'summary'`, `owner_type = 'room'`, `owner_id = <room>`, same embedding.
6. For each new turn, also insert a `kind = 'raw'` row containing the verbatim transcript (no embedding) so we can show "history" later.

Failure handling:
- Embedding or summarization failure → log, retry with exponential backoff up to 3 times, then give up silently. The chat request itself is never blocked on ingestion.
- Ingestion runs best-effort. We do not promise durability for memory in iteration 1.

### Retrieval

On every `POST /api/ai/chat`:

1. `userId` is taken from the auth middleware.
2. `roomId` is taken from `canvasContext.roomId`.
3. Compute the embedding for the latest user message.
4. Query top-K=5 by cosine distance:
   - `owner_type = 'user' AND owner_id = $userId`
   - `owner_type = 'room' AND owner_id = $roomId`
5. Truncate total memory to ≤ 800 tokens before injection.

Injected as a new section in the system prompt, between "RESPONSE BEHAVIOR" and "TIPS":

```text
## Relevant memory (user)
- 2026-08-20: User prefers blue pastels for flowcharts, has done login flow twice.
- 2026-08-22: User asked for ERD e-commerce with 4 entities.

## Relevant memory (room: design-team)
- 2026-08-25: Team uses Amber #f59e0b for warnings consistently.
```

Memory is never echoed back to the browser in iteration 1. It only enters the model's context.

### Limits and fallbacks

- Memory block is capped at 800 tokens.
- If `pgvector` is unavailable (or the extension is missing on a dev DB), retrieval returns an empty list and a warning is logged. The request proceeds.
- If embedding call fails, retrieval returns an empty list. Same fallback.
- If `EXCALIDRAW_AI_MEMORY_ENABLED=false`, the entire memory plane is skipped (handy for local dev or e2e tests).

## Frontend Changes

### `src/services/ai/aiService.ts`

- `sendChatMessage` accepts a new field `transcript: ChatMessage[]` (the full tab conversation up to but not including the new user message). It is serialized into the request body as `messages` for the backend.
- `sendChatMessage` now returns an `AsyncIterable<SSEEvent>` instead of a single `onEvent` callback, so the consumer can `await` between `tool_call` events and `submitToolResults` calls. (Promise + manual pause is acceptable; the iterable is just nicer.)
  - Concrete plan: keep the callback API but split the read loop into a separate `readSSE(response, { onEvent, onStart, onAgentIteration, onDone })` helper. `submitToolResults` becomes a new exported function.
- New `submitToolResults(requestId, results)` calls `POST /api/ai/tool-result`.
- New optional `listMemories()` and `clearMemories()` are stubs for iteration 1; they hit `/api/ai/memory` GET/DELETE but are not yet wired into the UI.

### `src/components/ai/AIChatPanel.tsx`

- In `handleSend`, build the transcript: `conversations[currentTabId]` minus the message we are about to add. Send it.
- On the `start` event, store `requestId` in a ref.
- On each `tool_call` event, run the existing `apply*` functions. Collect a `ToolResult[]` for the iteration.
- After all `tool_call` events in the iteration arrive, call `submitToolResults(requestId, results)`.
- On `agent_iteration`, update a small header badge "AI • iter N/20".
- On `agent_final`, mark the iteration as concluded.
- On `done`, run the existing finalization (tool summary, history.clear, etc.).
- "Stop" button: cancel the entire request via a single `AbortController`, not per fetch.

### `src/store/useAIChatStore.ts`

- New field `pendingToolResults: Map<requestId, ToolResult[]>` to handle the case where the loop requests a re-send. (For iteration 1 this is internal-only and not surfaced in the UI.)
- New optional `memories: MemoryEntry[]` field for future use; not populated yet.

### UI changes (minimal)

- Header chip: "AI • iter 3/20" while the loop is active.
- "Stop" button cancels the whole request.
- No memory inspector UI yet.

## Error Handling Summary

| Scenario | Behavior |
| --- | --- |
| LLM returns 5xx in iteration 1 | SSE `error` immediately, `agent_final: error`, `done`. |
| LLM returns 5xx in iteration > 1 | Same as above. |
| Browser does not POST tool-result within request context | SSE `error: cancelled`, `agent_final: error`. |
| Tool-result POST has bad shape | `400 Bad Request`. |
| Tool execution in browser throws | Tool is reported with `success: false, error: <msg>`. Model decides what to do next. |
| Memory retrieval fails | Empty memory block. Log warning. |
| Ingestion fails | Log, retry up to 3x, then give up. No user impact. |
| Anthropic vs OpenAI tool role mismatch | Adapters handle their respective formats; loop passes the full messages list each iteration. |

## Testing

### Unit

- `internal/ai/agent/loop_test.go`: with a mock provider, verify that:
  - Text-only responses short-circuit and emit `agent_final: stop`.
  - Tool-call responses trigger a wait for `/tool-result`, then a follow-up call.
  - 20 tool-calls hit the cap, then a wrap-up call is made with `tools: nil`.
  - Provider errors mid-loop emit `error` and `agent_final: error`.
- `internal/ai/memory/retrieve_test.go`: with a fake vector store, verify top-K ordering, user+room merge, and 800-token truncation.
- `internal/ai/memory/ingest_test.go`: ingestion produces one user row + one room row + N raw rows; failures do not panic.
- Provider adapter tests for `tool` role round-trip in both `openai.go` and `anthropic.go`.

### Integration

- `POST /api/ai/chat` + `POST /api/ai/tool-result` end-to-end with a real LLM in dev mode (existing dev-mode log path still works).
- DB migration test: `000010_ai_memory.up.sql` runs cleanly on a fresh DB and on a DB at `000009`.

### Frontend

- `parseSSEEvent` tests for `start`, `agent_iteration`, `agent_final`.
- `submitToolResults` happy path + 404 + 409.
- Update existing AI panel test fixtures.

### Manual E2E

1. Prompt: "Buatkan flowchart login 5 langkah".
2. AI iter 1: emits 5 `create_rectangle` + 4 `create_arrow` + 1 `camera_update`. Browser executes and submits results.
3. AI iter 2: emits `get_canvas_state` to verify, then `update_element_style` to fix a color. Browser submits.
4. AI iter 3: text summary. `agent_final: stop`. `done`.
5. Re-open the app fresh in a new tab. Send "buat flowchart login yang sebelumnya, ubah jadi warna hijau". Memory retrieval surfaces the prior login flow, model produces a green version without re-asking.

## Out of Scope (Iteration 1)

- UI for inspecting / editing memory.
- Memory compression / pruning (we keep everything; vacuum later).
- Cross-tab memory diff or sync.
- Configurable loop depth.
- Multi-tenant organization-level memory.
- Streaming summarization during the loop (we summarize only on `done`).
- Server-side tool execution. Tools remain browser-side.
- Alternative embedding providers (we ship with OpenAI's `text-embedding-3-small`).

## Open Questions

- Should we rate-limit the `/tool-result` endpoint per user? (Likely yes, but out of scope for iteration 1 — defer to a follow-up.)
- Should memory blocks be visible to the user in a "what does the AI know about me" panel? (Likely yes; out of scope for iteration 1.)
- Do we want to stream the wrap-up turn to the client, or buffer it? Plan: stream, since it's just another LLM call.

## Implementation Sequencing

1. **Memory plane first**, because it changes the system prompt and the request body, both of which the agent loop will use. Memory does not require a loop.
2. **Agent loop second**, because it requires the system prompt changes from step 1 and the full-transcript request body.
3. **Frontend transcript + tool-result submission last**, because it depends on the new `start` event and the new endpoint.

Each step ends with a green test suite and a manual smoke test.
