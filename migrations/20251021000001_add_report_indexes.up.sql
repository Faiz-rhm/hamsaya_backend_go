-- Migration: Add indexes for report tables to improve query performance
-- Created: 2025-10-21

-- Post Reports Indexes
CREATE INDEX IF NOT EXISTS idx_post_reports_status ON post_reports(report_status);
CREATE INDEX IF NOT EXISTS idx_post_reports_created ON post_reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_post_reports_status_created ON post_reports(report_status, created_at DESC);

-- Comment Reports Indexes
CREATE INDEX IF NOT EXISTS idx_comment_reports_status ON comment_reports(report_status);
CREATE INDEX IF NOT EXISTS idx_comment_reports_created ON comment_reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comment_reports_status_created ON comment_reports(report_status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comment_reports_user ON comment_reports(user_id);

-- User Reports Indexes
CREATE INDEX IF NOT EXISTS idx_user_reports_resolved ON user_reports(resolved);
CREATE INDEX IF NOT EXISTS idx_user_reports_created ON user_reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_reports_resolved_created ON user_reports(resolved, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_reports_reporter ON user_reports(reported_by_id);

-- Business Reports Indexes
CREATE INDEX IF NOT EXISTS idx_business_reports_status ON business_reports(report_status);
CREATE INDEX IF NOT EXISTS idx_business_reports_created ON business_reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_reports_status_created ON business_reports(report_status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_reports_user ON business_reports(user_id);
