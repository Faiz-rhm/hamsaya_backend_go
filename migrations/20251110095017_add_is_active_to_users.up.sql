-- Add is_active column to users table
ALTER TABLE users
ADD COLUMN is_active BOOLEAN DEFAULT true NOT NULL;

-- Update existing users: mark non-deleted users as active, deleted users as inactive
UPDATE users
SET is_active = (deleted_at IS NULL);

-- Add comment for documentation
COMMENT ON COLUMN users.is_active IS 'Whether the user account is active (true) or deactivated (false)';

-- Add index for users.is_active
CREATE INDEX IF NOT EXISTS idx_users_is_active
ON users(is_active)
WHERE is_active = true;
