-- Hot-path composite indexes identified by production-readiness audit.
-- All CREATE INDEX statements use IF NOT EXISTS so the migration is safe to
-- re-run if a single index is later promoted to its own dedicated migration.

-- Profile/feed paginated reads (GetUserPosts).
CREATE INDEX IF NOT EXISTS idx_posts_user_created
    ON posts(user_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- Unread-notifications query: per-user, filtered by read flag, ordered.
CREATE INDEX IF NOT EXISTS idx_notifications_user_read_created
    ON notifications(user_id, read, created_at DESC);

-- Bidirectional block filter on feed queries.
CREATE INDEX IF NOT EXISTS idx_user_blocks_pair
    ON user_blocks(blocker_id, blocked_id);

-- Session expiry cleanup job.
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires
    ON user_sessions(expires_at);
