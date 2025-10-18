-- ============================================================================
-- POSTS SYSTEM (FEED, EVENT, SELL, PULL types)
-- ============================================================================

-- Sell categories (for marketplace posts)
CREATE TABLE sell_categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    icon JSONB NOT NULL, -- {name, library}
    color VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'ACTIVE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Note: business_profiles table will be created in migration 005
-- We need to create it as a placeholder here for the foreign key
CREATE TABLE business_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Posts table (supports 4 types: FEED, EVENT, SELL, PULL)
CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    business_id UUID REFERENCES business_profiles(id) ON DELETE SET NULL,
    original_post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    category_id UUID REFERENCES sell_categories(id) ON DELETE SET NULL,

    -- Content fields
    title VARCHAR(255),
    description TEXT,
    type VARCHAR(20) NOT NULL CHECK (type IN ('FEED', 'EVENT', 'SELL', 'PULL')),
    status BOOLEAN DEFAULT true,
    visibility VARCHAR(20) DEFAULT 'PUBLIC' CHECK (visibility IN ('PUBLIC', 'FRIENDS', 'PRIVATE')),

    -- Sell-specific fields
    currency VARCHAR(3),
    price DECIMAL(10,2),
    discount DECIMAL(5,2),
    free BOOLEAN DEFAULT false,
    sold BOOLEAN DEFAULT false,
    is_promoted BOOLEAN DEFAULT false,
    country_code VARCHAR(10),
    contact_no VARCHAR(20),
    is_location BOOLEAN DEFAULT false,

    -- Event-specific fields
    start_date DATE,
    start_time TIME,
    end_date DATE,
    end_time TIME,
    event_state VARCHAR(20),
    interested_count INTEGER DEFAULT 0,
    going_count INTEGER DEFAULT 0,
    expired_at TIMESTAMP WITH TIME ZONE,

    -- Location fields
    address_location GEOGRAPHY(POINT, 4326),
    user_location GEOGRAPHY(POINT, 4326),
    country VARCHAR(100),
    province VARCHAR(100),
    district VARCHAR(100),
    neighborhood VARCHAR(100),

    -- Engagement counters
    total_comments INTEGER DEFAULT 0,
    total_likes INTEGER DEFAULT 0,
    total_shares INTEGER DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT posts_user_or_business CHECK (user_id IS NOT NULL OR business_id IS NOT NULL)
);

-- Post attachments (images/videos)
CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    photo JSONB NOT NULL, -- {url, name, mime_type, size, width, height}
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Post likes
CREATE TABLE post_likes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

-- Post bookmarks
CREATE TABLE post_bookmarks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

-- Post shares
CREATE TABLE post_shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    shared_post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    share_text TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Event interests
CREATE TABLE event_interests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_state VARCHAR(20) NOT NULL CHECK (event_state IN ('interested', 'going', 'not_interested')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(post_id, user_id)
);

-- Post reports
CREATE TABLE post_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL,
    additional_comments TEXT,
    report_status VARCHAR(20) DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Comments for maintainability
COMMENT ON TABLE posts IS 'Main posts table supporting 4 types: FEED, EVENT, SELL, PULL';
COMMENT ON TABLE attachments IS 'Post media attachments (images, videos)';
COMMENT ON TABLE event_interests IS 'User interest in events (interested/going/not_interested)';
