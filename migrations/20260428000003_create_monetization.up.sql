-- Monetization tables: ads, credit balances/transactions, boosts.
-- Designed to back the admin panel /ads, /credits, /boosts pages and to
-- provide a starting surface for the future user-facing monetization flows.
-- Schemas are intentionally minimal — fields can be added without rewriting
-- the admin UI as long as existing columns are preserved.

-- ─── Ads ─────────────────────────────────────────────────────────────────────
-- Paid placements submitted by advertisers (users) and reviewed by admins
-- before going live. Status transitions: PENDING → APPROVED → ACTIVE → EXPIRED
-- or PENDING → REJECTED. Active window is bounded by start_at / end_at.

CREATE TABLE IF NOT EXISTS ads (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    advertiser_id   UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           VARCHAR(120) NOT NULL,
    body            TEXT,
    image_url       TEXT,
    target_url      TEXT         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    start_at        TIMESTAMPTZ,
    end_at          TIMESTAMPTZ,
    impressions     INTEGER      NOT NULL DEFAULT 0,
    clicks          INTEGER      NOT NULL DEFAULT 0,
    reviewed_by     UUID         REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at     TIMESTAMPTZ,
    review_note     TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT ads_status_chk CHECK (status IN ('PENDING','APPROVED','REJECTED','ACTIVE','EXPIRED'))
);

CREATE INDEX IF NOT EXISTS idx_ads_status_created ON ads(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ads_advertiser     ON ads(advertiser_id, created_at DESC);

COMMENT ON TABLE  ads               IS 'Advertiser-submitted paid placements; reviewed by admin before going live.';
COMMENT ON COLUMN ads.status        IS 'PENDING|APPROVED|REJECTED|ACTIVE|EXPIRED — admin transitions this.';
COMMENT ON COLUMN ads.target_url    IS 'Click-through URL. Validated as http(s) at API layer.';
COMMENT ON COLUMN ads.review_note   IS 'Reason for rejection or approval, surfaced to advertiser.';

-- ─── Credit balances ─────────────────────────────────────────────────────────
-- One row per user with non-zero credits. Created on first credit grant.
-- Balance is denormalised; transactions table is the source of truth and
-- balance is rebuildable via SUM(amount).

CREATE TABLE IF NOT EXISTS credit_balances (
    user_id     UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance     INTEGER      NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE  credit_balances        IS 'Denormalised per-user credit balance. Transactions are authoritative.';
COMMENT ON COLUMN credit_balances.balance IS 'Cached SUM of credit_transactions.amount for the user. Never negative.';

-- ─── Credit transactions ────────────────────────────────────────────────────
-- Append-only ledger. Positive amount = credit (top-up, refund, comp); negative
-- = debit (boost spend, ad spend, fraud). admin_id is set when the transaction
-- was an admin-initiated adjustment.

CREATE TABLE IF NOT EXISTS credit_transactions (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      INTEGER      NOT NULL,
    type        VARCHAR(20)  NOT NULL,
    reason      VARCHAR(120),
    note        TEXT,
    admin_id    UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT credit_tx_type_chk CHECK (type IN (
        'TOPUP','PURCHASE','BOOST_SPEND','AD_SPEND','REFUND','ADJUST_ADD','ADJUST_REMOVE','PROMO'
    ))
);

CREATE INDEX IF NOT EXISTS idx_credit_tx_user_created ON credit_transactions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credit_tx_admin        ON credit_transactions(admin_id, created_at DESC);

COMMENT ON TABLE  credit_transactions       IS 'Append-only credit ledger. Source of truth for credit_balances.';
COMMENT ON COLUMN credit_transactions.type  IS 'TOPUP|PURCHASE|BOOST_SPEND|AD_SPEND|REFUND|ADJUST_ADD|ADJUST_REMOVE|PROMO';

-- ─── Boosts ──────────────────────────────────────────────────────────────────
-- A boost promotes a single post for a fixed window. Created by users (debits
-- credits) and observed/cancellable by admins.

CREATE TABLE IF NOT EXISTS boosts (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id         UUID         NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id         UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE',
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ  NOT NULL,
    credits_spent   INTEGER      NOT NULL DEFAULT 0 CHECK (credits_spent >= 0),
    cancelled_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    cancelled_at    TIMESTAMPTZ,
    cancel_reason   TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT boosts_status_chk CHECK (status IN ('ACTIVE','EXPIRED','CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_boosts_status_started ON boosts(status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_boosts_post           ON boosts(post_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_boosts_user           ON boosts(user_id, started_at DESC);

COMMENT ON TABLE  boosts          IS 'Per-post boost windows. Admins can cancel ACTIVE rows for policy reasons.';
COMMENT ON COLUMN boosts.status   IS 'ACTIVE|EXPIRED|CANCELLED.';

-- ─── Sample seed data ────────────────────────────────────────────────────────
-- Pull a stable set of users by email_verified=TRUE so the admin pages have
-- something visible on a fresh DB. The DO block tolerates an empty users
-- table (fresh install before seeding) by skipping the inserts.

DO $$
DECLARE
    advertiser_a UUID;
    advertiser_b UUID;
    boost_post_a UUID;
    boost_post_b UUID;
    boost_user_a UUID;
    boost_user_b UUID;
BEGIN
    SELECT id INTO advertiser_a FROM users WHERE email_verified ORDER BY created_at LIMIT 1;
    SELECT id INTO advertiser_b FROM users WHERE email_verified ORDER BY created_at OFFSET 1 LIMIT 1;
    IF advertiser_a IS NULL THEN
        RETURN; -- empty users table; skip seeding
    END IF;
    IF advertiser_b IS NULL THEN
        advertiser_b := advertiser_a;
    END IF;

    -- 4 ads spanning every status the UI handles.
    INSERT INTO ads (advertiser_id, title, body, image_url, target_url, status, start_at, end_at)
    VALUES
        (advertiser_a, 'Spring Sale 2026', 'Electronics 30% off through May.',
         NULL, 'https://example.com/spring', 'PENDING', NULL, NULL),
        (advertiser_b, 'Hamsaya Premium Launch', 'Try Premium free for 14 days.',
         NULL, 'https://hamsaya.af/premium', 'APPROVED',
         NOW(), NOW() + INTERVAL '7 days'),
        (advertiser_a, 'Local Bakery Grand Opening', 'Free coffee with first order.',
         NULL, 'https://example.com/bakery', 'ACTIVE',
         NOW() - INTERVAL '1 day', NOW() + INTERVAL '5 days'),
        (advertiser_b, 'Job Board — Now Live', 'Find local jobs in Kabul.',
         NULL, 'https://example.com/jobs', 'REJECTED', NULL, NULL)
    ON CONFLICT DO NOTHING;

    -- Credit balances + transactions for the first two users.
    INSERT INTO credit_balances (user_id, balance) VALUES
        (advertiser_a, 1500),
        (advertiser_b, 250)
    ON CONFLICT (user_id) DO NOTHING;

    INSERT INTO credit_transactions (user_id, amount, type, reason, note) VALUES
        (advertiser_a, 1000, 'TOPUP',       'Stripe purchase #1001', 'Card ending 4242'),
        (advertiser_a, 500,  'PROMO',       'Welcome bonus',          NULL),
        (advertiser_a, -50,  'BOOST_SPEND', 'Boost on post sample',   NULL),
        (advertiser_b, 200,  'TOPUP',       'Stripe purchase #1002',  NULL),
        (advertiser_b, 100,  'ADJUST_ADD',  'Comp for shipping delay',NULL),
        (advertiser_b, -50,  'AD_SPEND',    'Ad placement A',         NULL);

    -- Try to seed boosts using two real posts authored by these users.
    SELECT p.id, p.user_id INTO boost_post_a, boost_user_a
        FROM posts p WHERE p.user_id = advertiser_a ORDER BY p.created_at DESC LIMIT 1;
    SELECT p.id, p.user_id INTO boost_post_b, boost_user_b
        FROM posts p WHERE p.user_id = advertiser_b ORDER BY p.created_at DESC LIMIT 1;

    IF boost_post_a IS NOT NULL THEN
        INSERT INTO boosts (post_id, user_id, status, started_at, expires_at, credits_spent)
        VALUES (boost_post_a, boost_user_a, 'ACTIVE',
                NOW() - INTERVAL '2 hours', NOW() + INTERVAL '22 hours', 50)
        ON CONFLICT DO NOTHING;
    END IF;
    IF boost_post_b IS NOT NULL THEN
        INSERT INTO boosts (post_id, user_id, status, started_at, expires_at, credits_spent, cancelled_at, cancel_reason)
        VALUES (boost_post_b, boost_user_b, 'CANCELLED',
                NOW() - INTERVAL '3 days', NOW() - INTERVAL '2 days', 50,
                NOW() - INTERVAL '2 days', 'Sample cancellation')
        ON CONFLICT DO NOTHING;
    END IF;
END$$;
