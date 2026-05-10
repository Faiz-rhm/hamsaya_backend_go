-- Shadowban a user. Their posts continue to appear in their own feed
-- (so they don't realize they've been actioned and just create another
-- account), but everyone else's feed quietly excludes them. Softer than
-- a full ban — used for confirmed-but-unwilling-to-stop spammers, edge
-- abuse cases, or low-confidence-but-suspicious accounts where the
-- alternative is letting them keep harming the community.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS shadowbanned_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS shadowbanned_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS shadowban_reason TEXT;

-- Partial index so the feed-side anti-join (a hot query) only walks the
-- (small) set of currently-shadowbanned rows, not the full users table.
CREATE INDEX IF NOT EXISTS idx_users_shadowbanned
    ON users(id)
    WHERE shadowbanned_at IS NOT NULL;

COMMENT ON COLUMN users.shadowbanned_at  IS 'When admin shadowbanned this user. NULL = active. Posts hidden from non-author feeds while set.';
COMMENT ON COLUMN users.shadowbanned_by  IS 'Admin that applied the shadowban; for audit + appeal workflows.';
COMMENT ON COLUMN users.shadowban_reason IS 'Free-text rationale for the shadowban; surfaced on the admin user-detail page.';
