-- Revert the backup-code column width. NOTE: if any hashed (64-char) codes
-- exist, this will fail/truncate — clear mfa_backup_codes first if rolling back.
ALTER TABLE mfa_backup_codes ALTER COLUMN code TYPE VARCHAR(20);
