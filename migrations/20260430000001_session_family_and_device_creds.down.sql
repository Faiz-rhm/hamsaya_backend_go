DROP INDEX IF EXISTS idx_device_credentials_install;
DROP INDEX IF EXISTS idx_device_credentials_user_active;
DROP TABLE IF EXISTS device_credentials;

DROP INDEX IF EXISTS idx_user_sessions_replaced_by;
DROP INDEX IF EXISTS idx_user_sessions_family;
ALTER TABLE user_sessions DROP COLUMN IF EXISTS replaced_by_session_id;
ALTER TABLE user_sessions DROP COLUMN IF EXISTS family_id;
