-- Add role field to users table
-- Roles: 'user' (default), 'admin', 'moderator'

-- Create role enum type
CREATE TYPE user_role AS ENUM ('user', 'admin', 'moderator');

-- Add role column to users table with default 'user'
ALTER TABLE users
ADD COLUMN role user_role NOT NULL DEFAULT 'user';

-- Create index for faster role-based queries
CREATE INDEX idx_users_role ON users(role);

-- Add comment
COMMENT ON COLUMN users.role IS 'User role: user (default), admin, or moderator';
