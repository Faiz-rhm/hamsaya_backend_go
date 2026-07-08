-- Daily-bucketed business profile views so owner insights can chart
-- views-over-time. total_views on business_profiles stays the all-time
-- counter; this table only powers the time-series.
CREATE TABLE IF NOT EXISTS business_profile_daily_views (
    business_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    views INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (business_id, day)
);
