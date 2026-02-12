-- ============================================================================
-- FULL-TEXT SEARCH SUPPORT + COMPOSITE INDEXES
-- Replaces LIKE '%term%' with PostgreSQL native full-text search (tsvector/tsquery)
-- for dramatically faster search at scale.
-- ============================================================================

-- Add tsvector columns for full-text search on posts
ALTER TABLE posts ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Populate the search_vector column from existing data
UPDATE posts SET search_vector = 
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'B')
WHERE search_vector IS NULL;

-- Create GIN index for fast full-text search on posts
CREATE INDEX IF NOT EXISTS idx_posts_search_vector 
    ON posts USING GIN(search_vector) 
    WHERE deleted_at IS NULL;

-- Create trigger to auto-update search_vector on INSERT/UPDATE
CREATE OR REPLACE FUNCTION posts_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS posts_search_vector_trigger ON posts;
CREATE TRIGGER posts_search_vector_trigger
    BEFORE INSERT OR UPDATE OF title, description ON posts
    FOR EACH ROW
    EXECUTE FUNCTION posts_search_vector_update();

-- Add tsvector for business profiles
ALTER TABLE business_profiles ADD COLUMN IF NOT EXISTS search_vector tsvector;

UPDATE business_profiles SET search_vector =
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(address, '')), 'C')
WHERE search_vector IS NULL;

CREATE INDEX IF NOT EXISTS idx_business_profiles_search_vector
    ON business_profiles USING GIN(search_vector)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION business_profiles_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.address, '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS business_profiles_search_vector_trigger ON business_profiles;
CREATE TRIGGER business_profiles_search_vector_trigger
    BEFORE INSERT OR UPDATE OF name, description, address ON business_profiles
    FOR EACH ROW
    EXECUTE FUNCTION business_profiles_search_vector_update();

-- ============================================================================
-- COMPOSITE INDEXES FOR FEED PERFORMANCE
-- ============================================================================

-- Most common feed query path: active posts sorted by recency
CREATE INDEX IF NOT EXISTS idx_posts_feed_default
    ON posts(created_at DESC)
    WHERE deleted_at IS NULL AND status = true;

-- Feed filtered by type (FEED, SELL, EVENT, PULL)
CREATE INDEX IF NOT EXISTS idx_posts_feed_by_type
    ON posts(type, created_at DESC)
    WHERE deleted_at IS NULL AND status = true;

-- Feed filtered by province
CREATE INDEX IF NOT EXISTS idx_posts_feed_by_province
    ON posts(province, created_at DESC)
    WHERE deleted_at IS NULL AND status = true;

-- Profiles full-text search (name search)
CREATE INDEX IF NOT EXISTS idx_profiles_name_search
    ON profiles(
        (LOWER(COALESCE(first_name, '') || ' ' || COALESCE(last_name, '')))
    )
    WHERE deleted_at IS NULL;

-- Hash refresh tokens column (for upcoming security improvement)
ALTER TABLE user_sessions ADD COLUMN IF NOT EXISTS refresh_token_hash TEXT;
