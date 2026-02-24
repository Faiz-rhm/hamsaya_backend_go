-- Add unique constraint so UpsertHours ON CONFLICT (business_profile_id, day) works
ALTER TABLE business_hours
ADD CONSTRAINT business_hours_business_profile_id_day_key UNIQUE (business_profile_id, day);
