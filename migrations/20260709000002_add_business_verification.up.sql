-- Business verification: owners submit documents (license etc.), admins
-- review, approved businesses get a verified tick.
ALTER TABLE business_profiles
    ADD COLUMN IF NOT EXISTS is_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMP WITH TIME ZONE;

CREATE TABLE IF NOT EXISTS business_verification_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    license_no VARCHAR(100),
    note TEXT,
    -- Array of Photo objects (same shape as post/business attachments).
    documents JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    rejection_reason TEXT,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- One open request per business at a time.
CREATE UNIQUE INDEX IF NOT EXISTS idx_business_verification_pending
    ON business_verification_requests (business_id) WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_business_verification_status
    ON business_verification_requests (status, created_at DESC);
