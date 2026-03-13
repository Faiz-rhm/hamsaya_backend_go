-- Help center chat: messages from users to support (not user-to-user chat).
-- Each row is one message; content can include text and [Image: url] placeholders.
CREATE TABLE IF NOT EXISTS help_chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL DEFAULT '',
    is_from_user BOOLEAN NOT NULL DEFAULT true,
    app_version VARCHAR(50),
    device_info VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_help_chat_messages_user_id ON help_chat_messages(user_id);
CREATE INDEX IF NOT EXISTS idx_help_chat_messages_created_at ON help_chat_messages(user_id, created_at DESC);

COMMENT ON TABLE help_chat_messages IS 'Help center support messages; one thread per user.';
