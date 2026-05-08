ALTER TABLE user_blocks DROP COLUMN IF EXISTS reason;

DROP INDEX IF EXISTS idx_ads_targeting_languages;
DROP INDEX IF EXISTS idx_ads_targeting_provinces;
ALTER TABLE ads DROP COLUMN IF EXISTS target_languages;
ALTER TABLE ads DROP COLUMN IF EXISTS target_provinces;
ALTER TABLE ads DROP COLUMN IF EXISTS daily_impression_cap;
ALTER TABLE ads DROP COLUMN IF EXISTS weight;
