-- Per-user rate-limit overrides. Default rate limits are global
-- (RATE_LIMIT_REQUESTS_PER_HOUR), but some users (verified businesses,
-- system integrations, partner accounts) need custom caps. A row here
-- replaces the global limit for that user; rows can also be used to
-- *tighten* limits on suspicious users without a full ban.

CREATE TABLE IF NOT EXISTS user_rate_limit_overrides (
    user_id              UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    requests_per_hour    INTEGER      NOT NULL,
    reason               TEXT,
    created_by           UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT user_rate_limit_overrides_positive_chk CHECK (requests_per_hour >= 0)
);

COMMENT ON TABLE  user_rate_limit_overrides                    IS 'Per-user replacement of the global RATE_LIMIT_REQUESTS_PER_HOUR cap.';
COMMENT ON COLUMN user_rate_limit_overrides.requests_per_hour  IS '0 = effectively block; any positive integer = new hourly cap.';
