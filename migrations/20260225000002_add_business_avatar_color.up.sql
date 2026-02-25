-- Add avatar_color to business_profiles for placeholder color when no avatar image
ALTER TABLE business_profiles ADD COLUMN IF NOT EXISTS avatar_color VARCHAR(7);
