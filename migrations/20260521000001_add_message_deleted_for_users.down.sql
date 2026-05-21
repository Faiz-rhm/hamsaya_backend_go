DROP INDEX IF EXISTS idx_messages_deleted_for_user_ids;
ALTER TABLE messages DROP COLUMN IF EXISTS deleted_for_user_ids;
