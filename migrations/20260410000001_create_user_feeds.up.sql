CREATE TABLE IF NOT EXISTS user_feeds (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id     UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_feeds_user_post UNIQUE (user_id, post_id)
);
CREATE INDEX idx_user_feeds_cursor ON user_feeds(user_id, created_at DESC);
