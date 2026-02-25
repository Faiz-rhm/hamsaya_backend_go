-- Add avatar_color for placeholder background when user has no avatar photo
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS avatar_color VARCHAR(7);
