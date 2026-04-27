-- Daily post-type creation limits. One row per post type. Admin-editable
-- via /api/v1/admin/daily-limits. Counters live in Redis (keys
-- "daily_limit:{user_id}:{post_type}:{utc_yyyy-mm-dd}") with TTL through
-- the next UTC midnight; this table only holds the *limit*, not usage.

CREATE TABLE IF NOT EXISTS daily_post_limits (
    post_type TEXT PRIMARY KEY,
    user_limit INTEGER NOT NULL CHECK (user_limit >= 0),
    -- Multiplier applied when the post is authored on behalf of a business
    -- profile. Stored as a numeric multiplier (e.g. 2.0 = double a regular
    -- user's limit) so the admin can tune without schema migration.
    business_multiplier NUMERIC(4,2) NOT NULL DEFAULT 2.0
        CHECK (business_multiplier >= 0),
    -- Optional human-readable note shown in the admin panel for context.
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Seed sensible defaults. Admins can tune via the UI.
INSERT INTO daily_post_limits (post_type, user_limit, business_multiplier, description)
VALUES
    ('FEED',  5, 2.0, 'Standard social posts'),
    ('EVENT', 2, 2.0, 'Event posts — naturally low frequency'),
    ('SELL',  3, 2.0, 'Marketplace listings — capped to discourage spam'),
    ('PULL',  3, 2.0, 'Polls')
ON CONFLICT (post_type) DO NOTHING;
