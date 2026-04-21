ALTER TABLE room_files DROP COLUMN IF EXISTS storage_key;
DROP INDEX IF EXISTS idx_room_files_storage_key;
