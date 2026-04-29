-- Allow conversations to be scoped to a business so users can chat with
-- businesses (technically with the business owner) and have a separate
-- thread per business. business_id NULL means a personal user-to-user chat.
ALTER TABLE conversations
    ADD COLUMN business_id UUID NULL REFERENCES business_profiles(id) ON DELETE CASCADE;

-- Unique constraint must include business_id so each (user, user, business)
-- triple has exactly one conversation; (user, user, NULL) is the personal chat.
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_participant1_id_participant2_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_unique_triple
    ON conversations(
        participant1_id,
        participant2_id,
        COALESCE(business_id, '00000000-0000-0000-0000-000000000000'::uuid)
    );

CREATE INDEX IF NOT EXISTS idx_conversations_business_id
    ON conversations(business_id)
    WHERE business_id IS NOT NULL;
