-- Add business_id to post_comments so comments can be attributed to a business profile
ALTER TABLE post_comments
ADD COLUMN IF NOT EXISTS business_id UUID REFERENCES business_profiles(id) ON DELETE SET NULL;

COMMENT ON COLUMN post_comments.business_id IS 'When set, the comment was posted as this business (user_id is still the author).';
