-- Rollback: Remove report indexes

-- Post Reports Indexes
DROP INDEX IF EXISTS idx_post_reports_status;
DROP INDEX IF EXISTS idx_post_reports_created;
DROP INDEX IF EXISTS idx_post_reports_status_created;

-- Comment Reports Indexes
DROP INDEX IF EXISTS idx_comment_reports_status;
DROP INDEX IF EXISTS idx_comment_reports_created;
DROP INDEX IF EXISTS idx_comment_reports_status_created;
DROP INDEX IF EXISTS idx_comment_reports_user;

-- User Reports Indexes
DROP INDEX IF EXISTS idx_user_reports_resolved;
DROP INDEX IF EXISTS idx_user_reports_created;
DROP INDEX IF EXISTS idx_user_reports_resolved_created;
DROP INDEX IF EXISTS idx_user_reports_reporter;

-- Business Reports Indexes
DROP INDEX IF EXISTS idx_business_reports_status;
DROP INDEX IF EXISTS idx_business_reports_created;
DROP INDEX IF EXISTS idx_business_reports_status_created;
DROP INDEX IF EXISTS idx_business_reports_user;
