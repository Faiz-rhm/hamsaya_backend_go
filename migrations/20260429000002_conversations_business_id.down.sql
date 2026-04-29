DROP INDEX IF EXISTS idx_conversations_unique_triple;
DROP INDEX IF EXISTS idx_conversations_business_id;
ALTER TABLE conversations
    ADD CONSTRAINT conversations_participant1_id_participant2_id_key
    UNIQUE (participant1_id, participant2_id);
ALTER TABLE conversations DROP COLUMN IF EXISTS business_id;
