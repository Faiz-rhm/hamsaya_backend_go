-- ============================================================================
-- PERFORMANCE INDEXES
-- ============================================================================

-- Users & Profiles Indexes
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_oauth ON users(oauth_provider, oauth_provider_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_created_at ON users(created_at DESC) WHERE deleted_at IS NULL;

CREATE INDEX idx_profiles_location ON profiles USING GIST(location) WHERE deleted_at IS NULL;
CREATE INDEX idx_profiles_province ON profiles(province) WHERE deleted_at IS NULL;
CREATE INDEX idx_profiles_country ON profiles(country) WHERE deleted_at IS NULL;

-- Posts Indexes
CREATE INDEX idx_posts_user_id ON posts(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_business_id ON posts(business_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_type ON posts(type) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_created_at ON posts(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_location ON posts USING GIST(address_location) WHERE deleted_at IS NULL AND address_location IS NOT NULL;
CREATE INDEX idx_posts_category ON posts(category_id) WHERE deleted_at IS NULL AND type = 'SELL';
CREATE INDEX idx_posts_sold ON posts(sold) WHERE deleted_at IS NULL AND type = 'SELL';
CREATE INDEX idx_posts_province ON posts(province) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_status ON posts(status) WHERE deleted_at IS NULL;

-- Trending posts (composite index for sorting by engagement)
CREATE INDEX idx_posts_trending ON posts(total_likes, total_comments, total_shares, created_at DESC)
WHERE deleted_at IS NULL;

-- Social Interactions Indexes
CREATE INDEX idx_post_likes_post_id ON post_likes(post_id);
CREATE INDEX idx_post_likes_user_id ON post_likes(user_id);
CREATE INDEX idx_post_bookmarks_user_id ON post_bookmarks(user_id);
CREATE INDEX idx_post_bookmarks_post_id ON post_bookmarks(post_id);

-- Comments Indexes
CREATE INDEX idx_post_comments_post_id ON post_comments(post_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_post_comments_user_id ON post_comments(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_post_comments_parent ON post_comments(parent_comment_id) WHERE deleted_at IS NULL AND parent_comment_id IS NOT NULL;
CREATE INDEX idx_post_comment_likes_comment_id ON post_comment_likes(comment_id);
CREATE INDEX idx_post_comment_likes_user_id ON post_comment_likes(user_id);

-- Polls Indexes
CREATE INDEX idx_polls_post_id ON polls(post_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_poll_options_poll_id ON poll_options(poll_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_polls_user_id ON user_polls(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_polls_poll_id ON user_polls(poll_id) WHERE deleted_at IS NULL;

-- Business Indexes
CREATE INDEX idx_business_profiles_user_id ON business_profiles(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_business_profiles_location ON business_profiles USING GIST(address_location)
WHERE deleted_at IS NULL AND address_location IS NOT NULL;
CREATE INDEX idx_business_profiles_province ON business_profiles(province) WHERE deleted_at IS NULL;
CREATE INDEX idx_business_profiles_status ON business_profiles(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_business_followers_business ON business_profile_followers(business_id, created_at DESC);
CREATE INDEX idx_business_followers_user ON business_profile_followers(follower_id);
CREATE INDEX idx_business_hours_business ON business_hours(business_profile_id);

-- Authentication Indexes
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id) WHERE revoked = false;
CREATE INDEX idx_user_sessions_refresh_token ON user_sessions(refresh_token) WHERE revoked = false;
CREATE INDEX idx_user_sessions_expires_at ON user_sessions(expires_at) WHERE revoked = false;
CREATE INDEX idx_token_blacklist_token_hash ON token_blacklist(token_hash);
CREATE INDEX idx_token_blacklist_expires ON token_blacklist(expires_at);
CREATE INDEX idx_email_verifications_token ON email_verifications(token) WHERE used = false;
CREATE INDEX idx_password_reset_tokens_token ON password_reset_tokens(token) WHERE used = false;

-- MFA Indexes
CREATE INDEX idx_mfa_factors_user_id ON mfa_factors(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_mfa_challenges_challenge_id ON mfa_challenges(challenge_id) WHERE used = false;
CREATE INDEX idx_mfa_backup_codes_user ON mfa_backup_codes(user_id) WHERE used = false;

-- Notifications Indexes
CREATE INDEX idx_notifications_user_id ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id, read) WHERE read = false;
CREATE INDEX idx_notification_settings_profile ON notification_settings(profile_id);

-- Chat Indexes
CREATE INDEX idx_conversations_participants ON conversations(participant1_id, participant2_id);
CREATE INDEX idx_conversations_participant1 ON conversations(participant1_id, last_message_at DESC);
CREATE INDEX idx_conversations_participant2 ON conversations(participant2_id, last_message_at DESC);
CREATE INDEX idx_messages_conversation ON messages(conversation_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_messages_sender ON messages(sender_id) WHERE deleted_at IS NULL;

-- User Relationships Indexes
CREATE INDEX idx_user_follows_follower ON user_follows(follower_id);
CREATE INDEX idx_user_follows_following ON user_follows(following_id);
CREATE INDEX idx_user_blocks_blocker ON user_blocks(blocker_id);
CREATE INDEX idx_user_blocks_blocked ON user_blocks(blocked_id);

-- Reports Indexes
CREATE INDEX idx_post_reports_post ON post_reports(post_id);
CREATE INDEX idx_post_reports_user ON post_reports(user_id);
CREATE INDEX idx_comment_reports_comment ON comment_reports(comment_id);
CREATE INDEX idx_business_reports_business ON business_reports(business_id);
CREATE INDEX idx_user_reports_reported_user ON user_reports(reported_user);

-- Event Interests Indexes
CREATE INDEX idx_event_interests_post ON event_interests(post_id);
CREATE INDEX idx_event_interests_user ON event_interests(user_id);

-- Comments for maintainability
COMMENT ON INDEX idx_posts_trending IS 'Supports trending posts queries with engagement metrics';
COMMENT ON INDEX idx_posts_location IS 'PostGIS GIST index for location-based queries';
COMMENT ON INDEX idx_business_profiles_location IS 'PostGIS GIST index for nearby business discovery';
