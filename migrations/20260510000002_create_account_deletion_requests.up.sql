-- GDPR / data subject deletion requests. When a user hits "delete my
-- account" we don't immediately wipe; we queue here and an admin
-- reviews. This gives us a window to honor "I changed my mind", to flag
-- accounts under active investigation, and to satisfy compliance
-- requirements that data is removed only after a clearly auditable
-- review step.

CREATE TABLE IF NOT EXISTS account_deletion_requests (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    reason          TEXT,
    user_ip         VARCHAR(64),
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    reviewed_at     TIMESTAMPTZ,
    reviewed_by     UUID         REFERENCES users(id) ON DELETE SET NULL,
    review_notes    TEXT,
    completed_at    TIMESTAMPTZ,
    CONSTRAINT account_deletion_requests_status_chk CHECK (
      status IN ('pending','approved','rejected','completed')
    )
);

CREATE INDEX IF NOT EXISTS idx_acct_del_user ON account_deletion_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_acct_del_status_requested ON account_deletion_requests(status, requested_at DESC);

COMMENT ON TABLE  account_deletion_requests          IS 'GDPR-style deletion request queue; admin reviews before destructive deletion.';
COMMENT ON COLUMN account_deletion_requests.status   IS 'pending → approved/rejected → completed (when actual user delete runs).';
