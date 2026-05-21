-- Per-user soft delete for chat messages. Existing `deleted_at` column is
-- the "delete for everyone" signal (sender-only). This array tracks users
-- who chose "delete for me" — message stays visible to others but the
-- listed users no longer see it.
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS deleted_for_user_ids UUID[] NOT NULL DEFAULT '{}';

-- GIN index makes the membership check (`$user_id = ANY(deleted_for_user_ids)`)
-- usable on indexed scans for active chat threads. Without it, every
-- message list query would degrade to a sequential scan on busy
-- conversations.
CREATE INDEX IF NOT EXISTS idx_messages_deleted_for_user_ids
    ON messages USING GIN (deleted_for_user_ids);
