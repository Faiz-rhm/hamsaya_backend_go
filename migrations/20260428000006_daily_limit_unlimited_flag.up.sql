-- Add an explicit `unlimited` flag to daily_post_limits so admins can disable
-- enforcement per post type without using a magic sentinel value. user_limit
-- still represents the cap when unlimited=false; it is ignored otherwise.

ALTER TABLE daily_post_limits
    ADD COLUMN IF NOT EXISTS unlimited BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN daily_post_limits.unlimited
    IS 'When true, post creation for this type is unmetered. user_limit ignored.';
