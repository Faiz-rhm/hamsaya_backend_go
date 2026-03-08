-- Store user IDs mentioned/tagged in a comment (order preserved for client to match @name spans)
ALTER TABLE post_comments
ADD COLUMN IF NOT EXISTS mentioned_user_ids JSONB DEFAULT '[]';

COMMENT ON COLUMN post_comments.mentioned_user_ids IS 'Array of user IDs mentioned in the comment (e.g. ["uuid1","uuid2"]).';
