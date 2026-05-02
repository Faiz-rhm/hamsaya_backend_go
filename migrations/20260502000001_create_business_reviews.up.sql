-- Business reviews: per-user rating + optional comment for a business profile.
-- Aggregates (avg_rating, review_count) are maintained on business_profiles by
-- a trigger so the read path stays cheap (no JOIN/aggregate per profile fetch).

CREATE TABLE IF NOT EXISTS business_reviews (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_profile_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating              SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment             TEXT,
    is_hidden           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- One review per user per business. Edits update the existing row.
    CONSTRAINT business_reviews_unique_user UNIQUE (business_profile_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_business_reviews_business
    ON business_reviews (business_profile_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_reviews_user
    ON business_reviews (user_id, created_at DESC);

-- Aggregates on business_profiles. Defaults so existing rows behave as zero.
ALTER TABLE business_profiles
    ADD COLUMN IF NOT EXISTS avg_rating  NUMERIC(3,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS review_count INTEGER     NOT NULL DEFAULT 0;

-- Trigger function: recompute avg_rating + review_count for the affected
-- business profile. Only counts rows where is_hidden = false (admin moderation).
CREATE OR REPLACE FUNCTION recompute_business_review_aggregates()
RETURNS TRIGGER AS $$
DECLARE
    target_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_id := OLD.business_profile_id;
    ELSE
        target_id := NEW.business_profile_id;
    END IF;

    UPDATE business_profiles bp
    SET review_count = COALESCE(stats.cnt, 0),
        avg_rating   = COALESCE(stats.avg_rating, 0)
    FROM (
        SELECT
            COUNT(*)                                AS cnt,
            ROUND(AVG(rating)::numeric, 2)          AS avg_rating
        FROM business_reviews
        WHERE business_profile_id = target_id
          AND is_hidden = FALSE
    ) AS stats
    WHERE bp.id = target_id;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_business_reviews_aggregates ON business_reviews;
CREATE TRIGGER trg_business_reviews_aggregates
AFTER INSERT OR UPDATE OR DELETE ON business_reviews
FOR EACH ROW EXECUTE FUNCTION recompute_business_review_aggregates();
