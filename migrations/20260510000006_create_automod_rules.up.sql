-- Automod keyword/pattern rules. Each row is one rule that the
-- post-create path matches (case-insensitive) against title +
-- description. The action decides what happens when a rule fires:
--   * 'block'  → reject the post with 422 + show the user a toast
--   * 'flag'   → allow the post but enqueue a moderator report
--                automatically so it surfaces in /moderation
--   * 'shadow' → allow but mark post as shadow_status='hidden';
--                feed query already filters for shadowban — re-using
--                the same column would conflict with user-level
--                shadowban, so post-level shadowing lives in posts
--                table (added in a follow-up if needed). For v1,
--                shadow falls back to 'flag' and a comment in code.

CREATE TABLE IF NOT EXISTS automod_rules (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    pattern       TEXT         NOT NULL,
    is_regex      BOOLEAN      NOT NULL DEFAULT FALSE,
    action        VARCHAR(10)  NOT NULL DEFAULT 'flag',
    severity      VARCHAR(10)  NOT NULL DEFAULT 'medium',
    enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    description   TEXT,
    created_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_hit_at   TIMESTAMPTZ,
    hit_count     BIGINT       NOT NULL DEFAULT 0,
    CONSTRAINT automod_rules_action_chk   CHECK (action   IN ('block','flag','shadow')),
    CONSTRAINT automod_rules_severity_chk CHECK (severity IN ('low','medium','high','critical')),
    CONSTRAINT automod_rules_pattern_nonempty CHECK (length(pattern) > 0)
);

CREATE INDEX IF NOT EXISTS idx_automod_rules_enabled
    ON automod_rules(enabled, severity)
    WHERE enabled = TRUE;

COMMENT ON TABLE  automod_rules           IS 'Keyword/regex rules scanned at post-create time. Match → block / flag / shadow.';
COMMENT ON COLUMN automod_rules.pattern   IS 'Substring (case-insensitive) or regex (when is_regex=TRUE).';
COMMENT ON COLUMN automod_rules.action    IS 'block | flag | shadow. block rejects the post; flag auto-creates a report; shadow currently aliases to flag.';
COMMENT ON COLUMN automod_rules.hit_count IS 'Lifetime fires; updated by the post-create path; useful for tuning false-positive rules.';
