-- High-leverage feature batch:
--   • ads.weight + daily_impression_cap + target_provinces/languages
--   • user_blocks table for moderation (Apple App Store compliance)

-- ─── Ads: targeting + frequency cap ─────────────────────────────────────────
ALTER TABLE ads ADD COLUMN IF NOT EXISTS weight                INT DEFAULT 1 NOT NULL;
ALTER TABLE ads ADD COLUMN IF NOT EXISTS daily_impression_cap  INT;
ALTER TABLE ads ADD COLUMN IF NOT EXISTS target_provinces      TEXT[] DEFAULT '{}'::TEXT[];
ALTER TABLE ads ADD COLUMN IF NOT EXISTS target_languages      TEXT[] DEFAULT '{}'::TEXT[];

COMMENT ON COLUMN ads.weight                IS 'Selection weight; ORDER BY weight*RANDOM() DESC. Higher = more likely to surface.';
COMMENT ON COLUMN ads.daily_impression_cap  IS 'NULL = no cap; otherwise stop serving once today''s impressions reach this number.';
COMMENT ON COLUMN ads.target_provinces      IS 'Empty = no province targeting. When set, ad only served to users whose profile province is in this set.';
COMMENT ON COLUMN ads.target_languages      IS 'Empty = no language targeting. When set, ad only served when user locale matches.';

CREATE INDEX IF NOT EXISTS idx_ads_targeting_provinces ON ads USING GIN(target_provinces);
CREATE INDEX IF NOT EXISTS idx_ads_targeting_languages ON ads USING GIN(target_languages);

-- ─── User blocks: reason column ─────────────────────────────────────────────
-- The user_blocks table already exists (initial schema). We only add an
-- optional `reason` so the report→block flow can preserve admin context.
ALTER TABLE user_blocks ADD COLUMN IF NOT EXISTS reason VARCHAR(120);
COMMENT ON COLUMN user_blocks.reason IS 'Optional. Set when block was triggered from the report→block pipeline; otherwise NULL.';
