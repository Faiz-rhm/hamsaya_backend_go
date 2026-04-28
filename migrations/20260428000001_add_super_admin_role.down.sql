-- Postgres does not support DROP VALUE for enums; reverting requires
-- recreating the user_role type. Refuses to run while any user still holds
-- the super_admin role to avoid silent data loss.

DO $$
DECLARE
    n INT;
BEGIN
    SELECT count(*) INTO n FROM users WHERE role = 'super_admin';
    IF n > 0 THEN
        RAISE EXCEPTION 'cannot remove super_admin enum: % users still have that role; demote them first', n;
    END IF;
END $$;

ALTER TABLE admin_invites
    DROP CONSTRAINT IF EXISTS admin_invites_role_check;

ALTER TABLE admin_invites
    ADD CONSTRAINT admin_invites_role_check
    CHECK (role IN ('admin', 'moderator'));

-- Recreate user_role enum without super_admin.
ALTER TYPE user_role RENAME TO user_role_old;
CREATE TYPE user_role AS ENUM ('user', 'admin', 'moderator');
ALTER TABLE users
    ALTER COLUMN role TYPE user_role USING role::text::user_role;
DROP TYPE user_role_old;

COMMENT ON COLUMN users.role IS 'User role: user (default), admin, or moderator';
