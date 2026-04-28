-- Boolean feature flags toggleable from /system page (super_admin only).
-- Designed for platform-level switches: registration_open, posting_enabled,
-- maintenance_mode, etc. Soft cache TTL for hot reads is enforced in the
-- service layer; the table is the source of truth.

CREATE TABLE IF NOT EXISTS feature_flags (
    key             VARCHAR(64)  PRIMARY KEY,
    enabled         BOOLEAN      NOT NULL DEFAULT FALSE,
    description     TEXT,
    updated_by      UUID         REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT      feature_flags_key_format CHECK (key ~ '^[a-z][a-z0-9_]{2,63}$')
);

COMMENT ON TABLE  feature_flags                   IS 'Platform-wide boolean toggles managed from the admin /system page.';
COMMENT ON COLUMN feature_flags.key               IS 'snake_case identifier; immutable after creation.';
COMMENT ON COLUMN feature_flags.enabled           IS 'Current value; checked at runtime by gated code paths.';
COMMENT ON COLUMN feature_flags.description       IS 'Human-readable purpose; shown in /system UI.';
COMMENT ON COLUMN feature_flags.updated_by        IS 'Last super_admin who toggled this flag.';

-- Seed common flags so the /system UI has something to show on a fresh deploy.
INSERT INTO feature_flags (key, enabled, description) VALUES
    ('registration_open',     TRUE,  'Allow new user registrations via /auth/register.'),
    ('posting_enabled',       TRUE,  'Allow users to create posts. Disable for read-only mode.'),
    ('maintenance_mode',      FALSE, 'Show a maintenance banner to all clients and reject mutating requests.')
ON CONFLICT (key) DO NOTHING;
