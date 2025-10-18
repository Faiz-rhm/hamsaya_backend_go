-- Remove role field from users table

-- Drop index
DROP INDEX IF EXISTS idx_users_role;

-- Drop role column
ALTER TABLE users
DROP COLUMN IF EXISTS role;

-- Drop role enum type
DROP TYPE IF EXISTS user_role;
