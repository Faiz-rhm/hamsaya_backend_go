-- Track every database backup attempt so admins can see history, sizes, and
-- failures from the /backups page. The actual dump artifacts live on a
-- volume mount and in MinIO; this table holds only metadata.

CREATE TABLE IF NOT EXISTS db_backups (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    status        VARCHAR(20)  NOT NULL DEFAULT 'running',
    tier          VARCHAR(10)  NOT NULL,
    size_bytes    BIGINT,
    object_key    VARCHAR(255),
    local_path    VARCHAR(255),
    triggered_by  VARCHAR(20)  NOT NULL DEFAULT 'cron',
    admin_id      UUID         REFERENCES users(id) ON DELETE SET NULL,
    error         TEXT,
    CONSTRAINT db_backups_status_chk CHECK (status IN ('running','success','failed')),
    CONSTRAINT db_backups_tier_chk   CHECK (tier IN ('daily','weekly','monthly','adhoc')),
    CONSTRAINT db_backups_trig_chk   CHECK (triggered_by IN ('cron','admin'))
);

CREATE INDEX IF NOT EXISTS idx_db_backups_started_desc ON db_backups(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_db_backups_tier_status ON db_backups(tier, status, started_at DESC);

COMMENT ON TABLE  db_backups            IS 'Metadata for every pg_dump run. Artifacts live in object storage + local volume.';
COMMENT ON COLUMN db_backups.tier       IS 'GFS rotation tier: daily / weekly (Sun) / monthly (1st) / adhoc (admin-triggered).';
COMMENT ON COLUMN db_backups.object_key IS 'MinIO key under BACKUP_BUCKET; null if upload failed or not yet attempted.';
COMMENT ON COLUMN db_backups.local_path IS 'Path on the BACKUP_LOCAL_DIR volume; null if local copy not written.';
