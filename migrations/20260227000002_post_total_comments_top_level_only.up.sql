-- total_comments on posts should count only top-level comments (not sub-comments/replies)

-- Replace the trigger function so it only updates post total_comments for top-level comments
CREATE OR REPLACE FUNCTION update_post_comments_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- Only count top-level comments (no parent)
        IF NEW.parent_comment_id IS NULL THEN
            UPDATE posts SET total_comments = total_comments + 1 WHERE id = NEW.post_id;
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.parent_comment_id IS NULL THEN
            UPDATE posts SET total_comments = GREATEST(total_comments - 1, 0) WHERE id = OLD.post_id;
        END IF;
    ELSIF TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
        -- Soft delete: only decrement if it was a top-level comment
        IF COALESCE(OLD.parent_comment_id, NEW.parent_comment_id) IS NULL THEN
            UPDATE posts SET total_comments = GREATEST(total_comments - 1, 0) WHERE id = COALESCE(NEW.post_id, OLD.post_id);
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Recalculate total_comments for all posts to reflect only top-level comments
UPDATE posts p
SET total_comments = (
    SELECT COUNT(*)::int
    FROM post_comments c
    WHERE c.post_id = p.id
      AND c.parent_comment_id IS NULL
      AND c.deleted_at IS NULL
);
