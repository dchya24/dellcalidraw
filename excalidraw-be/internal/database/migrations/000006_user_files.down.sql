-- Rollback user files migration
DROP INDEX IF EXISTS idx_user_files_updated_at;
DROP INDEX IF EXISTS idx_user_files_user_id;
DROP TABLE IF EXISTS user_files;