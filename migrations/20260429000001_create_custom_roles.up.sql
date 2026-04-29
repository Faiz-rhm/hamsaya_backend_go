-- Custom named roles with permission matrices. Admins define roles here and
-- assign them to other admin users on top of (or instead of) the hard-coded
-- super_admin / admin / moderator tiers. A user's effective permission set is
-- the union of their base-role permissions and their custom-role permissions.

CREATE TABLE IF NOT EXISTS custom_roles (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(64)  NOT NULL,
    description   TEXT,
    permissions   JSONB        NOT NULL DEFAULT '[]',
    created_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    updated_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT custom_roles_name_unique UNIQUE (name),
    CONSTRAINT custom_roles_name_format CHECK (LENGTH(TRIM(name)) >= 2)
);

COMMENT ON TABLE  custom_roles             IS 'Admin-defined permission bundles assignable to individual users.';
COMMENT ON COLUMN custom_roles.permissions IS 'JSON array of permission-key strings (matches PERMISSIONS keys in src/lib/roles.ts).';

CREATE INDEX IF NOT EXISTS idx_custom_roles_name ON custom_roles(name);

-- Add nullable custom_role_id to the users table. A user may have:
--   role = 'moderator' + custom_role_id = <Finance Admin role>
-- giving them moderator base-permissions UNION Finance Admin permissions.
ALTER TABLE users ADD COLUMN IF NOT EXISTS
    custom_role_id UUID REFERENCES custom_roles(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_users_custom_role ON users(custom_role_id)
    WHERE custom_role_id IS NOT NULL;

-- Seed two starter roles so the UI has something to show on a fresh deploy.
INSERT INTO custom_roles (name, description, permissions) VALUES
    (
        'Content Manager',
        'Full content moderation: posts, comments, businesses, reports, feedback.',
        '["POSTS_VIEW","POSTS_MUTATE","POSTS_DELETE","COMMENTS_VIEW","COMMENTS_MUTATE","COMMENTS_DELETE","BUSINESSES_VIEW","BUSINESSES_APPROVE","REPORTS_VIEW","REPORTS_RESOLVE"]'
    ),
    (
        'Finance Admin',
        'Monetization oversight: ads, credits, boosts.',
        '["ADS_MANAGE","CREDITS_MANAGE","BOOSTS_MANAGE"]'
    )
ON CONFLICT (name) DO NOTHING;
