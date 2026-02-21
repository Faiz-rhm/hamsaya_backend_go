-- Remove business_id from notification data (cannot reliably revert without storing old data).
-- No-op: backfill is one-way for display logic only.
SELECT 1;
