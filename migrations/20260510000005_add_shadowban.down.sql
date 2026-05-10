DROP INDEX IF EXISTS idx_users_shadowbanned;
ALTER TABLE users
    DROP COLUMN IF EXISTS shadowban_reason,
    DROP COLUMN IF EXISTS shadowbanned_by,
    DROP COLUMN IF EXISTS shadowbanned_at;
