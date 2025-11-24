-- Rollback: Drop is_active indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_users_is_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_posts_is_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_business_profiles_is_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_business_categories_is_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_is_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_posts_type_is_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_posts_user_id_is_active;
