-- Allow VIEW_ONLY visibility for FEED posts (view-only: no likes/comments)
ALTER TABLE posts
DROP CONSTRAINT IF EXISTS posts_visibility_check;

ALTER TABLE posts
ADD CONSTRAINT posts_visibility_check
CHECK (visibility IN ('PUBLIC', 'FRIENDS', 'PRIVATE', 'VIEW_ONLY'));
