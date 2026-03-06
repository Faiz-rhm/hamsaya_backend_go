-- 1. Resolve duplicate emails: soft-delete all but one active user per email (keep oldest by created_at).
WITH duplicates AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY LOWER(TRIM(email)) ORDER BY created_at ASC, id ASC) AS rn
  FROM users
  WHERE deleted_at IS NULL
),
to_soft_delete AS (
  SELECT id FROM duplicates WHERE rn > 1
)
UPDATE users SET deleted_at = NOW(), updated_at = NOW()
WHERE id IN (SELECT id FROM to_soft_delete);

-- 2. Enforce unique email only for active users (deleted_at IS NULL).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
CREATE UNIQUE INDEX idx_users_email_unique_active ON users(email) WHERE deleted_at IS NULL;
