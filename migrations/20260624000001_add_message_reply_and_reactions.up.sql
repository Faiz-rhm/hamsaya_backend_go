-- Chat: reply-to + emoji reactions.

-- Reply: a message can quote another message in the same conversation.
-- ON DELETE SET NULL so deleting the quoted message doesn't cascade-delete replies.
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS reply_to_message_id UUID REFERENCES messages(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_messages_reply_to ON messages(reply_to_message_id)
    WHERE reply_to_message_id IS NOT NULL;

-- Reactions: one row per (message, user, emoji). A user may react with several
-- distinct emojis to the same message, but not the same emoji twice — enforced
-- by the unique constraint, which also makes "toggle" an upsert/delete.
CREATE TABLE IF NOT EXISTS message_reactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji      VARCHAR(16) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (message_id, user_id, emoji)
);

CREATE INDEX IF NOT EXISTS idx_message_reactions_message ON message_reactions(message_id);
