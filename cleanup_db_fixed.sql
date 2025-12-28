-- Cleanup script: Remove all data except admin user
-- Admin user/profile ID: f5d01bb5-5c2c-4129-b457-a630fd231728

DO $$
DECLARE
    admin_id UUID := 'f5d01bb5-5c2c-4129-b457-a630fd231728';
    deleted_count INTEGER;
BEGIN
    RAISE NOTICE 'Starting database cleanup...';
    RAISE NOTICE 'Admin ID: %', admin_id;

    -- Delete messages and conversations
    DELETE FROM messages;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % messages', deleted_count;

    DELETE FROM conversations;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % conversations', deleted_count;

    -- Delete notifications
    DELETE FROM notifications;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % notifications', deleted_count;

    -- Delete business-related data
    DELETE FROM business_attachments;
    DELETE FROM business_reports;
    DELETE FROM business_profile_followers;
    DELETE FROM business_profile_categories;
    DELETE FROM business_hours;
    DELETE FROM business_profiles;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % business profiles', deleted_count;

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
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % attachments', deleted_count;

    -- Delete polls and events
    DELETE FROM user_polls;
    DELETE FROM event_interests;
    DELETE FROM poll_options;
    DELETE FROM polls;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % polls', deleted_count;

    -- Delete posts
    DELETE FROM posts;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % posts', deleted_count;

    -- Delete user relationships (follows, blocks, reports)
    DELETE FROM user_follows;
    DELETE FROM user_blocks;
    DELETE FROM user_reports;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % user reports', deleted_count;

    -- Delete user sessions and tokens (including admin's old sessions)
    DELETE FROM user_sessions;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % user sessions', deleted_count;

    DELETE FROM token_blacklist;

    -- Delete MFA data (except admin's if exists)
    DELETE FROM mfa_backup_codes WHERE user_id != admin_id;
    DELETE FROM mfa_challenges WHERE user_id != admin_id;
    DELETE FROM mfa_factors WHERE user_id != admin_id;

    -- Delete auth-related data
    DELETE FROM email_verifications WHERE user_id != admin_id;
    DELETE FROM password_reset_tokens WHERE user_id != admin_id;

    -- Delete settings (except admin's) - uses profile_id which is same as user_id
    DELETE FROM notification_settings WHERE profile_id != admin_id;
    DELETE FROM privacy_settings WHERE profile_id != admin_id;

    -- Delete account deletion reasons (except admin's if exists)
    DELETE FROM account_deletion_reasons WHERE profile_id != admin_id;

    -- Delete profiles (except admin's) - profile id is same as user id
    DELETE FROM profiles WHERE id != admin_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % profiles', deleted_count;

    -- Delete users (except admin)
    DELETE FROM users WHERE id != admin_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % users', deleted_count;

    RAISE NOTICE '========================================';
    RAISE NOTICE 'Database cleanup completed successfully!';
    RAISE NOTICE 'Only admin user (%) remains.', admin_id;
    RAISE NOTICE '========================================';
END $$;

-- Verify admin user still exists
SELECT
    u.id,
    u.email,
    u.role,
    u.created_at,
    CASE WHEN p.id IS NOT NULL THEN 'Yes' ELSE 'No' END as has_profile
FROM users u
LEFT JOIN profiles p ON u.id = p.id
WHERE u.role = 'admin';
