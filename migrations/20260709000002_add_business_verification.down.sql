DROP TABLE IF EXISTS business_verification_requests;
ALTER TABLE business_profiles
    DROP COLUMN IF EXISTS is_verified,
    DROP COLUMN IF EXISTS verified_at;
