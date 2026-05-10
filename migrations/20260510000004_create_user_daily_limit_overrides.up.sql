-- Per-user daily post limit overrides. Default caps live in
-- daily_post_limits (per-post-type, with business multiplier). When a
-- specific user (or business owner) needs more headroom — verified
-- businesses, partner accounts, paid tiers — admins can grant a custom
-- daily cap for a given post_type without changing the global default.
--
-- Override semantics:
--   * unlimited=true → bypass the cap entirely for this user+post_type
--   * unlimited=false + override_limit set → replaces the effective
--     limit (still respects per-day reset). Applies whether the user
--     posts personally or as a business; admins who need finer control
--     can simply tighten/expand and the change applies to both modes.
--
-- Composite PK on (user_id, post_type) so an admin granting "FEED+SELL"
-- adds two rows — one tunable per axis.

CREATE TABLE IF NOT EXISTS user_daily_post_limit_overrides (
    user_id          UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_type        TEXT         NOT NULL,
    override_limit   INTEGER,
    unlimited        BOOLEAN      NOT NULL DEFAULT FALSE,
    reason           TEXT,
    created_by       UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, post_type),
    CONSTRAINT user_daily_lim_override_value_chk CHECK (
        unlimited = TRUE OR (override_limit IS NOT NULL AND override_limit >= 0)
    )
);

CREATE INDEX IF NOT EXISTS idx_user_daily_lim_override_user ON user_daily_post_limit_overrides(user_id);

COMMENT ON TABLE  user_daily_post_limit_overrides                IS 'Per-user replacement of the per-post-type daily cap. Beats daily_post_limits for that user+post_type.';
COMMENT ON COLUMN user_daily_post_limit_overrides.unlimited      IS 'When true, this user bypasses the cap for this post_type entirely.';
COMMENT ON COLUMN user_daily_post_limit_overrides.override_limit IS 'Replacement integer cap; used only when unlimited=false. 0 = effectively block.';
