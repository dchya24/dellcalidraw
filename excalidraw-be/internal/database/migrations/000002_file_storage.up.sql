ALTER TABLE room_files ADD COLUMN IF NOT EXISTS storage_key VARCHAR(512);
CREATE INDEX IF NOT EXISTS idx_room_files_storage_key ON room_files(storage_key);
