-- Drop user_feedback table and indexes
DROP INDEX IF EXISTS idx_user_feedback_rating;
DROP INDEX IF EXISTS idx_user_feedback_type;
DROP INDEX IF EXISTS idx_user_feedback_created_at;
DROP INDEX IF EXISTS idx_user_feedback_user_id;
DROP TABLE IF EXISTS user_feedback;
