-- Rollback: Remove full-text search and composite indexes

-- Remove refresh_token_hash column
ALTER TABLE user_sessions DROP COLUMN IF EXISTS refresh_token_hash;

-- Remove profile name search index
DROP INDEX IF EXISTS idx_profiles_name_search;

-- Remove composite feed indexes
DROP INDEX IF EXISTS idx_posts_feed_by_province;
DROP INDEX IF EXISTS idx_posts_feed_by_type;
DROP INDEX IF EXISTS idx_posts_feed_default;

-- Remove business profile full-text search
DROP TRIGGER IF EXISTS business_profiles_search_vector_trigger ON business_profiles;
DROP FUNCTION IF EXISTS business_profiles_search_vector_update();
DROP INDEX IF EXISTS idx_business_profiles_search_vector;
ALTER TABLE business_profiles DROP COLUMN IF EXISTS search_vector;

-- Remove post full-text search
DROP TRIGGER IF EXISTS posts_search_vector_trigger ON posts;
DROP FUNCTION IF EXISTS posts_search_vector_update();
DROP INDEX IF EXISTS idx_posts_search_vector;
ALTER TABLE posts DROP COLUMN IF EXISTS search_vector;
