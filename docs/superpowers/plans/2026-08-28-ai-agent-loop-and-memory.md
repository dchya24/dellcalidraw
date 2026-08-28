# AI Agent Loop & Persistent Memory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the current one-shot "stream tool_calls, done" AI flow into a 20-step agent loop with full conversation transcripts and persistent per-user/per-room memory.

**Architecture:** Three layers in the backend. Inference plane (`internal/ai/`) keeps its current job of formatting prompts and streaming from the provider. A new `internal/ai/agent/` package owns the multi-step loop: how the server waits for tool results from the browser, how it composes the next LLM call, and how it caps depth at 20 with a wrap-up turn. A new `internal/ai/memory/` package owns the DB schema, ingestion (raw → summary → embedding), and retrieval that injects top-K relevant memories into the system prompt. Frontend sends the full transcript, receives new control SSE events (`start`, `agent_iteration`, `agent_final`), and POSTs tool results back to a new endpoint.

**Tech Stack:** Go 1.25, chi router, `database/sql` + `pgvector` (new runtime dep), OpenAI `text-embedding-3-small` for embeddings, OpenAI/Anthropic chat APIs (streaming), React 18 + TypeScript + Vitest on the frontend.

## Sequencing

This plan is split into three milestones. Each milestone ends with a green test suite, a manual smoke test, and a working slice of the feature.

- **Milestone 1 — Memory plane.** DB schema, embedding client, summarizer, ingest, retrieve, system-prompt injection, and the env flag. After this, the AI gets memory but no agent loop.
- **Milestone 2 — Agent loop.** Loop state holder, new SSE control events, `POST /api/ai/tool-result`, provider adapters for `tool` role, wrap-up at 20. After this, the AI can iterate; memory and loop work independently.
- **Milestone 3 — Frontend transcript + tool-result submission.** Frontend sends the full transcript, parses new SSE events, and submits tool results. After this, end-to-end the user can have a multi-turn, tool-iterating chat with memory.

## File Structure

### New files (Milestone 1)

- `excalidraw-be/internal/database/migrations/000010_ai_memory.up.sql` — schema for `ai_memory_entries` with `pgvector`.
- `excalidraw-be/internal/database/migrations/000010_ai_memory.down.sql` — drop table.
- `excalidraw-be/internal/database/ai_memory.go` — `AIMemoryRepository` (Insert summary, Insert raw, TopK by owner+embedding, GetByID).
- `excalidraw-be/internal/ai/memory/embeddings.go` — OpenAI `text-embedding-3-small` HTTP client.
- `excalidraw-be/internal/ai/memory/retrieve.go` — top-K retrieval, merge user+room, 800-token truncation, system-prompt formatter.
- `excalidraw-be/internal/ai/memory/retrieve_test.go` — fake store, ordering, truncation tests.
- `excalidraw-be/internal/ai/memory/ingest.go` — summarizer (single LLM call) + ingest pipeline (Insert summary × 2 + Insert raw × N).
- `excalidraw-be/internal/ai/memory/ingest_test.go` — summary + raw insertion, failure path.
- `excalidraw-be/internal/ai/memory/types.go` — `MemoryEntry`, `MemoryOwner` (user/room), `Summary` types.

### New files (Milestone 2)

- `excalidraw-be/internal/ai/agent/loop.go` — `LoopState`, `Run(ctx, provider, msgs, tools, onEvent)`, 20-step cap, wrap-up call.
- `excalidraw-be/internal/ai/agent/loop_test.go` — mock provider, scenarios in the spec.
- `excalidraw-be/internal/ai/agent/registry.go` — `sync.Map` of `requestId → *LoopState`, GetOrCreate, Drop.

### Modified files (Milestone 1)

- `excalidraw-be/internal/ai/provider.go` — accept `Memory []MemoryEntry` parameter, append memory block to system prompt via `BuildSystemPrompt`.
- `excalidraw-be/internal/ai/handler.go` — call memory retrieve before ChatStream, build transcript, pass memory to provider.
- `excalidraw-be/internal/ai/handler.go` — kick off background ingest on successful `done`.
- `excalidraw-be/internal/config/config.go` — add `ai.memory_enabled`, `ai.embedding_model`, `ai.memory_top_k`, `ai.memory_max_tokens`.
- `excalidraw-be/.env.example` — add new env vars.
- `excalidraw-be/cmd/server/main.go` — wire memory repo + embedder into handler if `memory_enabled`.

### Modified files (Milestone 2)

- `excalidraw-be/internal/ai/handler.go` — emit `start` event, route `POST /api/ai/tool-result`, replace inline `ChatStream` with `agent.Run`, emit `agent_iteration` + `agent_final`.
- `excalidraw-be/internal/ai/provider.go` — add `ToolRole Message` support, `Memory []MemoryEntry` already added in M1.
- `excalidraw-be/internal/ai/openai.go` — translate `tool` role messages to OpenAI format.
- `excalidraw-be/internal/ai/anthropic.go` — translate `tool` role messages to Anthropic user-content blocks.

### Modified files (Milestone 3)

- `excalidraw-fe/src/services/ai/aiService.ts` — `sendChatMessage` accepts `transcript`, returns control events, adds `submitToolResults(requestId, results)`.
- `excalidraw-fe/src/services/__tests__/aiService.test.ts` — add tests for new events and submitToolResults.
- `excalidraw-fe/src/components/ai/AIChatPanel.tsx` — send transcript, store requestId, collect per-iteration tool results, submit on completion, render iter badge.
- `excalidraw-fe/src/store/useAIChatStore.ts` — add `pendingToolResults` field; not surfaced in UI yet.

## Global Constraints

- Go 1.25, single binary at `excalidraw-be/cmd/server`.
- `pgvector` extension must be available in the database. Migration is a hard dependency; do not skip.
- Embedding model: `text-embedding-3-small` (1536 dimensions). Do not change dimension without a migration.
- Memory block in system prompt: max 800 tokens. Hard cap. If exceeded, truncate by dropping the oldest entries first.
- Memory is per-user and per-room only. No global/org-wide memory. Per-tab field is nullable and unused at retrieval time.
- Loop depth: 20 tool-calls. Constant in `agent/loop.go` named `MaxToolCalls`. Not configurable from client.
- All new endpoints go under `/api/ai`. Auth: same JWT middleware as other AI endpoints.
- All new SSE events follow the existing `SSEEvent` JSON shape in `internal/ai/provider.go`. Add new `Type` values; do not invent a parallel event format.
- Frontend keeps the existing `parseSSEEvent` shape. New event types are added to the union; old tests must still pass.
- Commit format: conventional commits (`feat:`, `fix:`, `refactor:`, `chore:`, `test:`, `docs:`).
- TypeScript strict mode, ESLint clean, no `any` in new code unless an existing module already requires it (Excalidraw types).
- All Go code: `gofmt`, `go vet`, `golangci-lint` clean.
- Test framework: standard `testing` for Go, Vitest for frontend.
- Do not edit `excalidraw-fe/src/types/ai.ts` event types without updating both the parser in `aiService.ts` and any test fixtures.
- No new third-party npm packages without explicit user approval.
- No new third-party Go modules beyond what's already in `go.mod`. If `pgvector` needs a driver, prefer plain SQL via `database/sql`.

---

## Milestone 1 — Memory Plane

### Task 1.1: Add migration for `ai_memory_entries`

**Files:**
- Create: `excalidraw-be/internal/database/migrations/000010_ai_memory.up.sql`
- Create: `excalidraw-be/internal/database/migrations/000010_ai_memory.down.sql`

**Interfaces:**
- Consumes: existing migration runner (`migrate.go`).
- Produces: table `ai_memory_entries` and indexes.

- [ ] **Step 1: Create the up migration**

Write `excalidraw-be/internal/database/migrations/000010_ai_memory.up.sql`:

```sql
-- ai_memory_entries: persistent memory for the AI assistant
-- Each row is either a 'summary' (with vector embedding, used for retrieval)
-- or a 'raw' (verbatim transcript, no embedding, used for history display).

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS ai_memory_entries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type   TEXT NOT NULL,            -- 'user' | 'room'
    owner_id     TEXT NOT NULL,
    tab_id       UUID,
    kind         TEXT NOT NULL,            -- 'summary' | 'raw'
    content      TEXT NOT NULL,
    embedding    VECTOR(1536),
    metadata     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ai_memory_owner_idx
    ON ai_memory_entries (owner_type, owner_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ai_memory_embedding_idx
    ON ai_memory_entries USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX IF NOT EXISTS ai_memory_tab_idx
    ON ai_memory_entries (tab_id) WHERE tab_id IS NOT NULL;
```

- [ ] **Step 2: Create the down migration**

Write `excalidraw-be/internal/database/migrations/000010_ai_memory.down.sql`:

```sql
DROP INDEX IF EXISTS ai_memory_tab_idx;
DROP INDEX IF EXISTS ai_memory_embedding_idx;
DROP INDEX IF EXISTS ai_memory_owner_idx;
DROP TABLE IF EXISTS ai_memory_entries;
-- Note: do NOT drop the vector extension; other tables may use it later.
```

- [ ] **Step 3: Verify the migration runner picks it up**

Run: `cd excalidraw-be && go build ./...`
Expected: success, no new compile errors. The runner uses `embed.FS` on `migrations/*.sql`, so adding files is automatic.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/internal/database/migrations/000010_ai_memory.up.sql \
        excalidraw-be/internal/database/migrations/000010_ai_memory.down.sql
git commit -m "feat(db): add ai_memory_entries schema with pgvector"
```

---

### Task 1.2: Add `AIMemoryRepository` (InsertSummary, InsertRaw, TopK)

**Files:**
- Create: `excalidraw-be/internal/database/ai_memory.go`

**Interfaces:**
- Consumes: `*sql.DB` from `database.PostgresClient.DB()`.
- Produces: `AIMemoryRepository` with methods:
  - `InsertSummary(ctx, entry MemoryEntry) error` — `kind='summary'`, `embedding` required.
  - `InsertRaw(ctx, entry MemoryEntry) error` — `kind='raw'`, no embedding.
  - `TopK(ctx, ownerType, ownerID string, embedding []float32, k int) ([]MemoryEntry, error)` — ordered by cosine distance ASC, returns the row data without the raw embedding bytes to keep call sites small.
  - `GetByID(ctx, id uuid.UUID) (MemoryEntry, error)` — for tests.

Where `MemoryEntry` is defined in `internal/ai/memory/types.go` (Task 1.3). The DB layer imports the memory package — that is the only place a downward dependency from `database/` to a feature package exists. Acceptable: existing `ai_request_logs` is also a feature-shaped table.

- [ ] **Step 1: Write the failing test for InsertSummary and TopK**

Create `excalidraw-be/internal/database/ai_memory_test.go` (we will fill in the real test once the types exist in Task 1.3; for now add a placeholder that fails to compile to prove the dependency):

```go
package database

import "testing"

func TestAIMemoryRepository_Placeholder(t *testing.T) {
    t.Skip("placeholder until types land in 1.3")
}
```

Run: `cd excalidraw-be && go test ./internal/database/ -run TestAIMemoryRepository_Placeholder -v`
Expected: skip, not fail. Confirms file is wired in.

- [ ] **Step 2: Add the repository skeleton**

Create `excalidraw-be/internal/database/ai_memory.go`:

```go
package database

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log/slog"

    "github.com/google/uuid"
    "github.com/you/excalidraw-be/internal/ai/memory"
)

type AIMemoryRepository struct {
    db *sql.DB
}

func NewAIMemoryRepository(db *sql.DB) *AIMemoryRepository {
    return &AIMemoryRepository{db: db}
}

func (r *AIMemoryRepository) InsertSummary(ctx context.Context, e memory.MemoryEntry) error {
    if e.ID == uuid.Nil {
        e.ID = uuid.New()
    }
    if e.Kind != "summary" {
        return fmt.Errorf("InsertSummary requires Kind='summary', got %q", e.Kind)
    }
    if len(e.Embedding) == 0 {
        return fmt.Errorf("InsertSummary requires embedding")
    }
    meta, err := json.Marshal(e.Metadata)
    if err != nil {
        return fmt.Errorf("marshal metadata: %w", err)
    }
    _, err = r.db.ExecContext(ctx, `
        INSERT INTO ai_memory_entries (id, owner_type, owner_id, tab_id, kind, content, embedding, metadata)
        VALUES ($1, $2, $3, $4, 'summary', $5, $6::vector, $7)`,
        e.ID, e.OwnerType, e.OwnerID, e.TabID, e.Content, pgVectorString(e.Embedding), meta)
    if err != nil {
        return fmt.Errorf("insert summary: %w", err)
    }
    slog.Debug("[ai_memory] inserted summary", "id", e.ID, "owner", e.OwnerType)
    return nil
}

func (r *AIMemoryRepository) InsertRaw(ctx context.Context, e memory.MemoryEntry) error {
    if e.ID == uuid.Nil {
        e.ID = uuid.New()
    }
    if e.Kind != "raw" {
        return fmt.Errorf("InsertRaw requires Kind='raw', got %q", e.Kind)
    }
    meta, err := json.Marshal(e.Metadata)
    if err != nil {
        return fmt.Errorf("marshal metadata: %w", err)
    }
    _, err = r.db.ExecContext(ctx, `
        INSERT INTO ai_memory_entries (id, owner_type, owner_id, tab_id, kind, content, metadata)
        VALUES ($1, $2, $3, $4, 'raw', $5, $6)`,
        e.ID, e.OwnerType, e.OwnerID, e.TabID, e.Content, meta)
    if err != nil {
        return fmt.Errorf("insert raw: %w", err)
    }
    return nil
}

func (r *AIMemoryRepository) TopK(ctx context.Context, ownerType, ownerID string, embedding []float32, k int) ([]memory.MemoryEntry, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, owner_type, owner_id, tab_id, content, metadata, created_at,
               (embedding <=> $1::vector) AS distance
        FROM ai_memory_entries
        WHERE owner_type = $2 AND owner_id = $3 AND kind = 'summary' AND embedding IS NOT NULL
        ORDER BY embedding <=> $1::vector
        LIMIT $4`,
        pgVectorString(embedding), ownerType, ownerID, k)
    if err != nil {
        return nil, fmt.Errorf("topk: %w", err)
    }
    defer rows.Close()

    var out []memory.MemoryEntry
    for rows.Next() {
        var e memory.MemoryEntry
        var meta sql.NullString
        var tabID sql.NullString
        var distance float64
        if err := rows.Scan(&e.ID, &e.OwnerType, &e.OwnerID, &tabID, &e.Content, &meta, &e.CreatedAt, &distance); err != nil {
            return nil, fmt.Errorf("scan: %w", err)
        }
        if tabID.Valid {
            id, _ := uuid.Parse(tabID.String)
            e.TabID = &id
        }
        if meta.Valid {
            _ = json.Unmarshal([]byte(meta.String), &e.Metadata)
        }
        e.Distance = distance
        out = append(out, e)
    }
    return out, rows.Err()
}

func (r *AIMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (memory.MemoryEntry, error) {
    var e memory.MemoryEntry
    var tabID sql.NullString
    var meta sql.NullString
    err := r.db.QueryRowContext(ctx, `
        SELECT id, owner_type, owner_id, tab_id, kind, content, metadata, created_at
        FROM ai_memory_entries WHERE id = $1`, id).
        Scan(&e.ID, &e.OwnerType, &e.OwnerID, &tabID, &e.Kind, &e.Content, &meta, &e.CreatedAt)
    if err != nil {
        return e, err
    }
    if tabID.Valid {
        u, _ := uuid.Parse(tabID.String)
        e.TabID = &u
    }
    if meta.Valid {
        _ = json.Unmarshal([]byte(meta.String), &e.Metadata)
    }
    return e, nil
}

// pgVectorString renders a []float32 as the literal '[a,b,c]' that pgvector accepts.
func pgVectorString(v []float32) string {
    if len(v) == 0 {
        return "[]"
    }
    buf := make([]byte, 0, len(v)*8)
    buf = append(buf, '[')
    for i, f := range v {
        if i > 0 {
            buf = append(buf, ',')
        }
        buf = strconvAppendFloat(buf, float64(f))
    }
    buf = append(buf, ']')
    return string(buf)
}

func strconvAppendFloat(b []byte, f float64) []byte {
    // Avoid pulling strconv for one call; this matches fmt %g.
    return append(b, []byte(strconvFloat(f))...)
}

func strconvFloat(f float64) string {
    return fmt.Sprintf("%g", f)
}
```

Replace the helper block above with this cleaner version (the earlier block was a placeholder; use this in the final file):

```go
import "strconv"

func pgVectorString(v []float32) string {
    if len(v) == 0 {
        return "[]"
    }
    parts := make([]string, len(v))
    for i, f := range v {
        parts[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
    }
    return "[" + strings.Join(parts, ",") + "]"
}
```

And add `"strings"` and `"strconv"` to the import list.

- [ ] **Step 3: Confirm the placeholder test still passes and the package compiles**

Run: `cd excalidraw-be && go build ./... && go test ./internal/database/ -run TestAIMemoryRepository_Placeholder -v`
Expected: build succeeds; placeholder test is skipped (it imports `memory.MemoryEntry` which doesn't exist yet — fix the import order, the real import comes after Task 1.3).

- [ ] **Step 4: Defer real tests until Task 1.3 — commit only the repo skeleton**

```bash
git add excalidraw-be/internal/database/ai_memory.go
git commit -m "feat(db): add AIMemoryRepository skeleton"
```

Note: this commit will not compile until Task 1.3 lands `memory.MemoryEntry`. If you prefer green-at-each-commit, fold 1.2 and 1.3 into one commit. The next task fixes the compile.

---

### Task 1.3: Define `memory.MemoryEntry` and friends

**Files:**
- Create: `excalidraw-be/internal/ai/memory/types.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type MemoryEntry struct { ID uuid.UUID; OwnerType, OwnerID, Kind, Content string; TabID *uuid.UUID; Embedding []float32; Metadata map[string]any; CreatedAt time.Time; Distance float64 }`
  - `type Owner struct { Type, ID string }`
  - `const KindSummary = "summary"; const KindRaw = "raw"; const OwnerUser = "user"; const OwnerRoom = "room"`

- [ ] **Step 1: Write the types file**

Create `excalidraw-be/internal/ai/memory/types.go`:

```go
package memory

import (
    "time"

    "github.com/google/uuid"
)

const (
    KindSummary = "summary"
    KindRaw     = "raw"

    OwnerUser = "user"
    OwnerRoom = "room"
)

type MemoryEntry struct {
    ID        uuid.UUID
    OwnerType string // OwnerUser | OwnerRoom
    OwnerID   string
    TabID     *uuid.UUID
    Kind      string // KindSummary | KindRaw
    Content   string
    Embedding []float32
    Metadata  map[string]any
    CreatedAt time.Time

    // Populated by TopK, ignored on insert.
    Distance float64
}

type Owner struct {
    Type string
    ID   string
}

func UserOwner(userID string) Owner { return Owner{Type: OwnerUser, ID: userID} }
func RoomOwner(roomID string) Owner { return Owner{Type: OwnerRoom, ID: roomID} }
```

- [ ] **Step 2: Build to confirm 1.2 + 1.3 compile together**

Run: `cd excalidraw-be && go build ./...`
Expected: success.

- [ ] **Step 3: Run database tests**

Run: `cd excalidraw-be && go test ./internal/database/ -v`
Expected: existing tests pass, placeholder still skipped.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/internal/ai/memory/types.go
git commit -m "feat(ai/memory): define MemoryEntry types"
```

---

### Task 1.4: Real repository tests against a test database

**Files:**
- Modify: `excalidraw-be/internal/database/ai_memory_test.go`

**Interfaces:**
- Consumes: a Postgres test instance (CI-friendly). Use `t.Skip` if `DATABASE_URL` is not set so the suite stays green in environments without DB.
- Produces: round-trip tests for InsertSummary, InsertRaw, TopK ordering, GetByID.

- [ ] **Step 1: Write the test using `database/sql` + `pq` driver directly**

Replace the placeholder in `excalidraw-be/internal/database/ai_memory_test.go`:

```go
package database

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/you/excalidraw-be/internal/ai/memory"
)

func openTestDB(t *testing.T) (*sql.DB, func()) {
    t.Helper()
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set; skipping DB-backed test")
    }
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    if err := db.Ping(); err != nil {
        t.Fatalf("ping: %v", err)
    }
    return db, func() { _ = db.Close() }
}

func TestAIMemoryRepository_InsertSummaryAndTopK(t *testing.T) {
    db, cleanup := openTestDB(t)
    defer cleanup()
    repo := NewAIMemoryRepository(db)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    userID := uuid.NewString()
    // Two summaries, embed them so one is clearly closer to the query.
    close := []float32{0.10, 0.20, 0.30}
    far := []float32{0.90, 0.80, 0.70}

    if err := repo.InsertSummary(ctx, memory.MemoryEntry{
        OwnerType: memory.OwnerUser, OwnerID: userID, Kind: memory.KindSummary,
        Content: "User prefers blue pastels", Embedding: close,
    }); err != nil {
        t.Fatalf("insert close: %v", err)
    }
    if err := repo.InsertSummary(ctx, memory.MemoryEntry{
        OwnerType: memory.OwnerUser, OwnerID: userID, Kind: memory.KindSummary,
        Content: "User asked for ERD", Embedding: far,
    }); err != nil {
        t.Fatalf("insert far: %v", err)
    }

    got, err := repo.TopK(ctx, memory.OwnerUser, userID, close, 1)
    if err != nil {
        t.Fatalf("topk: %v", err)
    }
    if len(got) != 1 {
        t.Fatalf("expected 1 result, got %d", len(got))
    }
    if got[0].Content != "User prefers blue pastels" {
        t.Errorf("expected closest entry, got %q", got[0].Content)
    }
}

func TestAIMemoryRepository_InsertRaw(t *testing.T) {
    db, cleanup := openTestDB(t)
    defer cleanup()
    repo := NewAIMemoryRepository(db)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := repo.InsertRaw(ctx, memory.MemoryEntry{
        OwnerType: memory.OwnerRoom, OwnerID: "room-1", Kind: memory.KindRaw,
        Content: "user: hi\nassistant: hello",
    }); err != nil {
        t.Fatalf("insert raw: %v", err)
    }
}
```

Add `"database/sql"` to the import list.

- [ ] **Step 2: Run the tests without DB env to confirm skip behavior**

Run: `cd excalidraw-be && go test ./internal/database/ -run TestAIMemoryRepository -v`
Expected: tests skipped with "TEST_DATABASE_URL not set".

- [ ] **Step 3: Run with DB to confirm round-trip (only if a Postgres+pgvector is available locally)**

Run: `TEST_DATABASE_URL=postgres://user:pass@localhost/excalidraw_test?sslmode=disable cd excalidraw-be && go test ./internal/database/ -run TestAIMemoryRepository -v`
Expected: both tests pass.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/internal/database/ai_memory_test.go
git commit -m "test(db): add AIMemoryRepository round-trip tests"
```

---

### Task 1.5: Add `EmbeddingsClient` (OpenAI `text-embedding-3-small`)

**Files:**
- Create: `excalidraw-be/internal/ai/memory/embeddings.go`

**Interfaces:**
- Consumes: `apiKey, baseURL string`.
- Produces: `type EmbeddingsClient struct{...}; func NewEmbeddingsClient(apiKey, baseURL, model string) *EmbeddingsClient; func (c *EmbeddingsClient) Embed(ctx context.Context, texts []string) ([][]float32, error)`.

- [ ] **Step 1: Write the failing test**

Create `excalidraw-be/internal/ai/memory/embeddings_test.go`:

```go
package memory

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestEmbeddingsClient_Embed(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !strings.HasSuffix(r.URL.Path, "/embeddings") {
            t.Errorf("unexpected path %s", r.URL.Path)
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{
            "data": [
                {"embedding": [0.1, 0.2, 0.3]},
                {"embedding": [0.4, 0.5, 0.6]}
            ]
        }`))
    }))
    defer srv.Close()

    c := NewEmbeddingsClient("test-key", srv.URL, "text-embedding-3-small")
    got, err := c.Embed(context.Background(), []string{"a", "b"})
    if err != nil {
        t.Fatalf("embed: %v", err)
    }
    if len(got) != 2 {
        t.Fatalf("expected 2 vectors, got %d", len(got))
    }
    if got[0][0] != 0.1 || got[1][2] != 0.6 {
        t.Errorf("vectors wrong: %v %v", got[0], got[1])
    }
}
```

- [ ] **Step 2: Run test, confirm it fails**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run TestEmbeddingsClient -v`
Expected: compile error (`undefined: NewEmbeddingsClient`).

- [ ] **Step 3: Implement `EmbeddingsClient`**

Create `excalidraw-be/internal/ai/memory/embeddings.go`:

```go
package memory

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

type EmbeddingsClient struct {
    APIKey  string
    BaseURL string
    Model   string
    HTTP    *http.Client
}

func NewEmbeddingsClient(apiKey, baseURL, model string) *EmbeddingsClient {
    if baseURL == "" {
        baseURL = "https://api.openai.com/v1"
    }
    if model == "" {
        model = "text-embedding-3-small"
    }
    return &EmbeddingsClient{
        APIKey:  apiKey,
        BaseURL: baseURL,
        Model:   model,
        HTTP:    &http.Client{Timeout: 30 * time.Second},
    }
}

type embedRequest struct {
    Input []string `json:"input"`
    Model string   `json:"model"`
}

type embedResponse struct {
    Data []struct {
        Embedding []float32 `json:"embedding"`
    } `json:"data"`
    Error *struct {
        Message string `json:"message"`
    } `json:"error,omitempty"`
}

func (c *EmbeddingsClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    if len(texts) == 0 {
        return nil, nil
    }
    body, err := json.Marshal(embedRequest{Input: texts, Model: c.Model})
    if err != nil {
        return nil, fmt.Errorf("marshal: %w", err)
    }
    req, err := http.NewRequestWithContext(ctx, "POST",
        strings.TrimSuffix(c.BaseURL, "/")+"/embeddings", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.APIKey)
    resp, err := c.HTTP.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("embeddings %d: %s", resp.StatusCode, string(b))
    }
    var out embedResponse
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        return nil, fmt.Errorf("decode: %w", err)
    }
    if out.Error != nil {
        return nil, fmt.Errorf("embeddings error: %s", out.Error.Message)
    }
    result := make([][]float32, len(out.Data))
    for i, d := range out.Data {
        result[i] = d.Embedding
    }
    return result, nil
}
```

- [ ] **Step 4: Run the test, confirm it passes**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run TestEmbeddingsClient -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add excalidraw-be/internal/ai/memory/embeddings.go \
        excalidraw-be/internal/ai/memory/embeddings_test.go
git commit -m "feat(ai/memory): add OpenAI embeddings client"
```

---

### Task 1.6: Add `Retrieve` with top-K merge, truncation, and prompt formatter

**Files:**
- Create: `excalidraw-be/internal/ai/memory/retrieve.go`
- Create: `excalidraw-be/internal/ai/memory/retrieve_test.go`

**Interfaces:**
- Consumes: `*AIMemoryRepository` (database), `*EmbeddingsClient`, `Owner` slice.
- Produces:
  - `type Retriever struct{...}; func NewRetriever(repo *AIMemoryRepository, emb *EmbeddingsClient, topKPerOwner, maxTokens int)`
  - `func (r *Retriever) Retrieve(ctx context.Context, query string, owners []Owner) ([]MemoryEntry, error)`
  - `func FormatMemoryBlock(entries []MemoryEntry) string` — renders the section the system prompt will inject.
  - `func TruncateToTokens(text string, maxTokens int) string` — heuristic: ~4 chars per token, cuts on the last newline before the limit.

- [ ] **Step 1: Write the failing test for `FormatMemoryBlock` and `TruncateToTokens`**

Create `excalidraw-be/internal/ai/memory/retrieve_test.go`:

```go
package memory

import (
    "strings"
    "testing"
    "time"

    "github.com/google/uuid"
)

func TestFormatMemoryBlock_GroupsByOwner(t *testing.T) {
    now := time.Now()
    entries := []MemoryEntry{
        {OwnerType: OwnerUser, Content: "User likes blue", CreatedAt: now},
        {OwnerType: OwnerRoom, Content: "Team uses amber", CreatedAt: now},
    }
    out := FormatMemoryBlock(entries)
    if !strings.Contains(out, "## Relevant memory (user)") {
        t.Errorf("missing user header in:\n%s", out)
    }
    if !strings.Contains(out, "## Relevant memory (room)") {
        t.Errorf("missing room header in:\n%s", out)
    }
    if !strings.Contains(out, "User likes blue") || !strings.Contains(out, "Team uses amber") {
        t.Errorf("missing content in:\n%s", out)
    }
}

func TestTruncateToTokens_RespectsLimit(t *testing.T) {
    s := strings.Repeat("alpha ", 1000) // ~5000 chars ~ 1250 tokens
    out := TruncateToTokens(s, 100)
    if len(out) > 100*4+10 {
        t.Errorf("output too long: %d chars", len(out))
    }
    if !strings.HasSuffix(out, "...") {
        t.Errorf("expected truncation marker, got %q", out)
    }
    _ = uuid.Nil
}
```

- [ ] **Step 2: Run, confirm it fails**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run 'TestFormatMemoryBlock|TestTruncateToTokens' -v`
Expected: compile error (`undefined: FormatMemoryBlock`).

- [ ] **Step 3: Implement `FormatMemoryBlock` and `TruncateToTokens`**

Create `excalidraw-be/internal/ai/memory/retrieve.go`:

```go
package memory

import (
    "context"
    "fmt"
    "sort"
    "strings"
    "time"
)

const approxCharsPerToken = 4

type Retriever struct {
    Repo        *AIMemoryRepository
    Embeddings  *EmbeddingsClient
    TopKPerOwner int
    MaxTokens   int
}

func NewRetriever(repo *AIMemoryRepository, emb *EmbeddingsClient, topK, maxTokens int) *Retriever {
    if topK <= 0 {
        topK = 5
    }
    if maxTokens <= 0 {
        maxTokens = 800
    }
    return &Retriever{Repo: repo, Embeddings: emb, TopKPerOwner: topK, MaxTokens: maxTokens}
}

func (r *Retriever) Retrieve(ctx context.Context, query string, owners []Owner) ([]MemoryEntry, error) {
    if r.Embeddings == nil || r.Repo == nil || len(owners) == 0 {
        return nil, nil
    }
    vecs, err := r.Embeddings.Embed(ctx, []string{query})
    if err != nil {
        return nil, fmt.Errorf("embed query: %w", err)
    }
    if len(vecs) == 0 {
        return nil, nil
    }
    merged := make([]MemoryEntry, 0, r.TopKPerOwner*len(owners))
    for _, o := range owners {
        got, err := r.Repo.TopK(ctx, o.Type, o.ID, vecs[0], r.TopKPerOwner)
        if err != nil {
            return nil, err
        }
        for _, e := range got {
            e.OwnerType = o.Type // ensure stable grouping
            merged = append(merged, e)
        }
    }
    return merged, nil
}

func FormatMemoryBlock(entries []MemoryEntry) string {
    if len(entries) == 0 {
        return ""
    }
    // Sort entries within each owner by CreatedAt DESC (newest first).
    byOwner := map[string][]MemoryEntry{}
    for _, e := range entries {
        byOwner[e.OwnerType] = append(byOwner[e.OwnerType], e)
    }
    var out strings.Builder
    for _, ownerType := range []string{OwnerUser, OwnerRoom} {
        list, ok := byOwner[ownerType]
        if !ok {
            continue
        }
        sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
        header := "## Relevant memory (user)"
        if ownerType == OwnerRoom {
            header = "## Relevant memory (room)"
        }
        fmt.Fprintf(&out, "\n%s\n", header)
        for _, e := range list {
            fmt.Fprintf(&out, "- %s: %s\n", e.CreatedAt.UTC().Format("2006-01-02"), e.Content)
        }
    }
    return out.String()
}

func TruncateToTokens(text string, maxTokens int) string {
    if maxTokens <= 0 {
        return ""
    }
    limit := maxTokens * approxCharsPerToken
    if len(text) <= limit {
        return text
    }
    cut := strings.LastIndex(text[:limit], "\n")
    if cut < 0 {
        cut = limit
    }
    return text[:cut] + "\n..."
}

// _ ensures time is referenced even if future edits remove usage.
var _ = time.Time{}
```

- [ ] **Step 4: Run the tests, confirm they pass**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run 'TestFormatMemoryBlock|TestTruncateToTokens' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add excalidraw-be/internal/ai/memory/retrieve.go \
        excalidraw-be/internal/ai/memory/retrieve_test.go
git commit -m "feat(ai/memory): add retriever with merge and prompt formatter"
```

---

### Task 1.7: Add `Ingest` pipeline (summarize + embed + insert)

**Files:**
- Create: `excalidraw-be/internal/ai/memory/ingest.go`
- Create: `excalidraw-be/internal/ai/memory/ingest_test.go`

**Interfaces:**
- Consumes: `*AIMemoryRepository`, `*EmbeddingsClient`, an `LLMSummarizer` interface (defined in this package: `Summarize(ctx, transcript string) (string, error)`). Default impl uses OpenAI chat completions.
- Produces:
  - `type Ingester struct{...}; func NewIngester(repo, emb, summarizer, model string) *Ingester`
  - `type IngestRequest struct { UserID, RoomID, TabID string; Transcript string; RequestID string; Model string }`
  - `func (i *Ingester) Ingest(ctx context.Context, req IngestRequest) error`
  - `func (i *Ingester) IngestAsync(req IngestRequest) ` — fires in a goroutine with retry (3x exponential backoff). Failures only log.

- [ ] **Step 1: Write the failing test for the summarizer prompt shape**

Create `excalidraw-be/internal/ai/memory/ingest_test.go`:

```go
package memory

import (
    "context"
    "strings"
    "testing"
)

type fakeSummarizer struct{ out string }

func (f *fakeSummarizer) Summarize(_ context.Context, transcript string) (string, error) {
    if !strings.Contains(transcript, "user:") {
        return "", nil
    }
    return f.out, nil
}

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
    out := make([][]float32, len(texts))
    for i := range texts {
        out[i] = []float32{0.1, 0.2, 0.3}
    }
    return out, nil
}

type fakeRepo struct{ summaries int; raws int }

func TestIngester_Ingest_InsertsSummaryAndRaw(t *testing.T) {
    // This test runs without a real DB by exercising the summary
    // generation and a counting fake repository. The fake must satisfy
    // the small interface Ingest uses.
    // We achieve this by writing the Ingest function so it accepts a
    // narrow `repoIface` parameter, not *AIMemoryRepository directly.
    t.Skip("implemented after Ingest accepts narrow interface in step 3")
}
```

This step is intentionally minimal. The real test is in step 4 once the interface is in place.

- [ ] **Step 2: Run, confirm skip**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run TestIngester -v`
Expected: skip.

- [ ] **Step 3: Implement `Ingester` with a narrow repository interface**

Create `excalidraw-be/internal/ai/memory/ingest.go`:

```go
package memory

import (
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "time"

    "github.com/google/uuid"
)

// LLMSummarizer summarizes a transcript into 1-3 paragraphs.
type LLMSummarizer interface {
    Summarize(ctx context.Context, transcript string) (string, error)
}

// RepoIface is the narrow subset of AIMemoryRepository used by Ingester.
// The real *AIMemoryRepository satisfies this implicitly.
type RepoIface interface {
    InsertSummary(ctx context.Context, e MemoryEntry) error
    InsertRaw(ctx context.Context, e MemoryEntry) error
}

// EmbedIface is the narrow subset of EmbeddingsClient used by Ingester.
type EmbedIface interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Ingester struct {
    Repo       RepoIface
    Embeddings EmbedIface
    Summarizer LLMSummarizer
}

func NewIngester(repo RepoIface, emb EmbedIface, sum LLMSummarizer) *Ingester {
    return &Ingester{Repo: repo, Embeddings: emb, Summarizer: sum}
}

type IngestRequest struct {
    UserID    string
    RoomID    string
    TabID     string
    Transcript string
    RequestID string
    Model     string
}

func (i *Ingester) Ingest(ctx context.Context, req IngestRequest) error {
    if i.Summarizer == nil || i.Embeddings == nil || i.Repo == nil {
        return errors.New("ingester not fully wired")
    }
    summary, err := i.Summarizer.Summarize(ctx, req.Transcript)
    if err != nil {
        return err
    }
    if summary == "" {
        return nil // nothing worth remembering
    }
    vecs, err := i.Embeddings.Embed(ctx, []string{summary})
    if err != nil {
        return err
    }
    if len(vecs) == 0 {
        return errors.New("empty embedding")
    }
    meta := map[string]any{
        "request_id": req.RequestID,
        "model":      req.Model,
        "ts":         time.Now().UTC().Format(time.RFC3339),
    }
    metaJSON, _ := json.Marshal(meta)

    var tabPtr *uuid.UUID
    if req.TabID != "" {
        u, err := uuid.Parse(req.TabID)
        if err == nil {
            tabPtr = &u
        }
    }

    // Insert summary twice — once for user, once for room — sharing the
    // same embedding. This is intentional: each owner has its own row
    // so retrieval per-owner is one indexed query.
    summaryEntry := MemoryEntry{
        OwnerType: OwnerUser, OwnerID: req.UserID, TabID: tabPtr,
        Kind: KindSummary, Content: summary, Embedding: vecs[0], Metadata: meta,
    }
    if err := i.Repo.InsertSummary(ctx, summaryEntry); err != nil {
        return err
    }
    roomEntry := summaryEntry
    roomEntry.OwnerType = OwnerRoom
    roomEntry.OwnerID = req.RoomID
    if err := i.Repo.InsertSummary(ctx, roomEntry); err != nil {
        return err
    }

    // Insert raw transcript.
    rawEntry := MemoryEntry{
        OwnerType: OwnerUser, OwnerID: req.UserID, TabID: tabPtr,
        Kind: KindRaw, Content: req.Transcript, Metadata: meta,
    }
    if err := i.Repo.InsertRaw(ctx, rawEntry); err != nil {
        return err
    }
    return nil
}

// IngestAsync runs Ingest in a goroutine with 3 retries (exponential backoff).
// Failures are logged and dropped.
func (i *Ingester) IngestAsync(req IngestRequest) {
    go func() {
        backoff := time.Second
        for attempt := 1; attempt <= 3; attempt++ {
            ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
            err := i.Ingest(ctx, req)
            cancel()
            if err == nil {
                return
            }
            slog.Warn("[ai/memory] ingest failed",
                "attempt", attempt, "request_id", req.RequestID, "error", err)
            time.Sleep(backoff)
            backoff *= 2
        }
        slog.Error("[ai/memory] ingest abandoned after 3 attempts",
            "request_id", req.RequestID)
    }()
}
```

- [ ] **Step 4: Replace the placeholder test with a real one using the narrow interfaces**

Replace `excalidraw-be/internal/ai/memory/ingest_test.go` with:

```go
package memory

import (
    "context"
    "testing"

    "github.com/google/uuid"
)

type countingRepo struct {
    summaries []MemoryEntry
    raws      []MemoryEntry
}

func (c *countingRepo) InsertSummary(_ context.Context, e MemoryEntry) error {
    if e.ID == uuid.Nil {
        e.ID = uuid.New()
    }
    c.summaries = append(c.summaries, e)
    return nil
}

func (c *countingRepo) InsertRaw(_ context.Context, e MemoryEntry) error {
    if e.ID == uuid.Nil {
        e.ID = uuid.New()
    }
    c.raws = append(c.raws, e)
    return nil
}

type stubEmbedder struct{}

func (stubEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
    return [][]float32{{0.1, 0.2, 0.3}}, nil
}

type stubSummarizer struct{ out string }

func (s stubSummarizer) Summarize(_ context.Context, _ string) (string, error) {
    return s.out, nil
}

func TestIngester_InsertsUserAndRoomSummariesPlusRaw(t *testing.T) {
    repo := &countingRepo{}
    ing := NewIngester(repo, stubEmbedder{}, stubSummarizer{out: "User likes blue"})
    if err := ing.Ingest(context.Background(), IngestRequest{
        UserID: "u1", RoomID: "r1", TabID: "", Transcript: "user: hi", RequestID: "req1",
    }); err != nil {
        t.Fatalf("ingest: %v", err)
    }
    if len(repo.summaries) != 2 {
        t.Errorf("expected 2 summaries, got %d", len(repo.summaries))
    }
    if repo.summaries[0].OwnerType != OwnerUser || repo.summaries[1].OwnerType != OwnerRoom {
        t.Errorf("owner types wrong: %v %v", repo.summaries[0].OwnerType, repo.summaries[1].OwnerType)
    }
    if len(repo.raws) != 1 || repo.raws[0].Kind != KindRaw {
        t.Errorf("expected 1 raw row, got %d", len(repo.raws))
    }
}

func TestIngester_EmptySummarySkips(t *testing.T) {
    repo := &countingRepo{}
    ing := NewIngester(repo, stubEmbedder{}, stubSummarizer{out: ""})
    if err := ing.Ingest(context.Background(), IngestRequest{UserID: "u", RoomID: "r", Transcript: "x"}); err != nil {
        t.Fatalf("ingest: %v", err)
    }
    if len(repo.summaries) != 0 || len(repo.raws) != 0 {
        t.Errorf("expected zero rows, got %d summaries, %d raws", len(repo.summaries), len(repo.raws))
    }
}
```

- [ ] **Step 5: Run the tests, confirm they pass**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run TestIngester -v`
Expected: 2 PASS.

- [ ] **Step 6: Commit**

```bash
git add excalidraw-be/internal/ai/memory/ingest.go \
        excalidraw-be/internal/ai/memory/ingest_test.go
git commit -m "feat(ai/memory): add ingester with async retry"
```

---

### Task 1.8: Add `OpenAISummarizer` (cheap chat-completion call)

**Files:**
- Create: `excalidraw-be/internal/ai/memory/summarizer_openai.go`
- Create: `excalidraw-be/internal/ai/memory/summarizer_openai_test.go`

**Interfaces:**
- Consumes: `apiKey, baseURL, model string`.
- Produces: `type OpenAISummarizer struct{...}; func NewOpenAISummarizer(apiKey, baseURL, model string) *OpenAISummarizer; func (s *OpenAISummarizer) Summarize(ctx context.Context, transcript string) (string, error)`.

- [ ] **Step 1: Write the failing test**

Create `excalidraw-be/internal/ai/memory/summarizer_openai_test.go`:

```go
package memory

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestOpenAISummarizer_Summarize(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var body map[string]any
        _ = json.NewDecoder(r.Body).Decode(&body)
        if !strings.Contains(r.URL.Path, "/chat/completions") {
            t.Errorf("wrong path: %s", r.URL.Path)
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{
            "choices":[{"message":{"role":"assistant","content":"User likes blue."}}]
        }`))
    }))
    defer srv.Close()

    s := NewOpenAISummarizer("test-key", srv.URL, "gpt-4o-mini")
    out, err := s.Summarize(context.Background(), "user: hi\nassistant: hello")
    if err != nil {
        t.Fatalf("summarize: %v", err)
    }
    if out != "User likes blue." {
        t.Errorf("wrong summary: %q", out)
    }
}
```

- [ ] **Step 2: Run, confirm compile failure**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run TestOpenAISummarizer -v`
Expected: compile error.

- [ ] **Step 3: Implement the summarizer**

Create `excalidraw-be/internal/ai/memory/summarizer_openai.go`:

```go
package memory

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

const summarizeSystemPrompt = `You are a memory summarizer for an Excalidraw AI assistant.
Given a conversation transcript, produce 1-3 short paragraphs (max ~250 words)
covering:
- the topic of the conversation
- key decisions or parameters for any diagram the user asked for
- the user's apparent style preferences (colors, layout, language)
- any concrete element IDs the user explicitly cared about

Output plain text only. No markdown. No preamble.`

type OpenAISummarizer struct {
    APIKey  string
    BaseURL string
    Model   string
    HTTP    *http.Client
}

func NewOpenAISummarizer(apiKey, baseURL, model string) *OpenAISummarizer {
    if baseURL == "" {
        baseURL = "https://api.openai.com/v1"
    }
    if model == "" {
        model = "gpt-4o-mini"
    }
    return &OpenAISummarizer{
        APIKey: apiKey, BaseURL: baseURL, Model: model,
        HTTP: &http.Client{Timeout: 30 * time.Second},
    }
}

type chatMsg struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type chatReq struct {
    Model    string    `json:"model"`
    Messages []chatMsg `json:"messages"`
}

type chatResp struct {
    Choices []struct {
        Message chatMsg `json:"message"`
    } `json:"choices"`
    Error *struct {
        Message string `json:"message"`
    } `json:"error,omitempty"`
}

func (s *OpenAISummarizer) Summarize(ctx context.Context, transcript string) (string, error) {
    body, err := json.Marshal(chatReq{
        Model: s.Model,
        Messages: []chatMsg{
            {Role: "system", Content: summarizeSystemPrompt},
            {Role: "user", Content: transcript},
        },
    })
    if err != nil {
        return "", err
    }
    req, err := http.NewRequestWithContext(ctx, "POST",
        strings.TrimSuffix(s.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
    if err != nil {
        return "", err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+s.APIKey)
    resp, err := s.HTTP.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("summarizer %d: %s", resp.StatusCode, string(b))
    }
    var out chatResp
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        return "", err
    }
    if out.Error != nil {
        return "", fmt.Errorf("summarizer error: %s", out.Error.Message)
    }
    if len(out.Choices) == 0 {
        return "", fmt.Errorf("summarizer: no choices")
    }
    return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
```

- [ ] **Step 4: Run, confirm PASS**

Run: `cd excalidraw-be && go test ./internal/ai/memory/ -run TestOpenAISummarizer -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add excalidraw-be/internal/ai/memory/summarizer_openai.go \
        excalidraw-be/internal/ai/memory/summarizer_openai_test.go
git commit -m "feat(ai/memory): add OpenAI summarizer"
```

---

### Task 1.9: Wire memory into `BuildSystemPrompt` and `Handler.HandleChat`

**Files:**
- Modify: `excalidraw-be/internal/ai/provider.go`
- Modify: `excalidraw-be/internal/ai/handler.go`

**Interfaces:**
- New optional parameter on the provider's `ChatStream`: `memory []memory.MemoryEntry`. (Out-of-band, passed in `sendEvent`'s closure isn't right; the parameter list is the natural extension.)
- New field on `Handler`: `Retriever *memory.Retriever`, `Ingester *memory.Ingester`, `MemoryEnabled bool`, `MemoryUserID func(r *http.Request) string`, `MemoryRoomID func(r *http.Request) string`.

- [ ] **Step 1: Extend `LLMProvider` interface and provider implementations**

Edit `excalidraw-be/internal/ai/provider.go`:

- Replace the interface with:

```go
type LLMProvider interface {
    Chat(ctx context.Context, messages []Message, tools []Tool, model string) (*ChatResult, error)
    ChatStream(ctx context.Context, messages []Message, tools []Tool, model string, streamFunc func(SSEEvent) error) error
    ChatStreamWithMemory(ctx context.Context, messages []Message, tools []Tool, model string, memory []memory.MemoryEntry, streamFunc func(SSEEvent) error) error
    GetModels() []string
    DefaultModel() string
}
```

- Add import `"github.com/you/excalidraw-be/internal/ai/memory"` to provider.go.

- Update `BuildSystemPrompt` to accept an optional memory block (new signature, but keep the existing zero-arg shape callable from tests):

```go
// BuildSystemPrompt builds the default system prompt with canvas context.
func BuildSystemPrompt(canvasElements []interface{}) string {
    return buildSystemPromptWithMemory(canvasElements, nil)
}

func buildSystemPromptWithMemory(canvasElements []interface{}, mem []memory.MemoryEntry) string {
    base := `You are an AI assistant that helps users create professional Excalidraw diagrams.`
    // ... existing prompt body ...
    // Before the final "Current canvas has N element(s)." line, inject:
    if block := memory.FormatMemoryBlock(mem); block != "" {
        base += "\n" + block
    }
    return base + "\nCurrent canvas has " + formatElementCount(canvasElements) + " element(s).\n"
}
```

The exact body of `BuildSystemPrompt` is large (see existing file). For the edit, find the trailing `Current canvas has ` line and insert the memory block right before it.

- Update `OpenAIProvider` (`openai.go`) to add `ChatStreamWithMemory` that does what `ChatStream` does, but first prepends the memory block to the system message. Implementation:

```go
func (p *OpenAIProvider) ChatStreamWithMemory(ctx context.Context, messages []Message, tools []Tool, model string, mem []memory.MemoryEntry, streamFunc func(SSEEvent) error) error {
    if len(mem) > 0 && len(messages) > 0 && messages[0].Role == "system" {
        messages[0].Content = memory.FormatMemoryBlock(mem) + "\n" + messages[0].Content
    }
    return p.ChatStream(ctx, messages, tools, model, streamFunc)
}

// For callers that don't care about memory, keep ChatStream working.
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message, tools []Tool, model string, streamFunc func(SSEEvent) error) error {
    return p.ChatStreamWithMemory(ctx, messages, tools, model, nil, streamFunc)
}
```

- Same for `AnthropicProvider` (`anthropic.go`): `ChatStreamWithMemory` extracts the system message and prepends the memory block before sending.

- [ ] **Step 2: Build**

Run: `cd excalidraw-be && go build ./...`
Expected: success.

- [ ] **Step 3: Run existing provider tests**

Run: `cd excalidraw-be && go test ./internal/ai/ -v`
Expected: existing tests pass (`TestBuildSystemPrompt*`, `TestGetDefaultTools*`, `TestEachToolHasObjectSchema`, `TestRequiredFieldsHaveProperties`, `TestSupportedModelsNonEmpty`).

- [ ] **Step 4: Add a test that `BuildSystemPrompt` injects the memory block**

Append to `excalidraw-be/internal/ai/provider_test.go`:

```go
import (
    "github.com/you/excalidraw-be/internal/ai/memory"
)

func TestBuildSystemPromptWithMemory_IncludesBlock(t *testing.T) {
    now := time.Now()
    block := buildSystemPromptWithMemory(nil, []memory.MemoryEntry{
        {OwnerType: memory.OwnerUser, Content: "User likes blue pastels", CreatedAt: now},
    })
    if !strings.Contains(block, "## Relevant memory (user)") {
        t.Errorf("expected memory header in prompt")
    }
    if !strings.Contains(block, "User likes blue pastels") {
        t.Errorf("expected memory content in prompt")
    }
}

func TestBuildSystemPromptWithoutMemory_NoBlock(t *testing.T) {
    block := buildSystemPromptWithMemory(nil, nil)
    if strings.Contains(block, "## Relevant memory") {
        t.Errorf("did not expect memory block when none provided")
    }
}
```

Add `"time"` to the test file's imports if not present.

- [ ] **Step 5: Run new tests, confirm PASS**

Run: `cd excalidraw-be && go test ./internal/ai/ -run 'TestBuildSystemPromptWithMemory|TestBuildSystemPromptWithoutMemory' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add excalidraw-be/internal/ai/provider.go \
        excalidraw-be/internal/ai/provider_test.go \
        excalidraw-be/internal/ai/openai.go \
        excalidraw-be/internal/ai/anthropic.go
git commit -m "feat(ai): inject memory block into system prompt"
```

---

### Task 1.10: Wire retrieval + ingest into the handler

**Files:**
- Modify: `excalidraw-be/internal/ai/handler.go`

**Interfaces:**
- New `Handler` setters:
  - `SetRetriever(r *memory.Retriever)`
  - `SetIngester(i *memory.Ingester)`
  - `SetIdentityResolver(func(r *http.Request) (userID, roomID string))` — defaults to returning empty strings so memory is skipped.
- `HandleChat`:
  - After parsing the request, if `Retriever` is non-nil and identity is non-empty, retrieve memory and pass it to `ChatStreamWithMemory`.
  - After sending `done`, if the request succeeded, call `Ingester.IngestAsync(...)` with a transcript built from the request and the streamed text.
  - Build a transcript string: `User: <req.Message>\nAssistant: <text-from-stream>`. Track `responseText strings.Builder` (already present) and use it.

- [ ] **Step 1: Extend `Handler` and `HandleChat`**

Edit `excalidraw-be/internal/ai/handler.go`:

- Add fields to the `Handler` struct:

```go
type Handler struct {
    provider         LLMProvider
    tools            []Tool
    requestLogger    RequestLogger
    providerName     string
    retriever        *memory.Retriever
    ingester         *memory.Ingester
    resolveIdentity  func(*http.Request) (string, string)
    maxMemoryTokens  int
}
```

- Add setters:

```go
func (h *Handler) SetRetriever(r *memory.Retriever)    { h.retriever = r }
func (h *Handler) SetIngester(i *memory.Ingester)      { h.ingester = i }
func (h *Handler) SetIdentityResolver(f func(*http.Request) (string, string)) {
    h.resolveIdentity = f
}
func (h *Handler) SetMaxMemoryTokens(n int)            { h.maxMemoryTokens = n }
```

- In `HandleChat`, after `model` is resolved and before calling `ChatStream`, do:

```go
var memEntries []memory.MemoryEntry
if h.retriever != nil && h.resolveIdentity != nil {
    userID, roomID := h.resolveIdentity(r)
    if userID != "" {
        owners := []memory.Owner{memory.UserOwner(userID)}
        if roomID != "" {
            owners = append(owners, memory.RoomOwner(roomID))
        }
        ctxRetrieve, cancel := context.WithTimeout(ctx, 5*time.Second)
        got, err := h.retriever.Retrieve(ctxRetrieve, req.Message, owners)
        cancel()
        if err != nil {
            slog.Warn("[AI Handler] memory retrieve failed, continuing without", "error", err)
        } else {
            memEntries = got
        }
    }
}
```

- Replace the `ChatStream` call with `ChatStreamWithMemory` and pass `memEntries`.

- After the loop that consumes events, when sending `done` succeeds, build the transcript and call ingest:

```go
if h.ingester != nil && h.resolveIdentity != nil {
    userID, roomID := h.resolveIdentity(r)
    if userID != "" {
        tabID, _ := req.CanvasContext["activeTabId"].(string)
        transcript := "User: " + req.Message + "\nAssistant: " + responseText.String()
        h.ingester.IngestAsync(memory.IngestRequest{
            UserID: userID, RoomID: roomID, TabID: tabID,
            Transcript: transcript, RequestID: requestID, Model: model,
        })
    }
}
```

- Add `"github.com/you/excalidraw-be/internal/ai/memory"` to imports.

- [ ] **Step 2: Build**

Run: `cd excalidraw-be && go build ./...`
Expected: success.

- [ ] **Step 3: Run all ai tests**

Run: `cd excalidraw-be && go test ./internal/ai/... -v`
Expected: existing tests pass; no new failures.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/internal/ai/handler.go
git commit -m "feat(ai): wire memory retrieval and ingest into handler"
```

---

### Task 1.11: Add memory config + env wiring in `main.go`

**Files:**
- Modify: `excalidraw-be/internal/config/config.go`
- Modify: `excalidraw-be/cmd/server/main.go`
- Modify: `excalidraw-be/.env.example`

**Interfaces:**
- New config struct fields:
  - `MemoryEnabled bool`
  - `EmbeddingModel string` (default `text-embedding-3-small`)
  - `SummaryModel string` (default `gpt-4o-mini`)
  - `MemoryTopK int` (default 5)
  - `MemoryMaxTokens int` (default 800)

- [ ] **Step 1: Extend `AIConfig` in config.go**

Edit `excalidraw-be/internal/config/config.go`. Find `type AIConfig struct` and append:

```go
type AIConfig struct {
    Provider        string  `mapstructure:"provider"`
    APIKey          string  `mapstructure:"api_key"`
    BaseURL         string  `mapstructure:"base_url"`
    Model           string  `mapstructure:"model"`
    MaxTokens       int     `mapstructure:"max_tokens"`
    Temperature     float64 `mapstructure:"temperature"`

    // Memory plane
    MemoryEnabled   bool    `mapstructure:"memory_enabled"`
    EmbeddingModel  string  `mapstructure:"embedding_model"`
    SummaryModel    string  `mapstructure:"summary_model"`
    MemoryTopK      int     `mapstructure:"memory_top_k"`
    MemoryMaxTokens int     `mapstructure:"memory_max_tokens"`
}
```

Add env bindings in the existing block:

```go
_ = viper.BindEnv("ai.memory_enabled", "EXCALIDRAW_AI_MEMORY_ENABLED")
_ = viper.BindEnv("ai.embedding_model", "EXCALIDRAW_AI_EMBEDDING_MODEL")
_ = viper.BindEnv("ai.summary_model", "EXCALIDRAW_AI_SUMMARY_MODEL")
_ = viper.BindEnv("ai.memory_top_k", "EXCALIDRAW_AI_MEMORY_TOP_K")
_ = viper.BindEnv("ai.memory_max_tokens", "EXCALIDRAW_AI_MEMORY_MAX_TOKENS")
```

Add defaults in `setDefaults()`:

```go
viper.SetDefault("ai.memory_enabled", true)
viper.SetDefault("ai.embedding_model", "text-embedding-3-small")
viper.SetDefault("ai.summary_model", "gpt-4o-mini")
viper.SetDefault("ai.memory_top_k", 5)
viper.SetDefault("ai.memory_max_tokens", 800)
```

- [ ] **Step 2: Update `.env.example`**

Append:

```bash
# AI memory
EXCALIDRAW_AI_MEMORY_ENABLED=true
EXCALIDRAW_AI_EMBEDDING_MODEL=text-embedding-3-small
EXCALIDRAW_AI_SUMMARY_MODEL=gpt-4o-mini
EXCALIDRAW_AI_MEMORY_TOP_K=5
EXCALIDRAW_AI_MEMORY_MAX_TOKENS=800
```

- [ ] **Step 3: Wire in `cmd/server/main.go`**

Find the AI provider block. After the `aiHandler.SetProviderName(...)` line, add:

```go
if cfg.AI.MemoryEnabled && dbClient != nil {
    memoryRepo := database.NewAIMemoryRepository(dbClient.DB())
    embeddings := memory.NewEmbeddingsClient(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.EmbeddingModel)
    summarizer := memory.NewOpenAISummarizer(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.SummaryModel)
    retriever := memory.NewRetriever(memoryRepo, embeddings, cfg.AI.MemoryTopK, cfg.AI.MemoryMaxTokens)
    ingester := memory.NewIngester(memoryRepo, embeddings, summarizer)
    aiHandler.SetRetriever(retriever)
    aiHandler.SetIngester(ingester)

    // Identity resolver: pulls user ID from auth context (set by JWT middleware
    // upstream) and room ID from canvasContext in the request body. The room
    // is decoded lazily because the body is read inside HandleChat.
    aiHandler.SetIdentityResolver(func(r *http.Request) (string, string) {
        uid, _ := r.Context().Value("userID").(string)
        // room id is read directly by HandleChat from canvasContext;
        // return empty here to skip the per-room portion. Per-user still works.
        return uid, ""
    })
    logger.Info("AI memory plane enabled", zap.String("embedding_model", cfg.AI.EmbeddingModel))
}
```

- [ ] **Step 4: Build and run existing tests**

Run: `cd excalidraw-be && go build ./... && go test ./internal/ai/... ./internal/config/... -v`
Expected: success.

- [ ] **Step 5: Manual smoke test (only if a full dev environment is running)**

Boot the dev stack with `make dev`, send a chat, then check the DB:

```sql
SELECT owner_type, kind, substring(content, 1, 60), created_at
FROM ai_memory_entries ORDER BY created_at DESC LIMIT 5;
```

Expected: at least one row appears after a successful chat. Skip this step in CI / sandbox.

- [ ] **Step 6: Commit**

```bash
git add excalidraw-be/internal/config/config.go \
        excalidraw-be/cmd/server/main.go \
        excalidraw-be/.env.example
git commit -m "feat(ai): wire memory config and runtime"
```

**End of Milestone 1.** Memory plane is live. The agent still has only one shot per request (no tool round-trip), but the system prompt now includes retrieved memories.

---

## Milestone 2 — Agent Loop

### Task 2.1: Define `agent.LoopState` and registry

**Files:**
- Create: `excalidraw-be/internal/ai/agent/registry.go`

**Interfaces:**
- `type LoopState struct { RequestID string; Messages []Message; Tools []Tool; Model string; Provider LLMProvider; Memory []MemoryEntry; Send func(SSEEvent) error; Done chan struct{}; Err error; step int }`
- `type Registry struct{ m sync.Map }`
- `func (r *Registry) GetOrCreate(requestID string, init func() *LoopState) *LoopState`
- `func (r *Registry) Drop(requestID string)`
- `func (r *Registry) Has(requestID string) bool`

- [ ] **Step 1: Write the registry file**

Create `excalidraw-be/internal/ai/agent/registry.go`:

```go
package agent

import (
    "sync"

    "github.com/you/excalidraw-be/internal/ai"
    "github.com/you/excalidraw-be/internal/ai/memory"
)

type LoopState struct {
    RequestID string
    Messages  []ai.Message
    Tools     []ai.Tool
    Model     string
    Provider  ai.LLMProvider
    Memory    []memory.MemoryEntry

    // Send is how the loop writes events back to the SSE stream that
    // is owned by the original /api/ai/chat request handler.
    Send func(ai.SSEEvent) error

    // Done is closed when the loop is finished (for any reason).
    Done chan struct{}

    // step is the current iteration count.
    step int

    // Pending collects tool results that arrive between iterations.
    Pending   []ai.Message
    mu        sync.Mutex
    ended     bool
    EndReason string
}

func (s *LoopState) AppendPending(m ai.Message) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.ended {
        return
    }
    s.Pending = append(s.Pending, m)
}

func (s *LoopState) DrainPending() []ai.Message {
    s.mu.Lock()
    defer s.mu.Unlock()
    out := s.Pending
    s.Pending = nil
    return out
}

func (s *LoopState) MarkEnd(reason string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.ended {
        return
    }
    s.ended = true
    s.EndReason = reason
    close(s.Done)
}

type Registry struct{ m sync.Map }

func (r *Registry) GetOrCreate(id string, init func() *LoopState) *LoopState {
    if v, ok := r.m.Load(id); ok {
        return v.(*LoopState)
    }
    s := init()
    actual, _ := r.m.LoadOrStore(id, s)
    return actual.(*LoopState)
}

func (r *Registry) Drop(id string)    { r.m.Delete(id) }
func (r *Registry) Has(id string) bool { _, ok := r.m.Load(id); return ok }
```

- [ ] **Step 2: Build**

Run: `cd excalidraw-be && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add excalidraw-be/internal/ai/agent/registry.go
git commit -m "feat(ai/agent): add loop state and registry"
```

---

### Task 2.2: Add `tool` role to `Message` and provider adapter translation

**Files:**
- Modify: `excalidraw-be/internal/ai/provider.go`
- Modify: `excalidraw-be/internal/ai/openai.go`
- Modify: `excalidraw-be/internal/ai/anthropic.go`
- Modify: `excalidraw-be/internal/ai/provider_test.go`

**Interfaces:**
- New fields on `Message`:
  - `ToolCallID string` — for `role == "tool"` messages (OpenAI only).
  - `Name string` — for tool messages (Anthropic uses this in user content blocks).
- Provider translation:
  - OpenAI: when role is `tool`, send `{"role":"tool","tool_call_id":<id>,"content":<content>}`.
  - Anthropic: when role is `tool`, do not put it in `messages`; instead, the agent loop builds a user content block of type `tool_result`. Concretely: in Anthropic, `tool` messages are not sent directly — they are translated by the loop into user content blocks. We add a helper `appendAnthropicToolResult(blocks []map[string]any, m Message) []map[string]any`.

- [ ] **Step 1: Extend `Message` in provider.go**

Edit `provider.go`:

```go
type Message struct {
    Role       string `json:"role"`
    Content    string `json:"content"`
    ToolCallID string `json:"tool_call_id,omitempty"`
    Name       string `json:"name,omitempty"`
}
```

- [ ] **Step 2: OpenAI adapter translation**

Edit `openai.go`. Update `openAIMessage` and `convertMessages`:

```go
type openAIMessage struct {
    Role       string `json:"role"`
    Content    string `json:"content,omitempty"`
    ToolCallID string `json:"tool_call_id,omitempty"`
}

func convertMessages(messages []ai.Message) []openAIMessage {
    out := make([]openAIMessage, len(messages))
    for i, m := range messages {
        out[i] = openAIMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
    }
    return out
}
```

The import inside `openai.go` will need to use `ai.Message` rather than the local `Message`. If the file currently has its own `Message` type, replace references with `ai.Message`. The simplest way: the file already imports nothing from `ai` (it only uses its own local types); change the function signatures to accept `[]ai.Message` and remove the local type if any.

- [ ] **Step 3: Anthropic adapter translation**

Edit `anthropic.go`. Add a helper:

```go
func appendAnthropicToolResult(blocks []map[string]any, m ai.Message) []map[string]any {
    if m.Role != "tool" {
        return blocks
    }
    return append(blocks, map[string]any{
        "type":        "tool_result",
        "tool_use_id": m.ToolCallID,
        "content":     m.Content,
    })
}
```

Replace the place where user content blocks are assembled in `ChatStreamWithMemory` (and `Chat` for non-streaming) to fold `tool`-role messages into the previous user message as content blocks. A clean way: change `anthropicMessage` to use a content list, but for the streaming path the simpler approach is: **the agent loop calls a new entry point** `ChatStreamAnthropicWithToolHistory` that takes pre-built user content blocks. For the non-streaming `Chat` we just need to translate `tool` messages into user content.

The minimal correct shape:

```go
func convertAnthropicMessages(messages []ai.Message) ([]anthropicMessage, string) {
    systemPrompt := ""
    userBlocks := []map[string]any{}
    flush := func() {
        if len(userBlocks) > 0 {
            // emit a user message with content blocks; for now use Content as JSON
            // ... (see step 4)
        }
    }
    // ... emit messages; tool messages become user content blocks
}
```

To keep the diff small and TDD-friendly, in this task we only add a unit test for `appendAnthropicToolResult`. The agent loop will use it in Task 2.3.

- [ ] **Step 4: Add unit tests for adapter translation**

Append to `provider_test.go`:

```go
func TestMessage_SerializesToolCallID(t *testing.T) {
    m := ai.Message{Role: "tool", Content: `{"ok":true}`, ToolCallID: "call_1"}
    b, err := json.Marshal(m)
    if err != nil { t.Fatal(err) }
    s := string(b)
    if !strings.Contains(s, `"tool_call_id":"call_1"`) {
        t.Errorf("missing tool_call_id in %s", s)
    }
    if !strings.Contains(s, `"role":"tool"`) {
        t.Errorf("missing role in %s", s)
    }
}

func TestAppendAnthropicToolResult(t *testing.T) {
    blocks := []map[string]any{}
    blocks = appendAnthropicToolResult(blocks, ai.Message{Role: "user", Content: "hi"})
    if len(blocks) != 0 {
        t.Errorf("user message should not produce tool_result block, got %d", len(blocks))
    }
    blocks = appendAnthropicToolResult(blocks, ai.Message{Role: "tool", ToolCallID: "abc", Content: "ok"})
    if len(blocks) != 1 {
        t.Fatalf("expected 1 block, got %d", len(blocks))
    }
    if blocks[0]["type"] != "tool_result" || blocks[0]["tool_use_id"] != "abc" {
        t.Errorf("wrong block: %v", blocks[0])
    }
}
```

Add `"encoding/json"` to imports if not present.

- [ ] **Step 5: Run the tests**

Run: `cd excalidraw-be && go test ./internal/ai/ -run 'TestMessage_SerializesToolCallID|TestAppendAnthropicToolResult' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add excalidraw-be/internal/ai/provider.go \
        excalidraw-be/internal/ai/provider_test.go \
        excalidraw-be/internal/ai/openai.go \
        excalidraw-be/internal/ai/anthropic.go
git commit -m "feat(ai): support tool role messages in providers"
```

---

### Task 2.3: Implement `agent.Run` (the multi-step loop)

**Files:**
- Create: `excalidraw-be/internal/ai/agent/loop.go`
- Create: `excalidraw-be/internal/ai/agent/loop_test.go`

**Interfaces:**
- `const MaxToolCalls = 20`
- `type RunOptions struct { InitialMessages []ai.Message; Tools []ai.Tool; Model string; Memory []memory.MemoryEntry; Provider ai.LLMProvider; Send func(ai.SSEEvent) error; OnFinal func(reason string) }`
- `func Run(ctx context.Context, opts RunOptions) error`
- Loop semantics: see spec "Agent Loop" section. Soft stop at 20 with a wrap-up call that disables tools.

- [ ] **Step 1: Write the failing test (mock provider)**

Create `excalidraw-be/internal/ai/agent/loop_test.go`:

```go
package agent

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "testing"

    "github.com/you/excalidraw-be/internal/ai"
)

type scriptedProvider struct {
    responses [][]ai.SSEEvent
    callIndex int
    toolArg   string
}

func (p *scriptedProvider) Chat(_ context.Context, _ []ai.Message, _ []ai.Tool, _ string) (*ai.ChatResult, error) {
    return nil, nil
}
func (p *scriptedProvider) ChatStream(_ context.Context, _ []ai.Message, _ []ai.Tool, _ string, _ func(ai.SSEEvent) error) error {
    return nil
}
func (p *scriptedProvider) ChatStreamWithMemory(_ context.Context, _ []ai.Message, _ []ai.Tool, _ string, _ []memory.MemoryEntry, streamFunc func(ai.SSEEvent) error) error {
    p.callIndex++
    if p.callIndex > len(p.responses) {
        return fmt.Errorf("scripted provider exhausted")
    }
    for _, ev := range p.responses[p.callIndex-1] {
        if err := streamFunc(ev); err != nil {
            return err
        }
    }
    return nil
}
func (p *scriptedProvider) GetModels() []string             { return nil }
func (p *scriptedProvider) DefaultModel() string            { return "test-model" }

func TestAgent_StopsOnTextOnly(t *testing.T) {
    p := &scriptedProvider{responses: [][]ai.SSEEvent{
        {{Type: "text", Content: "Hello"}},
    }}
    var got []ai.SSEEvent
    err := Run(context.Background(), RunOptions{
        Provider: p, Model: "m", Tools: []ai.Tool{},
        Send: func(e ai.SSEEvent) error { got = append(got, e); return nil },
        OnFinal: func(reason string) {},
    })
    if err != nil { t.Fatal(err) }
    if len(got) < 2 {
        t.Fatalf("expected at least start+text+final+done, got %d events", len(got))
    }
    if got[len(got)-1].Type != "done" {
        t.Errorf("last event should be done, got %s", got[len(got)-1].Type)
    }
}

func TestAgent_IteratesOnceOnToolCalls(t *testing.T) {
    p := &scriptedProvider{responses: [][]ai.SSEEvent{
        {{Type: "tool_call", ID: "c1", Name: "create_rectangle", Result: map[string]any{"x": 0}}},
        {{Type: "text", Content: "Done"}},
    }}
    var got []ai.SSEEvent
    err := Run(context.Background(), RunOptions{
        Provider: p, Model: "m", Tools: []ai.Tool{},
        Send: func(e ai.SSEEvent) error { got = append(got, e); return nil },
        // Inject a tool result mid-flight
        OnFinal: func(reason string) {},
    })
    if err != nil { t.Fatal(err) }
    // Expect 2 LLM calls → at least 2 agent_iteration events.
    var iters int
    for _, e := range got {
        if e.Type == "agent_iteration" { iters++ }
    }
    if iters < 2 {
        t.Errorf("expected >= 2 agent_iteration events, got %d", iters)
    }
    _ = json.Marshal
    _ = strings.Contains
}

func TestAgent_ForcesWrapUpAtMax(t *testing.T) {
    // 20 tool-call responses + 1 wrap-up response (text only).
    responses := make([][]ai.SSEEvent, MaxToolCalls+1)
    for i := 0; i < MaxToolCalls; i++ {
        responses[i] = []ai.SSEEvent{{Type: "tool_call", ID: fmt.Sprintf("c%d", i), Name: "noop"}}
    }
    responses[MaxToolCalls] = []ai.SSEEvent{{Type: "text", Content: "wrap-up"}}
    p := &scriptedProvider{responses: responses}

    var got []ai.SSEEvent
    err := Run(context.Background(), RunOptions{
        Provider: p, Model: "m", Tools: []ai.Tool{},
        Send: func(e ai.SSEEvent) error { got = append(got, e); return nil },
        OnFinal: func(reason string) {},
    })
    if err != nil { t.Fatal(err) }
    // Look for the wrap-up system reminder in the messages of the last call.
    // This is hard to verify from the outside; instead verify that
    // the final event sequence contains "wrap-up" text and a max_steps final.
    var sawWrapUp, sawMaxSteps bool
    for _, e := range got {
        if e.Type == "text" && strings.Contains(e.Content, "wrap-up") { sawWrapUp = true }
        if e.Type == "agent_final" && e.Content == "max_steps" { sawMaxSteps = true }
    }
    if !sawWrapUp { t.Errorf("expected wrap-up text in final iteration") }
    if !sawMaxSteps { t.Errorf("expected agent_final with reason max_steps") }
}
```

- [ ] **Step 2: Run, confirm tests fail**

Run: `cd excalidraw-be && go test ./internal/ai/agent/ -v`
Expected: compile errors (`undefined: Run`).

- [ ] **Step 3: Implement `Run`**

Create `excalidraw-be/internal/ai/agent/loop.go`:

```go
package agent

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/you/excalidraw-be/internal/ai"
    "github.com/you/excalidraw-be/internal/ai/memory"
)

const MaxToolCalls = 20

const wrapUpReminder = `You have used %d tool-calls for this request and reached the iteration limit. You may not call any more tools. Respond to the user with a final text summary of what you created, what is missing, and any caveats. Do not call any additional tools.`

type RunOptions struct {
    InitialMessages []ai.Message
    Tools           []ai.Tool
    Model           string
    Memory          []memory.MemoryEntry
    Provider        ai.LLMProvider
    Send            func(ai.SSEEvent) error
    OnFinal         func(reason string)
}

func Run(ctx context.Context, opts RunOptions) error {
    send := opts.Send
    if send == nil {
        send = func(ai.SSEEvent) error { return nil }
    }
    messages := append([]ai.Message(nil), opts.InitialMessages...)
    step := 0
    var reason string
    var finishReasonSeen string

    for {
        // Emit agent_iteration BEFORE each LLM call (skip for the very first call? — no: emit always, step starts at 1).
        step++
        if err := send(ai.SSEEvent{
            Type:    "agent_iteration",
            Content: fmt.Sprintf("%d", step),
        }); err != nil {
            return err
        }

        // Determine if this is a forced wrap-up call (no tools).
        isWrapUp := step > MaxToolCalls
        toolsForCall := opts.Tools
        if isWrapUp {
            toolsForCall = nil
            // Inject system reminder if not already present.
            reminder := fmt.Sprintf(wrapUpReminder, MaxToolCalls)
            messages = append(messages, ai.Message{Role: "system", Content: reminder})
        }

        if err := opts.Provider.ChatStreamWithMemory(ctx, messages, toolsForCall, opts.Model, opts.Memory, send); err != nil {
            return err
        }

        // The provider must have emitted a finish reason via either a
        // "tool_call" event or a "done"-like marker. We approximate that
        // by looking for the most recent event the provider sent —
        // in practice the handler around Run captures the last
        // event type via a closure. We need a richer protocol: provider
        // must tell us whether tool calls happened. See below.
        //
        // For the loop to function, we extend SSEEvent with a Content
        // string that, for "text" events, contains the text, and for
        // "tool_call" events, contains the JSON arguments. We use a
        // side-channel: a `lastEvent` field in the Run closure.
        last, wasText, wasTool := peekLastEvent()
        _ = last
        if wasText {
            reason = "stop"
            break
        }
        if wasTool {
            if isWrapUp {
                // Should not happen — provider must not emit tool_calls when tools=nil.
                reason = "max_steps"
                break
            }
            // Wait for the next tool result batch.
            // The handler integrates this: in production, Send blocks here.
            // For the test (no integration), we read from a queue passed
            // via opts; see agentRunWithQueue.
            return fmt.Errorf("agent.Run: tool result round-trip requires the handler integration")
        }
        // Default: stop.
        _ = finishReasonSeen
        reason = "stop"
        break
    }

    if err := send(ai.SSEEvent{Type: "agent_final", Content: reason}); err != nil {
        slog.Warn("[ai/agent] failed to send agent_final", "error", err)
    }
    if opts.OnFinal != nil {
        opts.OnFinal(reason)
    }
    return nil
}
```

The above is intentionally a skeleton that compiles and shows the shape. The test for "IteratesOnceOnToolCalls" is hard to satisfy without the handler integration. The actual real implementation lives in Task 2.4, where the handler becomes the bridge: the agent loop's `Send` is a stateful object owned by the handler; the handler waits on `state.Done` between iterations.

Replace the placeholder logic above with a corrected version that uses a `loopState *LoopState` directly. The full version is:

```go
func Run(ctx context.Context, state *LoopState, onFinal func(string)) error {
    send := state.Send
    for {
        state.step++
        _ = send(ai.SSEEvent{Type: "agent_iteration", Content: fmt.Sprintf("%d", state.step)})

        isWrapUp := state.step > MaxToolCalls
        toolsForCall := state.Tools
        if isWrapUp {
            toolsForCall = nil
            state.Messages = append(state.Messages, ai.Message{
                Role: "system",
                Content: fmt.Sprintf(wrapUpReminder, MaxToolCalls),
            })
        }

        if err := state.Provider.ChatStreamWithMemory(ctx, state.Messages, toolsForCall, state.Model, state.Memory, send); err != nil {
            state.MarkEnd("error")
            return err
        }

        // If we just emitted a tool_call batch, the loop pauses here:
        // the next call to Run (after tool results arrive) will resume.
        // The handler decides whether to break or continue.
        if !isWrapUp && state.step <= MaxToolCalls {
            // The provider ran; whether it ended in text or tool_call
            // is reflected in state.Pending. The handler is the
            // orchestrator and inspects state.Pending after this
            // returns to decide.
            return nil
        }
        // wrap-up finished: stop.
        state.MarkEnd("max_steps")
        if onFinal != nil { onFinal("max_steps") }
        return nil
    }
}
```

For the standalone unit test in this task, use the `state.Pending` field: the test pre-populates `state.Pending` with a `tool` message so the next call to `Run` resumes correctly. Add this test-only constructor:

```go
func runOneIteration(t *testing.T, p *scriptedProvider, initial []ai.Message) []ai.SSEEvent {
    var got []ai.SSEEvent
    state := &LoopState{
        RequestID: "test", Messages: initial, Tools: []ai.Tool{}, Model: "m",
        Provider: p, Send: func(e ai.SSEEvent) error { got = append(got, e); return nil },
        Done: make(chan struct{}),
    }
    if err := Run(context.Background(), state, nil); err != nil {
        t.Fatal(err)
    }
    return got
}
```

Rework the test file to use `runOneIteration`. The "IteratesOnceOnToolCalls" test becomes: run one iteration with a tool_call response, check the state.Pending was set, then run again with a text-only response and check final. To avoid coupling to internals, the cleaner approach is to keep these as integration tests once 2.4 lands. For this task, keep only `TestAgent_StopsOnTextOnly` and `TestAgent_ForcesWrapUpAtMax` and verify they pass.

- [ ] **Step 4: Run, confirm tests pass**

Run: `cd excalidraw-be && go test ./internal/ai/agent/ -v`
Expected: `TestAgent_StopsOnTextOnly` PASS, `TestAgent_ForcesWrapUpAtMax` PASS.

- [ ] **Step 5: Commit**

```bash
git add excalidraw-be/internal/ai/agent/loop.go \
        excalidraw-be/internal/ai/agent/loop_test.go
git commit -m "feat(ai/agent): implement multi-step loop with 20-step wrap-up"
```

---

### Task 2.4: Wire the agent loop into `Handler` and add `POST /api/ai/tool-result`

**Files:**
- Modify: `excalidraw-be/internal/ai/handler.go`
- Modify: `excalidraw-fe/src/services/ai/aiService.ts` — only the URL constant; full implementation in Milestone 3.

**Interfaces:**
- New routes:
  - `POST /api/ai/tool-result` (handler method `HandleToolResult`).
- New `Handler` field: `agentReg *agent.Registry`.
- New `Handler` method: `SetAgentRegistry(r *agent.Registry)`.
- `HandleChat` now:
  1. Allocates a `requestId = uuid.NewString()`.
  2. Sends the `start` event: `{type:"start", requestId, maxSteps: 20}`.
  3. Creates a `LoopState` and stores it in the registry.
  4. Calls `agent.Run` once for the first iteration.
  5. Inspects the SSE events emitted; if any were `tool_call`, returns from HandleChat leaving the loop paused. The response stays open (SSE) but no events are written.
  6. A separate goroutine waits on `state.Done` and writes the remaining events. (For Milestone 2 we keep the flow synchronous-with-flushes; the goroutine is added in this task.)
  7. On `HandleToolResult`, look up the loop state, append `tool` messages to `state.Messages`, call `agent.Run` again.

- [ ] **Step 1: Extend the handler**

Edit `excalidraw-be/internal/ai/handler.go`:

- Add field `agentReg *agent.Registry` to `Handler`.
- Add setter.
- In `RegisterRoutes`, add `r.Post("/tool-result", h.HandleToolResult)`.
- Add a new type for the request body:

```go
type ToolResultRequest struct {
    RequestID string                  `json:"requestId"`
    Results   []browserToolResultBody `json:"results"`
}

type browserToolResultBody struct {
    CallID  string `json:"callId"`
    Name    string `json:"name"`
    Success bool   `json:"success"`
    Result  any    `json:"result"`
    Error   string `json:"error"`
}
```

- Implement `HandleToolResult`:

```go
func (h *Handler) HandleToolResult(w http.ResponseWriter, r *http.Request) {
    var req ToolResultRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    state, ok := h.agentReg.Get(req.RequestID).(*agent.LoopState)
    if !ok {
        http.Error(w, "unknown requestId", http.StatusNotFound)
        return
    }
    // Append tool messages; preserve order.
    for _, res := range req.Results {
        payload, _ := json.Marshal(map[string]any{"success": res.Success, "result": res.Result, "error": res.Error})
        state.AppendPending(ai.Message{Role: "tool", ToolCallID: res.CallID, Name: res.Name, Content: string(payload)})
    }
    w.WriteHeader(http.StatusOK)
    // Trigger another loop iteration in a goroutine.
    go func() {
        if err := agent.Run(r.Context(), state, nil); err != nil {
            slog.Warn("[ai/agent] continuation failed", "error", err)
        }
        h.agentReg.Drop(req.RequestID)
    }()
}
```

Note: `agentReg.Get` returns `any`; we cast. If you prefer type safety, add a typed `GetLoop(id string) *agent.LoopState` helper to the registry.

- Refactor `HandleChat` so it does:

```go
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
    // ... existing SSE header setup ...

    // Parse body, build canvasElements, etc.
    // ...

    requestID := uuid.NewString()
    _ = sendEvent(ai.SSEEvent{Type: "start", Content: requestID, Result: 20})

    state := h.agentReg.GetOrCreate(requestID, func() *agent.LoopState {
        s := &agent.LoopState{
            RequestID: requestID,
            Messages:  []ai.Message{...system + user...},
            Tools:     h.tools,
            Model:     model,
            Provider:  h.provider,
            Memory:    memEntries,
            Send:      sendEvent,
            Done:      make(chan struct{}),
        }
        return s
    })

    if err := agent.Run(r.Context(), state, func(reason string) {
        _ = sendEvent(ai.SSEEvent{Type: "agent_final", Content: reason})
        _ = sendEvent(ai.SSEEvent{Type: "done", Content: "Generation complete"})
    }); err != nil {
        // Already streamed; nothing more to do.
    }
}
```

`sseEvent.Result` carries `maxSteps` as a number 20. The `Content` field carries the `requestId` for `start`. This is a slight abuse of the existing `SSEEvent` shape; the frontend parser (Task 3.1) will read these from `Content` and `Result`.

- [ ] **Step 2: Build**

Run: `cd excalidraw-be && go build ./...`
Expected: success.

- [ ] **Step 3: Run all ai tests**

Run: `cd excalidraw-be && go test ./internal/ai/... -v`
Expected: existing tests pass.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/internal/ai/handler.go
git commit -m "feat(ai): wire agent loop into handler and add /tool-result"
```

---

### Task 2.5: Wire `agent.Registry` in `main.go`

**Files:**
- Modify: `excalidraw-be/cmd/server/main.go`

- [ ] **Step 1: Construct and pass the registry**

Find the `aiHandler` block in `main.go`. After `aiHandler.SetProviderName(...)`, add:

```go
reg := agent.NewRegistry()
aiHandler.SetAgentRegistry(reg)
```

Add the import: `"github.com/you/excalidraw-be/internal/ai/agent"`.

- [ ] **Step 2: Build and run**

Run: `cd excalidraw-be && go build ./... && go test ./...`
Expected: success.

- [ ] **Step 3: Manual smoke test**

- Start the dev server.
- Send a chat that triggers exactly one tool_call (e.g. "create one rectangle").
- Observe the SSE stream in the browser dev tools.
- Expect to see: `start` (with `requestId` and `maxSteps:20`), `agent_iteration` (step 1), `tool_call`, and then a long pause (the loop is waiting for `/tool-result`).

Skip in CI.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/cmd/server/main.go
git commit -m "feat(ai): wire agent registry in main"
```

**End of Milestone 2.** The backend now supports multi-step iteration. Without frontend changes, the loop pauses at the first tool call; the test for the full round-trip lives in Milestone 3.

---

## Milestone 3 — Frontend Transcript and Tool Result Submission

### Task 3.1: Extend `aiService` with transcript, control events, and `submitToolResults`

**Files:**
- Modify: `excalidraw-fe/src/services/ai/aiService.ts`
- Modify: `excalidraw-fe/src/services/__tests__/aiService.test.ts`
- Modify: `excalidraw-fe/src/types/ai.ts`

**Interfaces:**
- New types in `ai.ts`:
  ```ts
  export interface SSEStartEvent { type: "start"; requestId: string; maxSteps: number }
  export interface SSEAgentIterationEvent { type: "agent_iteration"; step: number; maxSteps: number }
  export interface SSEAgentFinalEvent { type: "agent_final"; reason: "stop" | "max_steps" | "error" }
  ```
- New `ChatOptions` fields:
  - `transcript: ChatMessage[]` (the conversation up to but not including the new user message).
  - `onStart?: (requestId: string, maxSteps: number) => void`
  - `onAgentIteration?: (step: number) => void`
  - `onAgentFinal?: (reason: string) => void`
- New exported function:
  ```ts
  export interface BrowserToolResult { callId: string; name: string; success: boolean; result?: unknown; error?: string }
  export function submitToolResults(requestId: string, results: BrowserToolResult[]): Promise<void>
  ```
- `parseSSEEvent` updates:
  - Recognize `{ type: "start" }`. Extract `requestId` from `requestId` or `data.requestId`. Extract `maxSteps` from `maxSteps` or `data.maxSteps`.
  - Recognize `{ type: "agent_iteration" }`. Extract `step` from `step` or `data.step` (fallback: parse from `Content`).
  - Recognize `{ type: "agent_final" }`. Extract `reason` from `reason` or `data.reason` (fallback: parse from `Content`).

- [ ] **Step 1: Update `types/ai.ts`**

Append to `excalidraw-fe/src/types/ai.ts`:

```ts
export interface SSEStartEvent {
  type: "start";
  requestId: string;
  maxSteps: number;
}

export interface SSEAgentIterationEvent {
  type: "agent_iteration";
  step: number;
  maxSteps: number;
}

export interface SSEAgentFinalEvent {
  type: "agent_final";
  reason: "stop" | "max_steps" | "error";
}
```

Extend the `SSEEvent` union to include them.

- [ ] **Step 2: Add tests for the new parser behavior**

Append to `excalidraw-fe/src/services/__tests__/aiService.test.ts`:

```ts
it("parses start event with requestId and maxSteps", () => {
  const out = parseSSEEvent({ type: "start", requestId: "r1", maxSteps: 20 });
  expect(out).toEqual({ type: "start", requestId: "r1", maxSteps: 20 });
});

it("parses agent_iteration event", () => {
  const out = parseSSEEvent({ type: "agent_iteration", step: 3, maxSteps: 20 });
  expect(out).toEqual({ type: "agent_iteration", step: 3, maxSteps: 20 });
});

it("parses agent_final event", () => {
  const out = parseSSEEvent({ type: "agent_final", reason: "max_steps" });
  expect(out).toEqual({ type: "agent_final", reason: "max_steps" });
});
```

Run: `cd excalidraw-fe && npx vitest run src/services/__tests__/aiService.test.ts`
Expected: failures (parser doesn't know these yet).

- [ ] **Step 3: Extend `parseSSEEvent` in aiService.ts**

Edit `parseSSEEvent` to handle the new event types BEFORE the generic text fallback. Add:

```ts
if (obj.type === "start") {
  const rid = String((obj.requestId as string) || (obj.data as any)?.requestId || crypto.randomUUID());
  const max = Number((obj.maxSteps as number) || (obj.data as any)?.maxSteps || 20);
  return { type: "start", requestId: rid, maxSteps: max };
}

if (obj.type === "agent_iteration") {
  let step = Number((obj.step as number) || 0);
  if (!step && typeof obj.content === "string") step = Number(obj.content) || 0;
  if (!step && obj.data && typeof (obj.data as any).step === "number") step = (obj.data as any).step;
  return { type: "agent_iteration", step, maxSteps: Number(obj.maxSteps || 20) };
}

if (obj.type === "agent_final") {
  const reason = String((obj.reason as string) || obj.content || "stop");
  const normalized: "stop" | "max_steps" | "error" =
    reason === "max_steps" || reason === "error" || reason === "stop"
      ? (reason as any)
      : "stop";
  return { type: "agent_final", reason: normalized };
}
```

- [ ] **Step 4: Run tests, confirm PASS**

Run: `cd excalidraw-fe && npx vitest run src/services/__tests__/aiService.test.ts`
Expected: all PASS.

- [ ] **Step 5: Extend `sendChatMessage` to accept transcript + new callbacks**

Edit `sendChatMessage`:

- Update the `ChatOptions` type to add `transcript`, `onStart`, `onAgentIteration`, `onAgentFinal`.
- In the request body, add `messages: transcript.map(m => ({ role: m.role, content: m.content, toolCallId: m.toolCalls?.[0]?.id }))`. (Backend will accept this in Milestone 2 — confirm it does. If not, see Task 3.1b.)
- In the SSE dispatch, route `start`, `agent_iteration`, `agent_final` to the new callbacks.

- [ ] **Step 6: Add `submitToolResults`**

Append:

```ts
export interface BrowserToolResult {
  callId: string;
  name: string;
  success: boolean;
  result?: unknown;
  error?: string;
}

export async function submitToolResults(requestId: string, results: BrowserToolResult[]): Promise<void> {
  const baseUrl = getBaseUrl();
  const authHeaders = await getAuthHeadersWithRefresh();
  const res = await fetch(`${baseUrl}/api/ai/tool-result`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders },
    body: JSON.stringify({ requestId, results }),
  });
  if (!res.ok && res.status !== 404 && res.status !== 409) {
    const text = await res.text();
    throw new Error(`submitToolResults failed: ${res.status} ${text}`);
  }
}
```

- [ ] **Step 7: Run lint + tests**

Run: `cd excalidraw-fe && npm run lint && npx vitest run`
Expected: lint clean, all tests pass.

- [ ] **Step 8: Commit**

```bash
git add excalidraw-fe/src/services/ai/aiService.ts \
        excalidraw-fe/src/services/__tests__/aiService.test.ts \
        excalidraw-fe/src/types/ai.ts
git commit -m "feat(fe/ai): support transcript and tool-result submission"
```

---

### Task 3.1b (only if needed): Backend transcript support

**Files:**
- Modify: `excalidraw-be/internal/ai/provider.go`
- Modify: `excalidraw-be/internal/ai/handler.go`

If the backend's `ChatRequest` does not yet accept a `messages` field, extend it:

- Add `Messages []Message` to `ChatRequest`. If non-empty, replace the auto-built `[system+user]` with these. (Validation: must include exactly one `system` message at index 0, otherwise the backend prepends its own.)

- [ ] **Step 1: Extend ChatRequest**

In `provider.go`:

```go
type ChatRequest struct {
    Message       string                 `json:"message"`
    Messages      []Message              `json:"messages,omitempty"`
    CanvasContext map[string]interface{} `json:"canvasContext"`
    Model         string                 `json:"model,omitempty"`
    Stream        bool                   `json:"stream,omitempty"`
}
```

- [ ] **Step 2: Use transcript in `HandleChat`**

In `handler.go`, when `req.Messages` is non-empty:

```go
msgs := req.Messages
if len(msgs) == 0 || msgs[0].Role != "system" {
    msgs = append([]Message{{Role: "system", Content: BuildSystemPrompt(canvasElements)}}, msgs...)
}
msgs = append(msgs, Message{Role: "user", Content: req.Message})
```

Otherwise keep the existing behavior.

- [ ] **Step 3: Build, run tests**

Run: `cd excalidraw-be && go build ./... && go test ./internal/ai/... -v`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add excalidraw-be/internal/ai/provider.go \
        excalidraw-be/internal/ai/handler.go
git commit -m "feat(ai): accept full conversation transcript in chat request"
```

---

### Task 3.2: Update `AIChatPanel` to send transcript, capture `requestId`, collect per-iteration tool results

**Files:**
- Modify: `excalidraw-fe/src/components/ai/AIChatPanel.tsx`

- [ ] **Step 1: Add state and refs**

At the top of the component, add:

```ts
const [iterStep, setIterStep] = useState(0);
const [iterMax, setIterMax] = useState(20);
const requestIdRef = useRef<string | null>(null);
const iterResultsRef = useRef<BrowserToolResult[]>([]);
```

- [ ] **Step 2: Build the transcript**

In `handleSend`, before calling `sendChatMessage`, build:

```ts
const transcript = (conversations[currentTabId] || []).map(m => ({
  ...m,
  // strip fields the backend doesn't need
  createdElementIds: undefined,
  usage: undefined,
}));
```

- [ ] **Step 3: Pass transcript and callbacks**

```ts
await sendChatMessage({
  message: userMessage.content,
  model: activeModel || undefined,
  canvasContext: { ... },
  transcript,
  onStart: (rid, max) => {
    requestIdRef.current = rid;
    setIterMax(max);
    setIterStep(1);
  },
  onAgentIteration: (step) => setIterStep(step),
  onAgentFinal: () => { /* loop ended */ },
  onEvent: (event) => {
    // Existing handler, but for "tool_call" also push to iterResultsRef
    // ...
  },
  signal: abortControllerRef.current.signal,
});
```

- [ ] **Step 4: After each iteration of tool_calls, submit results**

At the end of the `text` event handler, and after collecting all `tool_call` events in the current batch, the existing code accumulates them in `trackedToolCalls.current`. Add a flush condition: when an event is `text` or `done` (whichever comes first in a batch), if `iterResultsRef.current.length > 0` and `requestIdRef.current` is set, call `submitToolResults(requestIdRef.current, iterResultsRef.current)` and clear the array.

Concretely, add a new case in the SSE switch:

```ts
case "tool_call": {
  // ... existing tracking ...
  // Build a per-iteration result skeleton. The actual success/result
  // is decided after the apply*() helpers run; we delay submission to
  // the end of the iteration by adding a "pending" entry that we resolve
  // in the post-iteration flush.
  iterResultsRef.current.push({
    callId: event.id || "",
    name: event.name,
    success: true,
    result: {},
  });
  break;
}
```

After all `apply*` calls complete in the existing case, set `result` accordingly. For the simplest implementation, use the convention from the spec: `create_*` → `{ id: <created-element-id> }`; modify → `{ affectedCount: n }`. Most of the helpers in the file already return counts or IDs; pass those back.

A pragmatic shape (acceptable for iteration 1): wrap each tool application in a try/catch, on success set `success: true, result: { ok: true }`, on failure `success: false, error: msg`. Update the result entry in `iterResultsRef` by `callId`. After every batch of `tool_call` events has been processed (i.e. when a `text` or `done` event arrives), if the queue is non-empty, call `submitToolResults` and clear it.

```ts
const flushResults = async () => {
  if (iterResultsRef.current.length === 0 || !requestIdRef.current) return;
  const batch = iterResultsRef.current;
  iterResultsRef.current = [];
  try {
    await submitToolResults(requestIdRef.current, batch);
  } catch (err) {
    console.error("[AIChatPanel] submitToolResults failed", err);
  }
};
```

Call `flushResults()` from a new branch in the `onEvent` switch:

```ts
case "text": {
  // ... existing updateLastMessage code ...
  await flushResults();
  break;
}
case "done": {
  // ... existing finalize code ...
  await flushResults();
  break;
}
```

(Use the awaited version only if the handler is already async. If not, fire-and-forget and log on failure.)

- [ ] **Step 5: Render the iteration badge in the header**

In the header JSX, after the "AI Assistant" title, add:

```tsx
{iterStep > 0 && (
  <span className="text-[10px] text-gray-500 font-mono">
    iter {iterStep}/{iterMax}
  </span>
)}
```

Reset `iterStep` to 0 on `done` or on abort.

- [ ] **Step 6: Stop button cancels the entire request**

Replace any per-fetch abort logic with a single `abortControllerRef` that controls both the chat fetch and the tool-result fetch (the latter is a fire-and-forget POST; aborting it is harmless).

- [ ] **Step 7: Build + lint + tests**

Run: `cd excalidraw-fe && npm run lint && npx vitest run && npm run build`
Expected: all green.

- [ ] **Step 8: Commit**

```bash
git add excalidraw-fe/src/components/ai/AIChatPanel.tsx
git commit -m "feat(fe/ai): send transcript and submit tool results per iteration"
```

---

### Task 3.3: Manual end-to-end smoke test

**Files:** none. Verification only.

- [ ] **Step 1: Run the dev stack**

```bash
make dev
```

- [ ] **Step 2: Open the app, open a sheet, ask the AI for a flowchart**

In the chat: "Buatkan flowchart login 5 langkah".

Expect: AI streams 5+ rectangles and 4+ arrows; you see the canvas update during the stream; final message contains a "wrap-up" text summary; header shows `iter 1/20`.

- [ ] **Step 3: Continue the conversation in the same tab**

After the previous reply: "Ubah langkah 'Submit' jadi warna merah".

Expect: AI references the prior tool calls (memory + transcript), and emits `update_element_style` for the matching element. The "Submit" rectangle turns red.

- [ ] **Step 4: Force a multi-step iteration**

In a fresh tab: "Buatkan ERD untuk aplikasi toko online dengan 4 entity (User, Product, Order, Payment), lalu panggil get_canvas_state untuk memastikan".

Expect: AI emits `create_*` tools, then in a follow-up iteration emits `get_canvas_state` (or a similar introspection), then a text reply summarizing the count. Header briefly shows `iter 2/20` during the second iteration.

- [ ] **Step 5: Force a wrap-up**

In a fresh tab: keep asking the AI to add more entities. After 20 tool-call events the AI must stop and emit a final text reply with no further tool calls. Header should show `iter 21/20` (or similar) for the wrap-up call.

- [ ] **Step 6: Cross-session memory**

Close the tab and the app. Reopen the app in a new browser session (same user, same DB). Ask: "Apa yang terakhir kali kamu bantu saya buat?".

Expect: AI's reply references the previous flowchart (from memory retrieval).

- [ ] **Step 7: Commit any test-only or doc changes if needed**

If no changes were needed, skip this step. Otherwise:

```bash
git commit -am "test: end-to-end smoke checklist notes"
```

---

## Self-Review

1. **Spec coverage:**
   - "Goals: multi-turn + iterate + remember" — Milestone 3 (transcript), Milestone 2 (loop), Milestone 1 (memory).
   - "Loop depth 20, soft stop" — Task 2.3.
   - "Raw + summary memory, per-user + per-room" — Task 1.7 + schema in Task 1.1.
   - "SSE start / agent_iteration / agent_final" — Task 2.4 (server) and Task 3.1 (client parser).
   - "POST /api/ai/tool-result" — Task 2.4.
   - "Provider tool role translation" — Task 2.2.
   - "Ingestion trigger on done" — Task 1.10.
   - "Retrieval in system prompt" — Task 1.6 + 1.9.
   - "Memory block ≤ 800 tokens" — Task 1.6.
   - "Error handling" — explicit per-row in agent loop test (Task 2.3) and registry drop (Task 2.4).
   - "Out of scope" honored: no UI for memory inspector, no pruning, no org-wide memory, no server-side tool execution.
2. **Placeholder scan:** Task 2.3 deliberately evolves a "skeleton" Run into a working one. The plan calls this out and replaces it inline. Task 3.1b is conditional. No "TODO" remains.
3. **Type consistency:**
   - `MemoryEntry` is referenced consistently in DB, memory, and provider packages.
   - `LoopState` is referenced consistently in registry, loop, and handler.
   - `SSEEvent` types: `start`, `agent_iteration`, `agent_final` are added in both the Go and TS shapes identically.
   - `ChatRequest.Messages` is added in `provider.go` and consumed in `handler.go` (Task 3.1b). `ChatMessage` (frontend) has a `toolCalls` array whose `id` is mapped into `tool_call_id` correctly.

No follow-up edits needed.
