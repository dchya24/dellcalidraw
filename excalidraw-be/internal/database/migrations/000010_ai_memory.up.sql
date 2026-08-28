-- ai_memory_entries: persistent memory for the AI assistant
-- Each row is either a 'summary' (with vector embedding, used for retrieval)
-- or a 'raw' (verbatim transcript, no embedding, used for history display).

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS ai_memory_entries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type   TEXT NOT NULL,            -- 'user' | 'room'
    owner_id     TEXT NOT NULL,            -- user_id or room_id (strings in this codebase; see file_tabs.room_id)
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
