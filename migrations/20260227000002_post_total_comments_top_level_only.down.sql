-- Restore original behavior: total_comments counts all comments (including replies)

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

-- Optionally recalculate to count all comments again (uncomment if you need to roll back the semantic)
-- UPDATE posts p SET total_comments = (SELECT COUNT(*)::int FROM post_comments c WHERE c.post_id = p.id AND c.deleted_at IS NULL);
