-- Manual review queue for every uploaded media attachment. Posts are
-- still visible immediately (no review-before-publish gate — that would
-- crush the user experience and leave a backlog the team can never
-- clear). Instead each attachment lands here with status='pending' and
-- admins can audit, approve (no-op), or reject (soft-deletes the
-- attachment). For automated CSAM/NSFW classification, hook a worker
-- that reads pending rows and writes back labels — not in this MVP.

CREATE TABLE IF NOT EXISTS media_moderation_queue (
    attachment_id  UUID         PRIMARY KEY REFERENCES attachments(id) ON DELETE CASCADE,
    post_id        UUID         REFERENCES posts(id) ON DELETE CASCADE,
    enqueued_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    status         VARCHAR(15)  NOT NULL DEFAULT 'pending',
    reviewed_at    TIMESTAMPTZ,
    reviewed_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    review_notes   TEXT,
    auto_labels    JSONB,
    CONSTRAINT media_moderation_status_chk CHECK (status IN ('pending','approved','rejected'))
);

-- Pending-first ordering for the admin queue page.
CREATE INDEX IF NOT EXISTS idx_media_moderation_status_enqueued
    ON media_moderation_queue(status, enqueued_at DESC);

COMMENT ON TABLE  media_moderation_queue              IS 'One row per uploaded attachment. Pending rows surface in /admin/media-moderation.';
COMMENT ON COLUMN media_moderation_queue.auto_labels  IS 'Reserved for future automatic classifier output (NSFW score, OCR text, etc).';
