-- Add super_admin tier to user_role enum and admin_invites CHECK constraint.
-- Auto-promote the oldest existing admin to super_admin when no super_admin
-- yet exists, so deployments inherit a working bootstrap account.

ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'super_admin';

-- ALTER TYPE ... ADD VALUE cannot run inside the same transaction as a SELECT
-- against that new value in some Postgres versions, so the promotion below
-- runs at COMMIT time via a one-shot DO block in a separate statement
-- boundary. Wrap in advisory lock to avoid two parallel migration runners
-- both promoting different rows.
COMMIT;

DO $$
DECLARE
    super_count INT;
    promote_id UUID;
BEGIN
    PERFORM pg_advisory_xact_lock(20260428000001);

    SELECT count(*) INTO super_count FROM users WHERE role = 'super_admin';
    IF super_count > 0 THEN
        RAISE NOTICE 'super_admin already exists; skipping auto-promotion';
        RETURN;
    END IF;

    SELECT id INTO promote_id
    FROM users
    WHERE role = 'admin'
    ORDER BY created_at ASC
    LIMIT 1;

    IF promote_id IS NULL THEN
        RAISE NOTICE 'no existing admin to promote; first /auth/admin invite acceptance must seed super_admin manually';
        RETURN;
    END IF;

    UPDATE users SET role = 'super_admin' WHERE id = promote_id;
    RAISE NOTICE 'promoted user % to super_admin (oldest admin)', promote_id;
END $$;

ALTER TABLE admin_invites
    DROP CONSTRAINT IF EXISTS admin_invites_role_check;

ALTER TABLE admin_invites
    ADD CONSTRAINT admin_invites_role_check
    CHECK (role IN ('admin', 'moderator', 'super_admin'));

COMMENT ON COLUMN users.role IS 'User role: user (default), moderator, admin, or super_admin (highest tier)';
