-- Add contact fields (phone + WhatsApp) to ads so advertisers can be reached
-- directly from a placement. Both nullable; one or the other (or neither) is
-- acceptable. UI validates format; DB just stores the string.

ALTER TABLE ads ADD COLUMN IF NOT EXISTS phone_number    VARCHAR(40);
ALTER TABLE ads ADD COLUMN IF NOT EXISTS whatsapp_number VARCHAR(40);

COMMENT ON COLUMN ads.phone_number    IS 'Optional E.164-ish phone number surfaced on the placement.';
COMMENT ON COLUMN ads.whatsapp_number IS 'Optional WhatsApp contact (E.164-ish digits).';
