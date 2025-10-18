-- Drop triggers
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP TRIGGER IF EXISTS trg_profiles_updated_at ON profiles;
DROP TRIGGER IF EXISTS trg_posts_updated_at ON posts;
DROP TRIGGER IF EXISTS trg_post_comments_updated_at ON post_comments;
DROP TRIGGER IF EXISTS trg_business_profiles_updated_at ON business_profiles;
DROP TRIGGER IF EXISTS trg_user_sessions_updated_at ON user_sessions;
DROP TRIGGER IF EXISTS trg_conversation_last_message ON messages;
DROP TRIGGER IF EXISTS trg_event_interest_counts ON event_interests;
DROP TRIGGER IF EXISTS trg_business_follow_count ON business_profile_followers;
DROP TRIGGER IF EXISTS trg_poll_vote_count ON user_polls;
DROP TRIGGER IF EXISTS trg_comment_replies_count ON post_comments;
DROP TRIGGER IF EXISTS trg_comment_likes_count ON post_comment_likes;
DROP TRIGGER IF EXISTS trg_post_shares_count ON post_shares;
DROP TRIGGER IF EXISTS trg_post_comments_count ON post_comments;
DROP TRIGGER IF EXISTS trg_post_likes_count ON post_likes;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS update_conversation_last_message();
DROP FUNCTION IF EXISTS update_event_interest_counts();
DROP FUNCTION IF EXISTS update_business_follow_count();
DROP FUNCTION IF EXISTS update_poll_vote_count();
DROP FUNCTION IF EXISTS update_comment_replies_count();
DROP FUNCTION IF EXISTS update_comment_likes_count();
DROP FUNCTION IF EXISTS update_post_shares_count();
DROP FUNCTION IF EXISTS update_post_comments_count();
DROP FUNCTION IF EXISTS update_post_likes_count();
DROP FUNCTION IF EXISTS get_nearby_posts(DOUBLE PRECISION, DOUBLE PRECISION, DOUBLE PRECISION, VARCHAR);
DROP FUNCTION IF EXISTS calculate_distance(GEOGRAPHY, GEOGRAPHY);
