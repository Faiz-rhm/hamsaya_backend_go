-- Restore original: simple UNIQUE on email for all rows
DROP INDEX IF EXISTS idx_users_email_unique_active;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
