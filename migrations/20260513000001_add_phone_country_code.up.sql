-- Add ISO-3166 alpha-2 phone country code to users so mobile can
-- restore the user's dial-region in the phone picker without re-parsing
-- the E.164 number every load.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone_country_code VARCHAR(2);

COMMENT ON COLUMN users.phone_country_code IS
    'ISO-3166 alpha-2 region for the phone number (e.g. AF, US). Populated by the mobile picker.';
