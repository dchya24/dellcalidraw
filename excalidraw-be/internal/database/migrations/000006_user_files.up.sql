-- Phase 6: User File Management
-- Stores metadata about user files (actual canvas data stays in rooms)

-- User files table
CREATE TABLE IF NOT EXISTS user_files (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL DEFAULT 'Untitled',
    tab_count   INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_files_user_id ON user_files(user_id);
CREATE INDEX IF NOT EXISTS idx_user_files_updated_at ON user_files(updated_at DESC);