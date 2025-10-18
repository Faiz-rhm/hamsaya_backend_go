-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Calculate distance between two geography points (returns meters)
CREATE OR REPLACE FUNCTION calculate_distance(
    point1 GEOGRAPHY,
    point2 GEOGRAPHY
) RETURNS DOUBLE PRECISION AS $$
BEGIN
    RETURN ST_Distance(point1, point2);
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Get posts within radius
CREATE OR REPLACE FUNCTION get_nearby_posts(
    user_lat DOUBLE PRECISION,
    user_lng DOUBLE PRECISION,
    radius_meters DOUBLE PRECISION,
    post_type VARCHAR DEFAULT NULL
) RETURNS TABLE (
    post_id UUID,
    distance_meters DOUBLE PRECISION
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        ST_Distance(
            p.address_location::geography,
            ST_SetSRID(ST_MakePoint(user_lng, user_lat), 4326)::geography
        ) as distance
    FROM posts p
    WHERE
        p.deleted_at IS NULL
        AND p.address_location IS NOT NULL
        AND ST_DWithin(
            p.address_location::geography,
            ST_SetSRID(ST_MakePoint(user_lng, user_lat), 4326)::geography,
            radius_meters
        )
        AND (post_type IS NULL OR p.type = post_type)
    ORDER BY distance;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- TRIGGER FUNCTIONS FOR AUTOMATIC COUNTER UPDATES
-- ============================================================================

-- Update post total_likes counter
CREATE OR REPLACE FUNCTION update_post_likes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET total_likes = total_likes + 1 WHERE id = NEW.post_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET total_likes = GREATEST(total_likes - 1, 0) WHERE id = OLD.post_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_likes_count
AFTER INSERT OR DELETE ON post_likes
FOR EACH ROW EXECUTE FUNCTION update_post_likes_count();

-- Update post total_comments counter
CREATE OR REPLACE FUNCTION update_post_comments_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET total_comments = total_comments + 1 WHERE id = NEW.post_id;
    ELSIF TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL) THEN
        UPDATE posts SET total_comments = GREATEST(total_comments - 1, 0)
        WHERE id = COALESCE(NEW.post_id, OLD.post_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_comments_count
AFTER INSERT OR UPDATE OR DELETE ON post_comments
FOR EACH ROW EXECUTE FUNCTION update_post_comments_count();

-- Update post total_shares counter
CREATE OR REPLACE FUNCTION update_post_shares_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET total_shares = total_shares + 1 WHERE id = NEW.original_post_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET total_shares = GREATEST(total_shares - 1, 0) WHERE id = OLD.original_post_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_shares_count
AFTER INSERT OR DELETE ON post_shares
FOR EACH ROW EXECUTE FUNCTION update_post_shares_count();

-- Update comment total_likes counter
CREATE OR REPLACE FUNCTION update_comment_likes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE post_comments SET total_likes = total_likes + 1 WHERE id = NEW.comment_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE post_comments SET total_likes = GREATEST(total_likes - 1, 0) WHERE id = OLD.comment_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_comment_likes_count
AFTER INSERT OR DELETE ON post_comment_likes
FOR EACH ROW EXECUTE FUNCTION update_comment_likes_count();

-- Update comment total_replies counter
CREATE OR REPLACE FUNCTION update_comment_replies_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.parent_comment_id IS NOT NULL THEN
        UPDATE post_comments SET total_replies = total_replies + 1 WHERE id = NEW.parent_comment_id;
    ELSIF (TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL))
          AND COALESCE(NEW.parent_comment_id, OLD.parent_comment_id) IS NOT NULL THEN
        UPDATE post_comments SET total_replies = GREATEST(total_replies - 1, 0)
        WHERE id = COALESCE(NEW.parent_comment_id, OLD.parent_comment_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_comment_replies_count
AFTER INSERT OR UPDATE OR DELETE ON post_comments
FOR EACH ROW EXECUTE FUNCTION update_comment_replies_count();

-- Update poll option vote_count
CREATE OR REPLACE FUNCTION update_poll_vote_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE poll_options SET vote_count = vote_count + 1 WHERE id = NEW.poll_option_id;
    ELSIF TG_OP = 'UPDATE' AND NEW.poll_option_id != OLD.poll_option_id THEN
        UPDATE poll_options SET vote_count = GREATEST(vote_count - 1, 0) WHERE id = OLD.poll_option_id;
        UPDATE poll_options SET vote_count = vote_count + 1 WHERE id = NEW.poll_option_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE poll_options SET vote_count = GREATEST(vote_count - 1, 0) WHERE id = OLD.poll_option_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_poll_vote_count
AFTER INSERT OR UPDATE OR DELETE ON user_polls
FOR EACH ROW EXECUTE FUNCTION update_poll_vote_count();

-- Update business follower count
CREATE OR REPLACE FUNCTION update_business_follow_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE business_profiles SET total_follow = total_follow + 1 WHERE id = NEW.business_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE business_profiles SET total_follow = GREATEST(total_follow - 1, 0) WHERE id = OLD.business_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_business_follow_count
AFTER INSERT OR DELETE ON business_profile_followers
FOR EACH ROW EXECUTE FUNCTION update_business_follow_count();

-- Update event interest counts
CREATE OR REPLACE FUNCTION update_event_interest_counts()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.event_state = 'interested' THEN
            UPDATE posts SET interested_count = interested_count + 1 WHERE id = NEW.post_id;
        ELSIF NEW.event_state = 'going' THEN
            UPDATE posts SET going_count = going_count + 1 WHERE id = NEW.post_id;
        END IF;
    ELSIF TG_OP = 'UPDATE' AND NEW.event_state != OLD.event_state THEN
        IF OLD.event_state = 'interested' THEN
            UPDATE posts SET interested_count = GREATEST(interested_count - 1, 0) WHERE id = NEW.post_id;
        ELSIF OLD.event_state = 'going' THEN
            UPDATE posts SET going_count = GREATEST(going_count - 1, 0) WHERE id = NEW.post_id;
        END IF;
        IF NEW.event_state = 'interested' THEN
            UPDATE posts SET interested_count = interested_count + 1 WHERE id = NEW.post_id;
        ELSIF NEW.event_state = 'going' THEN
            UPDATE posts SET going_count = going_count + 1 WHERE id = NEW.post_id;
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.event_state = 'interested' THEN
            UPDATE posts SET interested_count = GREATEST(interested_count - 1, 0) WHERE id = OLD.post_id;
        ELSIF OLD.event_state = 'going' THEN
            UPDATE posts SET going_count = GREATEST(going_count - 1, 0) WHERE id = OLD.post_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_event_interest_counts
AFTER INSERT OR UPDATE OR DELETE ON event_interests
FOR EACH ROW EXECUTE FUNCTION update_event_interest_counts();

-- Update conversation last_message_at
CREATE OR REPLACE FUNCTION update_conversation_last_message()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE conversations
        SET last_message_at = NEW.created_at
        WHERE id = NEW.conversation_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_conversation_last_message
AFTER INSERT ON messages
FOR EACH ROW EXECUTE FUNCTION update_conversation_last_message();

-- Update updated_at timestamp automatically
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger to relevant tables
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_profiles_updated_at BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_posts_updated_at BEFORE UPDATE ON posts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_post_comments_updated_at BEFORE UPDATE ON post_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_business_profiles_updated_at BEFORE UPDATE ON business_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_user_sessions_updated_at BEFORE UPDATE ON user_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for maintainability
COMMENT ON FUNCTION update_post_likes_count IS 'Automatically updates post like counter';
COMMENT ON FUNCTION update_post_comments_count IS 'Automatically updates post comment counter';
COMMENT ON FUNCTION update_poll_vote_count IS 'Automatically updates poll vote counters';
COMMENT ON FUNCTION calculate_distance IS 'Calculates distance between two points in meters';
COMMENT ON FUNCTION get_nearby_posts IS 'Returns posts within specified radius';
