-- Track when a chat message was last edited. NULL = never edited.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ;
