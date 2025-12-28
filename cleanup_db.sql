-- Cleanup script: Remove all data except admin user
-- Admin user ID: f5d01bb5-5c2c-4129-b457-a630fd231728

BEGIN;

-- Store admin user ID
DO $$
DECLARE
    admin_id UUID := 'f5d01bb5-5c2c-4129-b457-a630fd231728';
BEGIN
    -- Delete messages and conversations
    DELETE FROM messages;
    DELETE FROM conversations;

    -- Delete notifications
    DELETE FROM notifications;

    -- Delete business-related data
    DELETE FROM business_attachments;
    DELETE FROM business_reports;
    DELETE FROM business_profile_followers;
    DELETE FROM business_profile_categories;
    DELETE FROM business_hours;
    DELETE FROM business_profiles;

    -- Delete post-related data (comments, likes, shares, bookmarks)
    DELETE FROM comment_attachments;
    DELETE FROM comment_reports;
    DELETE FROM post_comment_likes;
    DELETE FROM post_comments;
    DELETE FROM post_reports;
    DELETE FROM post_bookmarks;
    DELETE FROM post_likes;
    DELETE FROM post_shares;
    DELETE FROM attachments;

    -- Delete polls and events
    DELETE FROM user_polls;
    DELETE FROM event_interests;
    DELETE FROM poll_options;
    DELETE FROM polls;

    -- Delete posts
    DELETE FROM posts;

    -- Delete user relationships (follows, blocks, reports)
    DELETE FROM user_follows;
    DELETE FROM user_blocks;
    DELETE FROM user_reports;

    -- Delete user sessions and tokens (including admin's old sessions)
    DELETE FROM user_sessions;
    DELETE FROM token_blacklist;

    -- Delete MFA data (except admin's if exists)
    DELETE FROM mfa_backup_codes WHERE user_id != admin_id;
    DELETE FROM mfa_challenges WHERE user_id != admin_id;
    DELETE FROM mfa_factors WHERE user_id != admin_id;

    -- Delete auth-related data
    DELETE FROM email_verifications WHERE user_id != admin_id;
    DELETE FROM password_reset_tokens WHERE user_id != admin_id;

    -- Delete settings (except admin's)
    DELETE FROM notification_settings WHERE user_id != admin_id;
    DELETE FROM privacy_settings WHERE user_id != admin_id;

    -- Delete profiles (except admin's)
    DELETE FROM profiles WHERE user_id != admin_id;

    -- Delete account deletion reasons
    DELETE FROM account_deletion_reasons;

    -- Delete users (except admin)
    DELETE FROM users WHERE id != admin_id;

    -- Reset sequences if needed
    -- ALTER SEQUENCE IF EXISTS some_sequence RESTART WITH 1;

    RAISE NOTICE 'Database cleanup completed. Only admin user (%) remains.', admin_id;
END $$;

COMMIT;

-- Verify admin user still exists
SELECT id, email, role, created_at FROM users WHERE role = 'admin';
