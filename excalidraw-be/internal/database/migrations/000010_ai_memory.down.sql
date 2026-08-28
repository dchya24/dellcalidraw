DROP INDEX IF EXISTS ai_memory_tab_idx;
DROP INDEX IF EXISTS ai_memory_embedding_idx;
DROP INDEX IF EXISTS ai_memory_owner_idx;
DROP TABLE IF EXISTS ai_memory_entries;
-- Note: do NOT drop the vector extension; other tables may use it later.
