-- Attach optional product context (a sale post) to a chat message so the
-- recipient sees what the sender is asking about. Survives across sessions.
ALTER TABLE messages
    ADD COLUMN product_id UUID NULL REFERENCES posts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_messages_product_id
    ON messages(product_id)
    WHERE product_id IS NOT NULL;
