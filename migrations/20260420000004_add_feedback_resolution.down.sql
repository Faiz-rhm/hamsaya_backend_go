ALTER TABLE user_feedback
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS resolved_by,
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS admin_notes;
