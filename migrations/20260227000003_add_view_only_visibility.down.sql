-- Revert: remove VIEW_ONLY from allowed visibility
ALTER TABLE posts
DROP CONSTRAINT IF EXISTS posts_visibility_check;

ALTER TABLE posts
ADD CONSTRAINT posts_visibility_check
CHECK (visibility IN ('PUBLIC', 'FRIENDS', 'PRIVATE'));

-- Reset any VIEW_ONLY posts to PUBLIC so constraint holds
UPDATE posts SET visibility = 'PUBLIC' WHERE visibility = 'VIEW_ONLY';
