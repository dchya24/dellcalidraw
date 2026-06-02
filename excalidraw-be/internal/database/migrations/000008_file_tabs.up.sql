-- file_tabs: stores per-tab canvas data (elements, appState) for user files
CREATE TABLE IF NOT EXISTS file_tabs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id     UUID NOT NULL REFERENCES user_files(id) ON DELETE CASCADE,
    tab_key     VARCHAR(255) NOT NULL,
    title       VARCHAR(255) NOT NULL DEFAULT 'Sheet 1',
    room_id     VARCHAR(50),
    elements    JSONB NOT NULL DEFAULT '[]',
    app_state   JSONB NOT NULL DEFAULT '{}',
    files_data  JSONB NOT NULL DEFAULT '{}',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_tabs_file_id ON file_tabs(file_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_file_tabs_file_id_tab_key ON file_tabs(file_id, tab_key);
