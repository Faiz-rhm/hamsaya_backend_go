-- Client-generated idempotency token for post creation.
-- The mobile app persists a durable "post job" before uploading media and
-- retries it (foreground resume / WorkManager) until the post is created.
-- Without a dedupe key, a create that succeeds but whose ack is lost (app
-- killed before the client records success) would be replayed into a
-- duplicate post. The client sends a stable UUID per post job; this column
-- + partial unique index make CreatePost idempotent per user.
ALTER TABLE posts ADD COLUMN IF NOT EXISTS client_token TEXT;

-- Partial unique index: only enforced for rows that carry a token, so legacy
-- posts (NULL) are unaffected. Scoped per user so tokens can't collide across
-- accounts.
CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_user_client_token
    ON posts (user_id, client_token)
    WHERE client_token IS NOT NULL AND deleted_at IS NULL;
