-- MFA backup codes are now stored hashed (SHA-256 hex = 64 chars) instead of
-- the old 8-char plaintext. The column was VARCHAR(20), which overflowed on
-- insert ("value too long for type character varying(20)"). Widen to hold the
-- hash. Existing rows (old plaintext codes) are shorter and unaffected; they
-- stop matching after the app starts hashing on verify — users regenerate.
ALTER TABLE mfa_backup_codes ALTER COLUMN code TYPE VARCHAR(64);
