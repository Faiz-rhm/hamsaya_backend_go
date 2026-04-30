-- Refresh-token rotation family. All sessions descended from a single login
-- share a family_id. When a rotated refresh token is presented outside the
-- grace window (reuse detection), every session in the family is revoked,
-- forcing a logout from every device that descended from the leaked token.
ALTER TABLE user_sessions ADD COLUMN IF NOT EXISTS family_id UUID;
ALTER TABLE user_sessions ADD COLUMN IF NOT EXISTS replaced_by_session_id UUID;

-- Backfill: each existing session is its own family. Active families share
-- the session id; rotated/revoked sessions get NULL since we can't recover
-- the original chain.
UPDATE user_sessions SET family_id = id WHERE family_id IS NULL AND revoked = false;

CREATE INDEX IF NOT EXISTS idx_user_sessions_family ON user_sessions(family_id) WHERE revoked = false;
CREATE INDEX IF NOT EXISTS idx_user_sessions_replaced_by ON user_sessions(replaced_by_session_id);

-- Long-lived device credentials. A device credential is a high-entropy
-- random secret stored hashed in the DB and as plaintext in the device
-- Keychain/Keystore. It is exchanged at /auth/device/login for a fresh
-- refresh token pair, allowing idle clients past the refresh window to
-- re-establish a session without showing the login screen.
CREATE TABLE IF NOT EXISTS device_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- SHA-256 of the plaintext credential. Plaintext never touches the DB.
    credential_hash VARCHAR(64) UNIQUE NOT NULL,
    install_id VARCHAR(128),
    device_name VARCHAR(255),
    platform VARCHAR(32),
    -- Optional expiry. NULL means non-expiring (until revoked).
    expires_at TIMESTAMP WITH TIME ZONE,
    revoked BOOLEAN NOT NULL DEFAULT false,
    revoked_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_device_credentials_user_active
    ON device_credentials(user_id) WHERE revoked = false;
CREATE INDEX IF NOT EXISTS idx_device_credentials_install
    ON device_credentials(user_id, install_id) WHERE revoked = false;
