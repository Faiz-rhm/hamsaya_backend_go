-- Persistent application log buffer surfaced through /admin/logs.
-- Captures warn-and-above zap entries asynchronously so admins can triage
-- without shelling into the container. Loki / Grafana remain the
-- long-term home; this table holds a recent slice (ringed by retention job
-- when traffic warrants it).

CREATE TABLE IF NOT EXISTS app_logs (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    level       VARCHAR(10)  NOT NULL,
    message     TEXT         NOT NULL,
    source      VARCHAR(120),
    request_id  VARCHAR(64),
    error       TEXT,
    fields      JSONB,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT app_logs_level_chk CHECK (level IN ('debug','info','warn','error','dpanic','panic','fatal'))
);

CREATE INDEX IF NOT EXISTS idx_app_logs_created_desc ON app_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_logs_level_created ON app_logs(level, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_logs_request_id ON app_logs(request_id) WHERE request_id IS NOT NULL;

COMMENT ON TABLE  app_logs            IS 'Recent warn+ application log entries surfaced to admins via /admin/logs.';
COMMENT ON COLUMN app_logs.fields     IS 'Structured zap fields encoded as JSON for ad-hoc inspection.';
COMMENT ON COLUMN app_logs.request_id IS 'X-Request-Id from the originating HTTP request when available.';
