DROP INDEX IF EXISTS idx_messages_product_id;
ALTER TABLE messages DROP COLUMN IF EXISTS product_id;
