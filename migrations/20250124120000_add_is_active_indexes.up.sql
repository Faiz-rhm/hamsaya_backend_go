-- Migration: Add indexes on is_active columns for performance
-- Critical for filtering active/inactive records

-- Users table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_is_active
ON users(is_active)
WHERE is_active = true;

-- Posts table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_is_active
ON posts(is_active)
WHERE is_active = true;

-- Business profiles table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_business_profiles_is_active
ON business_profiles(is_active)
WHERE is_active = true;

-- Business categories table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_business_categories_is_active
ON business_categories(is_active)
WHERE is_active = true;

-- Notifications table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_is_active
ON notifications(is_active)
WHERE is_active = true;

-- Composite index for posts with type and is_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_type_is_active
ON posts(type, is_active)
WHERE is_active = true;

-- Composite index for posts with user_id and is_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_user_id_is_active
ON posts(user_id, is_active)
WHERE is_active = true;

-- Add comment for documentation
COMMENT ON INDEX idx_users_is_active IS 'Partial index for active users lookup';
COMMENT ON INDEX idx_posts_is_active IS 'Partial index for active posts lookup';
COMMENT ON INDEX idx_posts_type_is_active IS 'Composite index for filtering posts by type and active status';
