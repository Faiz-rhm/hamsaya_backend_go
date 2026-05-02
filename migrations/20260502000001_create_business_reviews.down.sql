DROP TRIGGER IF EXISTS trg_business_reviews_aggregates ON business_reviews;
DROP FUNCTION IF EXISTS recompute_business_review_aggregates();

ALTER TABLE business_profiles
    DROP COLUMN IF EXISTS avg_rating,
    DROP COLUMN IF EXISTS review_count;

DROP TABLE IF EXISTS business_reviews;
