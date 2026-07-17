DROP INDEX IF EXISTS idx_posts_user_client_token;
ALTER TABLE posts DROP COLUMN IF EXISTS client_token;
