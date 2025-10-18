# Production-Ready Go Backend for Hamsaya Mobile App

## Executive Summary

This document provides a comprehensive implementation plan for a production-ready Go backend that **exactly mirrors** the existing Flutter mobile application's functionality, data models, and API patterns. Based on extensive analysis of the Flutter codebase, this plan covers all features, flows, and technical requirements.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Technology Stack](#technology-stack)
3. [Database Schema (Complete)](#database-schema-complete)
4. [API Endpoints (Complete Mapping)](#api-endpoints-complete-mapping)
5. [Authentication & Security](#authentication--security)
6. [Core Services Implementation](#core-services-implementation)
7. [Image Storage & Processing](#image-storage--processing)
8. [Real-time Features](#real-time-features)
9. [Location & Geocoding](#location--geocoding)
10. [Testing Strategy](#testing-strategy)
11. [Deployment & DevOps](#deployment--devops)
12. [Phase-by-Phase Implementation](#phase-by-phase-implementation)

---

## Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Flutter Mobile App                        │
│                    (Existing - No Changes)                       │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS/WSS
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         API Gateway                              │
│                    (Nginx + Rate Limiting)                       │
└────────────────────────────┬────────────────────────────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
          ▼                  ▼                  ▼
┌──────────────────┐ ┌──────────────┐ ┌────────────────┐
│   Go REST API    │ │  WebSocket   │ │  File Storage  │
│   (Gin/Echo)     │ │   Service    │ │  (S3/MinIO)    │
└────────┬─────────┘ └──────┬───────┘ └────────┬───────┘
         │                  │                  │
         └──────────────────┼──────────────────┘
                            │
          ┌─────────────────┴──────────────────┐
          │                                    │
          ▼                                    ▼
┌──────────────────┐                 ┌─────────────────┐
│   PostgreSQL     │                 │     Redis       │
│  (Primary DB)    │                 │  (Cache/Queue)  │
└──────────────────┘                 └─────────────────┘
```

### Design Principles

1. **Drop-in Replacement**: API must be 100% compatible with existing Supabase endpoints
2. **Feature Parity**: Every feature in the Flutter app must be supported
3. **Performance**: Sub-100ms response times for 95% of requests
4. **Scalability**: Horizontal scaling support from day one
5. **Security**: OAuth2, JWT, MFA, rate limiting, input validation
6. **Observability**: Structured logging, metrics, distributed tracing

---

## Technology Stack

### Core Technologies

| Component | Technology | Version | Justification |
|-----------|-----------|---------|---------------|
| **Language** | Go | 1.21+ | Performance, concurrency, simple deployment |
| **Web Framework** | Gin | v1.9+ | Fast, middleware support, excellent routing |
| **Database Driver** | pgx | v5+ | Best PostgreSQL driver, prepared statements, connection pooling |
| **Authentication** | golang-jwt | v5+ | Standard JWT implementation |
| **OAuth2** | golang.org/x/oauth2 | latest | Google, Apple, Facebook support |
| **Image Processing** | imaging | v1.6+ | Resize, compress, format conversion |
| **Object Storage** | MinIO Go Client | v7+ | S3-compatible storage |
| **WebSocket** | gorilla/websocket | v1.5+ | Mature, production-ready |
| **Validation** | go-playground/validator | v10+ | Struct validation |
| **Configuration** | viper | v1.16+ | Environment management |
| **Logging** | zap | v1.26+ | Structured, high-performance logging |
| **Testing** | testify | v1.8+ | Assertions and mocking |
| **Migrations** | golang-migrate | v4+ | Database versioning |
| **Cache** | go-redis | v9+ | Redis client |
| **TOTP/MFA** | pquerna/otp | v1.4+ | Time-based OTP |

### Development Tools

- **Swagger/OpenAPI**: API documentation
- **golangci-lint**: Code quality
- **Docker**: Containerization
- **Docker Compose**: Local development
- **GitHub Actions**: CI/CD pipeline
- **Prometheus**: Metrics collection
- **Grafana**: Metrics visualization
- **Sentry**: Error tracking

---

## Database Schema (Complete)

Based on comprehensive analysis of all Flutter models and repository operations, here is the complete database schema:

### Core Tables

```sql
-- ============================================================================
-- USERS & AUTHENTICATION
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "postgis";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone VARCHAR(20),
    password_hash VARCHAR(255),
    email_verified BOOLEAN DEFAULT false,
    phone_verified BOOLEAN DEFAULT false,
    mfa_enabled BOOLEAN DEFAULT false,
    oauth_provider VARCHAR(50), -- 'google', 'apple', 'facebook', null
    oauth_provider_id VARCHAR(255),
    last_login_at TIMESTAMP WITH TIME ZONE,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE profiles (
    id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    avatar JSONB, -- {url, name, size, width, height, mime_type}
    cover JSONB,
    about TEXT,
    gender VARCHAR(10),
    dob DATE,
    website VARCHAR(255),
    location GEOGRAPHY(POINT, 4326), -- PostGIS type
    country VARCHAR(100),
    province VARCHAR(100),
    district VARCHAR(100),
    neighborhood VARCHAR(100),
    is_complete BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- ============================================================================
-- POSTS SYSTEM
-- ============================================================================

CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    business_id UUID REFERENCES business_profiles(id) ON DELETE SET NULL,
    original_post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    category_id UUID REFERENCES sell_categories(id) ON DELETE SET NULL,

    -- Content fields
    title VARCHAR(255),
    description TEXT,
    type VARCHAR(20) NOT NULL, -- 'FEED', 'EVENT', 'SELL', 'PULL'
    status BOOLEAN DEFAULT true,
    visibility VARCHAR(20) DEFAULT 'PUBLIC', -- 'PUBLIC', 'FRIENDS', 'PRIVATE'

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

CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    photo JSONB NOT NULL, -- {url, name, mime_type, size, width, height}
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- ============================================================================
-- SOCIAL INTERACTIONS
-- ============================================================================

CREATE TABLE post_likes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

CREATE TABLE post_bookmarks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

CREATE TABLE post_shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    shared_post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    share_text TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- COMMENTS SYSTEM
-- ============================================================================

CREATE TABLE post_comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_comment_id UUID REFERENCES post_comments(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    location GEOGRAPHY(POINT, 4326),
    total_likes INTEGER DEFAULT 0,
    total_replies INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE comment_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    comment_id UUID NOT NULL REFERENCES post_comments(id) ON DELETE CASCADE,
    photo JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE post_comment_likes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment_id UUID NOT NULL REFERENCES post_comments(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, comment_id)
);

-- ============================================================================
-- POLLS SYSTEM
-- ============================================================================

CREATE TABLE polls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE poll_options (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option TEXT NOT NULL,
    vote_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE user_polls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    poll_option_id UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, poll_id)
);

-- ============================================================================
-- BUSINESS PROFILES
-- ============================================================================

CREATE TABLE business_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    license_no VARCHAR(100),
    description TEXT,
    address TEXT,
    phone_number VARCHAR(20),
    email VARCHAR(255),
    website VARCHAR(255),
    avatar JSONB,
    cover JSONB,
    status BOOLEAN DEFAULT true,
    additional_info TEXT,
    address_location GEOGRAPHY(POINT, 4326),
    country VARCHAR(100),
    province VARCHAR(100),
    district VARCHAR(100),
    neighborhood VARCHAR(100),
    show_location BOOLEAN DEFAULT true,
    total_views INTEGER DEFAULT 0,
    total_follow INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE business_categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE business_profile_categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_profile_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    business_category_id UUID NOT NULL REFERENCES business_categories(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(business_profile_id, business_category_id)
);

CREATE TABLE business_hours (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_profile_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    day VARCHAR(10) NOT NULL, -- 'Monday', 'Tuesday', etc.
    open_time TIME,
    close_time TIME,
    is_closed BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE business_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_profile_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    photo JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE business_profile_followers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(business_id, follower_id)
);

-- ============================================================================
-- CATEGORIES
-- ============================================================================

CREATE TABLE sell_categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    icon JSONB NOT NULL, -- {name, library}
    color VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'ACTIVE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- EVENT INTERACTIONS
-- ============================================================================

CREATE TABLE event_interests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_state VARCHAR(20) NOT NULL, -- 'interested', 'going', 'not_interested'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(post_id, user_id)
);

-- ============================================================================
-- REPORTING SYSTEM
-- ============================================================================

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

CREATE TABLE comment_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment_id UUID NOT NULL REFERENCES post_comments(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL,
    additional_comments TEXT,
    report_status VARCHAR(20) DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE user_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reported_user UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_by_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL,
    description TEXT,
    resolved BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE business_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_id UUID NOT NULL REFERENCES business_profiles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL,
    additional_comments TEXT,
    report_status VARCHAR(20) DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- USER RELATIONSHIPS
-- ============================================================================

CREATE TABLE user_follows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(follower_id, following_id),
    CONSTRAINT no_self_follow CHECK (follower_id != following_id)
);

CREATE TABLE user_blocks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(blocker_id, blocked_id),
    CONSTRAINT no_self_block CHECK (blocker_id != blocked_id)
);

-- ============================================================================
-- CHAT SYSTEM
-- ============================================================================

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    participant1_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    participant2_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_message_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(participant1_id, participant2_id),
    CONSTRAINT ordered_participants CHECK (participant1_id < participant2_id)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT,
    message_type VARCHAR(20) DEFAULT 'TEXT',
    read_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- ============================================================================
-- NOTIFICATIONS
-- ============================================================================

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255),
    message TEXT,
    data JSONB,
    read BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE notification_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL, -- 'POSTS', 'MESSAGES', 'EVENTS', 'SALES'
    push_pref BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(profile_id, category)
);

-- ============================================================================
-- PRIVACY & SETTINGS
-- ============================================================================

CREATE TABLE privacy_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE UNIQUE,
    full_name_visibility VARCHAR(50) DEFAULT 'Anyone on Hamsaya',
    profile_photos_visibility VARCHAR(50) DEFAULT 'Anyone on Hamsaya',
    profile_bio_visibility VARCHAR(50) DEFAULT 'Anyone on Hamsaya',
    discoverability_visibility VARCHAR(50) DEFAULT 'Anyone on Hamsaya',
    mention_visibility VARCHAR(50) DEFAULT 'Anyone can mention me',
    direct_message_visibility VARCHAR(50) DEFAULT 'Anyone on Hamsaya',
    sync_contacts BOOLEAN DEFAULT false,
    discover_by_email BOOLEAN DEFAULT true,
    discover_by_phone BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- AUTHENTICATION & SECURITY
-- ============================================================================

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token VARCHAR(255) UNIQUE NOT NULL,
    access_token_hash VARCHAR(255) NOT NULL,
    device_info JSONB,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN DEFAULT false,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE token_blacklist (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    reason VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE email_verifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- MFA (Multi-Factor Authentication)
-- ============================================================================

CREATE TABLE mfa_factors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    factor_type VARCHAR(20) NOT NULL, -- 'TOTP', 'SMS', 'EMAIL'
    secret_key TEXT,
    factor_id VARCHAR(255) UNIQUE,
    status VARCHAR(20) DEFAULT 'unverified', -- 'verified', 'unverified'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE mfa_challenges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    factor_id UUID REFERENCES mfa_factors(id) ON DELETE CASCADE,
    challenge_id VARCHAR(255) UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE mfa_backup_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(20) UNIQUE NOT NULL,
    used BOOLEAN DEFAULT false,
    used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- ACCOUNT MANAGEMENT
-- ============================================================================

CREATE TABLE account_deletion_reasons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL,
    details TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================

-- Users & Profiles
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_oauth ON users(oauth_provider, oauth_provider_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_profiles_location ON profiles USING GIST(location) WHERE deleted_at IS NULL;
CREATE INDEX idx_profiles_province ON profiles(province) WHERE deleted_at IS NULL;

-- Posts
CREATE INDEX idx_posts_user_id ON posts(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_business_id ON posts(business_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_type ON posts(type) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_created_at ON posts(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_location ON posts USING GIST(address_location) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_category ON posts(category_id) WHERE deleted_at IS NULL AND type = 'SELL';
CREATE INDEX idx_posts_sold ON posts(sold) WHERE deleted_at IS NULL AND type = 'SELL';
CREATE INDEX idx_posts_province ON posts(province) WHERE deleted_at IS NULL;

-- Social Interactions
CREATE INDEX idx_post_likes_post_id ON post_likes(post_id);
CREATE INDEX idx_post_likes_user_id ON post_likes(user_id);
CREATE INDEX idx_post_bookmarks_user_id ON post_bookmarks(user_id);
CREATE INDEX idx_post_comments_post_id ON post_comments(post_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_post_comments_user_id ON post_comments(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_post_comments_parent ON post_comments(parent_comment_id) WHERE deleted_at IS NULL;

-- Business
CREATE INDEX idx_business_profiles_user_id ON business_profiles(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_business_profiles_location ON business_profiles USING GIST(address_location) WHERE deleted_at IS NULL;
CREATE INDEX idx_business_followers_business ON business_profile_followers(business_id);

-- Authentication
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id) WHERE revoked = false;
CREATE INDEX idx_user_sessions_refresh_token ON user_sessions(refresh_token) WHERE revoked = false;
CREATE INDEX idx_token_blacklist_token_hash ON token_blacklist(token_hash) WHERE expires_at > NOW();
CREATE INDEX idx_email_verifications_token ON email_verifications(token) WHERE used = false;
CREATE INDEX idx_password_reset_tokens_token ON password_reset_tokens(token) WHERE used = false;

-- MFA
CREATE INDEX idx_mfa_factors_user_id ON mfa_factors(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_mfa_challenges_challenge_id ON mfa_challenges(challenge_id) WHERE used = false;

-- Notifications
CREATE INDEX idx_notifications_user_id ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id) WHERE read = false;

-- Chat
CREATE INDEX idx_conversations_participants ON conversations(participant1_id, participant2_id);
CREATE INDEX idx_messages_conversation ON messages(conversation_id, created_at DESC) WHERE deleted_at IS NULL;

-- ============================================================================
-- TRIGGERS FOR COUNTERS
-- ============================================================================

-- Update post total_likes counter
CREATE OR REPLACE FUNCTION update_post_likes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET total_likes = total_likes + 1 WHERE id = NEW.post_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET total_likes = GREATEST(total_likes - 1, 0) WHERE id = OLD.post_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_likes_count
AFTER INSERT OR DELETE ON post_likes
FOR EACH ROW EXECUTE FUNCTION update_post_likes_count();

-- Update post total_comments counter
CREATE OR REPLACE FUNCTION update_post_comments_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET total_comments = total_comments + 1 WHERE id = NEW.post_id;
    ELSIF TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL) THEN
        UPDATE posts SET total_comments = GREATEST(total_comments - 1, 0) WHERE id = COALESCE(NEW.post_id, OLD.post_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_comments_count
AFTER INSERT OR UPDATE OR DELETE ON post_comments
FOR EACH ROW EXECUTE FUNCTION update_post_comments_count();

-- Update comment total_likes counter
CREATE OR REPLACE FUNCTION update_comment_likes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE post_comments SET total_likes = total_likes + 1 WHERE id = NEW.comment_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE post_comments SET total_likes = GREATEST(total_likes - 1, 0) WHERE id = OLD.comment_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_comment_likes_count
AFTER INSERT OR DELETE ON post_comment_likes
FOR EACH ROW EXECUTE FUNCTION update_comment_likes_count();

-- Update comment total_replies counter
CREATE OR REPLACE FUNCTION update_comment_replies_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.parent_comment_id IS NOT NULL THEN
        UPDATE post_comments SET total_replies = total_replies + 1 WHERE id = NEW.parent_comment_id;
    ELSIF (TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL)) AND COALESCE(NEW.parent_comment_id, OLD.parent_comment_id) IS NOT NULL THEN
        UPDATE post_comments SET total_replies = GREATEST(total_replies - 1, 0) WHERE id = COALESCE(NEW.parent_comment_id, OLD.parent_comment_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_comment_replies_count
AFTER INSERT OR UPDATE OR DELETE ON post_comments
FOR EACH ROW EXECUTE FUNCTION update_comment_replies_count();

-- Update poll option vote_count
CREATE OR REPLACE FUNCTION update_poll_vote_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE poll_options SET vote_count = vote_count + 1 WHERE id = NEW.poll_option_id;
    ELSIF TG_OP = 'UPDATE' AND NEW.poll_option_id != OLD.poll_option_id THEN
        UPDATE poll_options SET vote_count = GREATEST(vote_count - 1, 0) WHERE id = OLD.poll_option_id;
        UPDATE poll_options SET vote_count = vote_count + 1 WHERE id = NEW.poll_option_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE poll_options SET vote_count = GREATEST(vote_count - 1, 0) WHERE id = OLD.poll_option_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_poll_vote_count
AFTER INSERT OR UPDATE OR DELETE ON user_polls
FOR EACH ROW EXECUTE FUNCTION update_poll_vote_count();

-- Update business follower count
CREATE OR REPLACE FUNCTION update_business_follow_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE business_profiles SET total_follow = total_follow + 1 WHERE id = NEW.business_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE business_profiles SET total_follow = GREATEST(total_follow - 1, 0) WHERE id = OLD.business_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_business_follow_count
AFTER INSERT OR DELETE ON business_profile_followers
FOR EACH ROW EXECUTE FUNCTION update_business_follow_count();

-- Update event interest counts
CREATE OR REPLACE FUNCTION update_event_interest_counts()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.event_state = 'interested' THEN
            UPDATE posts SET interested_count = interested_count + 1 WHERE id = NEW.post_id;
        ELSIF NEW.event_state = 'going' THEN
            UPDATE posts SET going_count = going_count + 1 WHERE id = NEW.post_id;
        END IF;
    ELSIF TG_OP = 'UPDATE' AND NEW.event_state != OLD.event_state THEN
        IF OLD.event_state = 'interested' THEN
            UPDATE posts SET interested_count = GREATEST(interested_count - 1, 0) WHERE id = NEW.post_id;
        ELSIF OLD.event_state = 'going' THEN
            UPDATE posts SET going_count = GREATEST(going_count - 1, 0) WHERE id = NEW.post_id;
        END IF;
        IF NEW.event_state = 'interested' THEN
            UPDATE posts SET interested_count = interested_count + 1 WHERE id = NEW.post_id;
        ELSIF NEW.event_state = 'going' THEN
            UPDATE posts SET going_count = going_count + 1 WHERE id = NEW.post_id;
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.event_state = 'interested' THEN
            UPDATE posts SET interested_count = GREATEST(interested_count - 1, 0) WHERE id = OLD.post_id;
        ELSIF OLD.event_state = 'going' THEN
            UPDATE posts SET going_count = GREATEST(going_count - 1, 0) WHERE id = OLD.post_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_event_interest_counts
AFTER INSERT OR UPDATE OR DELETE ON event_interests
FOR EACH ROW EXECUTE FUNCTION update_event_interest_counts();

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Calculate distance between two geography points (returns meters)
CREATE OR REPLACE FUNCTION calculate_distance(
    point1 GEOGRAPHY,
    point2 GEOGRAPHY
) RETURNS DOUBLE PRECISION AS $$
BEGIN
    RETURN ST_Distance(point1, point2);
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Get posts within radius
CREATE OR REPLACE FUNCTION get_nearby_posts(
    user_lat DOUBLE PRECISION,
    user_lng DOUBLE PRECISION,
    radius_meters DOUBLE PRECISION,
    post_type VARCHAR DEFAULT NULL
) RETURNS TABLE (
    post_id UUID,
    distance_meters DOUBLE PRECISION
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        ST_Distance(
            p.address_location::geography,
            ST_SetSRID(ST_MakePoint(user_lng, user_lat), 4326)::geography
        ) as distance
    FROM posts p
    WHERE
        p.deleted_at IS NULL
        AND p.address_location IS NOT NULL
        AND ST_DWithin(
            p.address_location::geography,
            ST_SetSRID(ST_MakePoint(user_lng, user_lat), 4326)::geography,
            radius_meters
        )
        AND (post_type IS NULL OR p.type = post_type)
    ORDER BY distance;
END;
$$ LANGUAGE plpgsql;
```

---

## API Endpoints (Complete Mapping)

Based on comprehensive analysis of all repository files, here's the complete API specification matching the Flutter app's needs:

### Authentication & User Management

```
POST   /api/v1/auth/register                  # Email/password registration
POST   /api/v1/auth/login                     # Email/password login
POST   /api/v1/auth/logout                    # Sign out current session
POST   /api/v1/auth/refresh                   # Refresh access token
POST   /api/v1/auth/oauth/google              # Google OAuth sign-in
POST   /api/v1/auth/oauth/apple               # Apple OAuth sign-in
POST   /api/v1/auth/oauth/facebook            # Facebook OAuth sign-in
POST   /api/v1/auth/forgot-password           # Request password reset
POST   /api/v1/auth/reset-password            # Reset password with token
POST   /api/v1/auth/verify-email              # Verify email with token
POST   /api/v1/auth/resend-verification       # Resend verification email
POST   /api/v1/auth/change-password           # Change password (authenticated)

# MFA Endpoints
POST   /api/v1/auth/mfa/enroll                # Enroll in MFA (returns QR code)
POST   /api/v1/auth/mfa/verify                # Verify MFA code
POST   /api/v1/auth/mfa/disable               # Disable MFA
POST   /api/v1/auth/mfa/challenge             # Create MFA challenge (login flow)
POST   /api/v1/auth/mfa/verify-challenge      # Verify MFA challenge
GET    /api/v1/auth/mfa/backup-codes          # Get backup codes
POST   /api/v1/auth/mfa/backup-codes          # Generate new backup codes
POST   /api/v1/auth/mfa/backup-codes/verify   # Verify backup code

# Session Management
GET    /api/v1/auth/sessions                  # List active sessions
DELETE /api/v1/auth/sessions/:session_id      # Revoke specific session
DELETE /api/v1/auth/sessions                  # Revoke all sessions
```

### Profile Management

```
GET    /api/v1/users/me                       # Get authenticated user profile
PUT    /api/v1/users/me                       # Update profile (upsert operation)
POST   /api/v1/users/me/avatar                # Upload avatar (multipart)
POST   /api/v1/users/me/cover                 # Upload cover photo (multipart)
DELETE /api/v1/users/me/avatar                # Delete avatar
DELETE /api/v1/users/me/cover                 # Delete cover
GET    /api/v1/users/:user_id                 # Get user profile by ID
GET    /api/v1/users/search                   # Search users (query params)

# User Relationships
GET    /api/v1/users/:user_id/followers       # Get followers list
GET    /api/v1/users/:user_id/following       # Get following list
POST   /api/v1/users/:user_id/follow          # Follow user
DELETE /api/v1/users/:user_id/follow          # Unfollow user
POST   /api/v1/users/:user_id/block           # Block user
DELETE /api/v1/users/:user_id/block           # Unblock user
GET    /api/v1/users/:user_id/blocked         # Check if blocked
POST   /api/v1/users/:user_id/report          # Report user

# Bookmarks
GET    /api/v1/users/me/bookmarks             # Get bookmarked posts (paginated)

# Account Management
DELETE /api/v1/users/me/account               # Delete account (with reason)
```

### Post Management

```
# Core Post Operations
GET    /api/v1/posts                          # Get feed (paginated, filtered)
POST   /api/v1/posts                          # Create post (any type)
GET    /api/v1/posts/:post_id                 # Get post by ID
PUT    /api/v1/posts/:post_id                 # Update post
DELETE /api/v1/posts/:post_id                 # Soft delete post

# Post Interactions
POST   /api/v1/posts/:post_id/like            # Like post (idempotent)
DELETE /api/v1/posts/:post_id/like            # Unlike post
POST   /api/v1/posts/:post_id/bookmark        # Bookmark post
DELETE /api/v1/posts/:post_id/bookmark        # Remove bookmark
POST   /api/v1/posts/:post_id/share           # Share post
POST   /api/v1/posts/:post_id/report          # Report post

# Post Discovery
GET    /api/v1/posts/trending                 # Trending posts (engagement-based)
GET    /api/v1/posts/nearby                   # Location-based posts
GET    /api/v1/posts/search                   # Search posts (full-text)

# User Posts
GET    /api/v1/users/:user_id/posts           # Get user's posts (paginated)
GET    /api/v1/users/me/posts                 # Get authenticated user's posts
```

### Comment Management

```
GET    /api/v1/posts/:post_id/comments        # Get comments (paginated, nested support)
POST   /api/v1/posts/:post_id/comments        # Create comment (with attachments)
GET    /api/v1/comments/:comment_id           # Get comment by ID
PUT    /api/v1/comments/:comment_id           # Update comment
DELETE /api/v1/comments/:comment_id           # Soft delete comment

# Comment Interactions
POST   /api/v1/comments/:comment_id/like      # Like comment
DELETE /api/v1/comments/:comment_id/like      # Unlike comment
POST   /api/v1/comments/:comment_id/report    # Report comment

# Nested Comments (Replies)
GET    /api/v1/comments/:comment_id/replies   # Get replies to comment
POST   /api/v1/comments/:comment_id/replies   # Reply to comment
```

### Poll Management

```
POST   /api/v1/posts/:post_id/polls           # Create poll (on PULL type post)
GET    /api/v1/posts/:post_id/polls           # Get poll data
POST   /api/v1/polls/:poll_id/vote            # Vote on poll option (upsert)
DELETE /api/v1/polls/:poll_id/vote            # Remove vote
GET    /api/v1/polls/:poll_id/results         # Get poll results
```

### Business Management

```
# Business Profile Operations
GET    /api/v1/businesses                     # List authenticated user's businesses
POST   /api/v1/businesses                     # Create business profile
GET    /api/v1/businesses/:business_id        # Get business by ID
PUT    /api/v1/businesses/:business_id        # Update business details
DELETE /api/v1/businesses/:business_id        # Delete business (soft delete)

# Business Media
POST   /api/v1/businesses/:business_id/avatar              # Upload avatar
POST   /api/v1/businesses/:business_id/cover               # Upload cover
POST   /api/v1/businesses/:business_id/attachments         # Upload gallery images
DELETE /api/v1/businesses/:business_id/attachments/:id     # Delete gallery image
GET    /api/v1/businesses/:business_id/attachments         # List gallery images

# Business Categories
GET    /api/v1/businesses/categories          # List all business categories
GET    /api/v1/businesses/:business_id/categories         # Get business categories
PUT    /api/v1/businesses/:business_id/categories         # Update categories (replace)

# Business Working Hours
GET    /api/v1/businesses/:business_id/hours  # Get working hours
PUT    /api/v1/businesses/:business_id/hours  # Update working hours (upsert)

# Business Location
PUT    /api/v1/businesses/:business_id/location            # Update location

# Business Interactions
POST   /api/v1/businesses/:business_id/follow              # Follow business
DELETE /api/v1/businesses/:business_id/follow              # Unfollow business
GET    /api/v1/businesses/:business_id/followers           # Get followers list
POST   /api/v1/businesses/:business_id/report              # Report business

# Business Posts
GET    /api/v1/businesses/:business_id/posts  # Get business posts (paginated)
```

### Sales & Marketplace

```
GET    /api/v1/sales                          # List sell posts (filtered, paginated)
GET    /api/v1/sales/:post_id                 # Get sell post details
GET    /api/v1/sales/categories               # List sell categories
GET    /api/v1/sales/my-listings              # Get user's sell posts
PUT    /api/v1/sales/:post_id/sold            # Mark item as sold
PUT    /api/v1/sales/:post_id/available       # Mark item as available
```

### Event Management

```
GET    /api/v1/events                         # List events (filtered, paginated)
GET    /api/v1/events/:post_id                # Get event details
POST   /api/v1/events/:post_id/interest       # Express interest (interested/going/not_interested)
GET    /api/v1/events/:post_id/interested     # Get list of interested users
GET    /api/v1/events/:post_id/going          # Get list of going users
```

### Discovery & Search

```
GET    /api/v1/discover                       # Discover content (map-based)
GET    /api/v1/discover/markers               # Get map markers (posts/businesses)
GET    /api/v1/search                         # Global search (posts, users, businesses)
GET    /api/v1/search/posts                   # Search posts specifically
GET    /api/v1/search/users                   # Search users specifically
GET    /api/v1/search/businesses              # Search businesses specifically
```

### Chat & Messaging

```
GET    /api/v1/chat/conversations             # List conversations (paginated)
POST   /api/v1/chat/conversations             # Create conversation
GET    /api/v1/chat/conversations/:id         # Get conversation details
GET    /api/v1/chat/conversations/:id/messages # Get messages (paginated)
POST   /api/v1/chat/messages                  # Send message
DELETE /api/v1/chat/messages/:id              # Delete message (soft delete)
PUT    /api/v1/chat/messages/:id/read         # Mark message as read
```

### Notifications

```
GET    /api/v1/notifications                  # List notifications (paginated)
PUT    /api/v1/notifications/:id/read         # Mark as read
PUT    /api/v1/notifications/read-all         # Mark all as read
GET    /api/v1/notifications/unread-count     # Get unread count
DELETE /api/v1/notifications/:id              # Delete notification

# Notification Settings
GET    /api/v1/settings/notifications         # Get notification settings
PUT    /api/v1/settings/notifications         # Update notification settings (by category)
```

### Privacy Settings

```
GET    /api/v1/settings/privacy               # Get privacy settings
PUT    /api/v1/settings/privacy               # Update privacy settings (upsert)
```

### File Upload

```
POST   /api/v1/upload                         # Generic file upload (returns URL)
POST   /api/v1/upload/avatar                  # Upload avatar (auto-crop/resize)
POST   /api/v1/upload/cover                   # Upload cover (auto-crop/resize)
POST   /api/v1/upload/post                    # Upload post attachment
POST   /api/v1/upload/comment                 # Upload comment attachment
POST   /api/v1/upload/business                # Upload business attachment
DELETE /api/v1/upload/:path                   # Delete uploaded file
```

### Health & Monitoring

```
GET    /health                                # Health check endpoint
GET    /health/ready                          # Readiness probe
GET    /health/live                           # Liveness probe
GET    /metrics                               # Prometheus metrics
```

---

## Authentication & Security

### JWT Token Implementation

**Access Token Structure:**
```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "aal": 1,
  "iat": 1234567890,
  "exp": 1234568790,
  "iss": "hamsaya"
}
```

**AAL (Authentication Assurance Level):**
- **AAL1**: Basic authentication (email/password or OAuth)
- **AAL2**: MFA verified (required for sensitive operations)

**Token Lifecycle:**
- **Access Token**: 15 minutes
- **Refresh Token**: 7 days
- **Token Rotation**: New refresh token on each refresh
- **Blacklist**: Maintains revoked tokens until expiry

### OAuth2 Implementation

**Google Sign-In Flow:**
1. Client initiates OAuth flow
2. User authenticates with Google
3. Client receives `id_token`
4. Client sends `id_token` to `/api/v1/auth/oauth/google`
5. Server verifies token with Google
6. Server creates/updates user account
7. Server returns JWT token pair

**Apple Sign-In Flow:**
1. Similar to Google but with nonce validation
2. Uses Apple's JWT token verification
3. Extracts email (may be private relay email)
4. Creates/links account

**Facebook Sign-In Flow:**
1. Client gets Facebook access token
2. Sends to `/api/v1/auth/oauth/facebook`
3. Server validates with Facebook Graph API
4. Returns JWT token pair

### MFA (TOTP) Implementation

**Enrollment Process:**
```
POST /api/v1/auth/mfa/enroll
Response: {
  "factor_id": "uuid",
  "qr_code_url": "data:image/png;base64,...",
  "secret_key": "BASE32SECRET",
  "backup_codes": ["12345678", "23456789", ...]
}
```

**Login with MFA:**
```
1. POST /api/v1/auth/login (email/password)
   Response: { "requires_mfa": true, "challenge_id": "uuid" }

2. POST /api/v1/auth/mfa/verify-challenge
   Body: { "challenge_id": "uuid", "code": "123456" }
   Response: { "access_token": "...", "refresh_token": "..." }
```

### Password Security

- **Hashing**: bcrypt with cost factor 12
- **Strength Requirements**:
  - Minimum 8 characters
  - At least 1 uppercase letter
  - At least 1 lowercase letter
  - At least 1 digit
  - At least 1 special character
- **Password History**: Store hash of last 5 passwords
- **Account Lockout**: 5 failed attempts = 15 minute lockout

### Rate Limiting

**Per-User Limits:**
- **Authentication**: 5 attempts per 15 minutes
- **API Requests**: 1000 requests per hour
- **File Uploads**: 10 uploads per minute
- **Post Creation**: 10 posts per hour

**Per-IP Limits:**
- **Unauthenticated**: 100 requests per hour
- **Burst**: 20 requests per minute

**Implementation**: Redis-based sliding window

---

## Core Services Implementation

### Service Architecture

```
internal/
├── services/
│   ├── auth_service.go          # Authentication & session management
│   ├── user_service.go          # User profile CRUD
│   ├── post_service.go          # Post management
│   ├── comment_service.go       # Comment management
│   ├── poll_service.go          # Poll management
│   ├── business_service.go      # Business profile management
│   ├── chat_service.go          # Real-time messaging
│   ├── notification_service.go  # Push notifications
│   ├── storage_service.go       # File storage abstraction
│   ├── location_service.go      # Geocoding & distance calculation
│   └── feed_service.go          # Feed algorithm & discovery
```

### Key Service Patterns

#### 1. Repository Pattern

```go
// internal/repositories/user_repository.go
type UserRepository interface {
    Create(ctx context.Context, user *models.User) error
    GetByID(ctx context.Context, id string) (*models.User, error)
    GetByEmail(ctx context.Context, email string) (*models.User, error)
    Update(ctx context.Context, user *models.User) error
    Delete(ctx context.Context, id string) error
}

type postgresUserRepository struct {
    db *pgxpool.Pool
}

func (r *postgresUserRepository) Create(ctx context.Context, user *models.User) error {
    query := `
        INSERT INTO users (id, email, password_hash, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `
    return r.db.QueryRow(ctx, query,
        user.ID, user.Email, user.PasswordHash,
        time.Now(), time.Now(),
    ).Scan(&user.ID)
}
```

#### 2. Service Layer

```go
// internal/services/user_service.go
type UserService struct {
    userRepo    repositories.UserRepository
    profileRepo repositories.ProfileRepository
    authService *AuthService
    cache       *redis.Client
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*models.User, *TokenPair, error) {
    // Validate input
    if err := s.validator.Struct(req); err != nil {
        return nil, nil, ErrInvalidInput
    }

    // Check if user exists
    existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
    if existing != nil {
        return nil, nil, ErrUserExists
    }

    // Hash password
    passwordHash, err := s.authService.HashPassword(req.Password)
    if err != nil {
        return nil, nil, err
    }

    // Create user
    user := &models.User{
        ID:           uuid.New().String(),
        Email:        req.Email,
        PasswordHash: passwordHash,
    }

    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, nil, err
    }

    // Create profile
    profile := &models.Profile{
        ID:        user.ID,
        FirstName: req.FirstName,
        LastName:  req.LastName,
    }

    if err := s.profileRepo.Create(ctx, profile); err != nil {
        return nil, nil, err
    }

    // Generate tokens
    tokens, err := s.authService.GenerateTokenPair(user.ID, user.Email)
    if err != nil {
        return nil, nil, err
    }

    return user, tokens, nil
}
```

#### 3. Transaction Pattern

```go
// internal/services/post_service.go
func (s *PostService) CreatePost(ctx context.Context, userID string, req *CreatePostRequest) (*models.Post, error) {
    // Begin transaction
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)

    // Create post
    post := &models.Post{
        ID:          uuid.New().String(),
        UserID:      userID,
        Description: req.Description,
        Type:        req.Type,
        Visibility:  req.Visibility,
    }

    if err := s.postRepo.CreateTx(ctx, tx, post); err != nil {
        return nil, err
    }

    // Upload and attach images
    for _, imageData := range req.Images {
        url, err := s.storageService.UploadImage(ctx, imageData)
        if err != nil {
            return nil, err
        }

        attachment := &models.Attachment{
            PostID: post.ID,
            Photo:  models.Photo{URL: url, ...},
        }

        if err := s.attachmentRepo.CreateTx(ctx, tx, attachment); err != nil {
            return nil, err
        }
    }

    // If PULL type, create poll
    if req.Type == "PULL" && req.PollOptions != nil {
        if err := s.pollService.CreatePollTx(ctx, tx, post.ID, req.PollOptions); err != nil {
            return nil, err
        }
    }

    // Commit transaction
    if err := tx.Commit(ctx); err != nil {
        return nil, err
    }

    // Invalidate cache
    s.cache.Del(ctx, fmt.Sprintf("user:posts:%s", userID))

    return post, nil
}
```

#### 4. Feed Algorithm

```go
// internal/services/feed_service.go
type FeedService struct {
    postRepo     repositories.PostRepository
    userRepo     repositories.UserRepository
    cache        *redis.Client
    locationSvc  *LocationService
}

func (s *FeedService) GetFeed(ctx context.Context, userID string, filter *FeedFilter) ([]*models.Post, error) {
    // Get user's location
    user, err := s.userRepo.GetByID(ctx, userID)
    if err != nil {
        return nil, err
    }

    var posts []*models.Post

    switch filter.SortType {
    case "recent":
        posts, err = s.postRepo.GetRecent(ctx, filter.Limit, filter.Offset)

    case "nearby":
        if user.Location != nil {
            posts, err = s.postRepo.GetNearby(ctx,
                user.Location.Lat, user.Location.Lng,
                filter.RadiusKm, filter.Limit, filter.Offset)
        }

    case "trending":
        posts, err = s.calculateTrending(ctx, filter.Limit, filter.Offset)
    }

    if err != nil {
        return nil, err
    }

    // Enrich posts with user-specific data (liked_by_me, bookmarked_by_me)
    return s.enrichPosts(ctx, posts, userID), nil
}

func (s *FeedService) calculateTrending(ctx context.Context, limit, offset int) ([]*models.Post, error) {
    // Trending score = (likes * 2 + comments * 3 + shares * 5) / age_hours^1.5
    query := `
        SELECT p.*,
            ((p.total_likes * 2 + p.total_comments * 3 + p.total_shares * 5) /
             POWER(EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600, 1.5)) AS trending_score
        FROM posts p
        WHERE p.deleted_at IS NULL
            AND p.created_at > NOW() - INTERVAL '7 days'
        ORDER BY trending_score DESC
        LIMIT $1 OFFSET $2
    `

    // Execute query and scan results...
}
```

---

## Image Storage & Processing

### Storage Architecture

```
Client → API → Image Processor → Object Storage (S3/MinIO)
                     ↓
                 Validation
                     ↓
              Compression/Resize
                     ↓
                 WebP Conversion
                     ↓
              Generate Thumbnails
                     ↓
                Upload to Storage
```

### Implementation

```go
// pkg/storage/storage_service.go
type StorageService struct {
    s3Client     *minio.Client
    bucketName   string
    cdnURL       string
    imageProc    *ImageProcessor
}

func (s *StorageService) UploadImage(ctx context.Context, file multipart.File, filename string, imageType ImageType) (*Photo, error) {
    // Validate image
    if err := s.validateImage(file); err != nil {
        return nil, err
    }

    // Decode image
    img, format, err := image.Decode(file)
    if err != nil {
        return nil, err
    }

    // Process based on type
    var processed image.Image
    switch imageType {
    case ImageTypeAvatar:
        processed = s.imageProc.ResizeAndCrop(img, 400, 400)
    case ImageTypeCover:
        processed = s.imageProc.ResizeAndCrop(img, 1600, 900)
    case ImageTypePost:
        processed = s.imageProc.Resize(img, 1200, 0) // Maintain aspect ratio
    }

    // Convert to WebP
    buf := new(bytes.Buffer)
    if err := webp.Encode(buf, processed, &webp.Options{Quality: 80}); err != nil {
        return nil, err
    }

    // Generate unique filename
    objectName := fmt.Sprintf("%s/%s.webp", uuid.New().String(), filename)

    // Upload to S3
    _, err = s.s3Client.PutObject(ctx, s.bucketName, objectName,
        buf, int64(buf.Len()),
        minio.PutObjectOptions{ContentType: "image/webp"})
    if err != nil {
        return nil, err
    }

    // Generate public URL
    url := fmt.Sprintf("%s/%s/%s", s.cdnURL, s.bucketName, objectName)

    bounds := processed.Bounds()
    return &Photo{
        URL:      url,
        Name:     filename,
        MimeType: "image/webp",
        Size:     buf.Len(),
        Width:    bounds.Dx(),
        Height:   bounds.Dy(),
    }, nil
}

func (s *StorageService) DeleteImage(ctx context.Context, url string) error {
    // Extract object name from URL
    objectName := s.extractObjectName(url)

    return s.s3Client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
}
```

### Image Processor

```go
// pkg/storage/image_processor.go
type ImageProcessor struct {}

func (p *ImageProcessor) ResizeAndCrop(img image.Image, width, height int) image.Image {
    // Calculate aspect ratio
    bounds := img.Bounds()
    srcWidth := bounds.Dx()
    srcHeight := bounds.Dy()

    srcRatio := float64(srcWidth) / float64(srcHeight)
    dstRatio := float64(width) / float64(height)

    var resized image.Image
    if srcRatio > dstRatio {
        // Wider: resize height, crop width
        resized = imaging.Resize(img, 0, height, imaging.Lanczos)
        resized = imaging.CropCenter(resized, width, height)
    } else {
        // Taller: resize width, crop height
        resized = imaging.Resize(img, width, 0, imaging.Lanczos)
        resized = imaging.CropCenter(resized, width, height)
    }

    return resized
}

func (p *ImageProcessor) Resize(img image.Image, width, height int) image.Image {
    return imaging.Fit(img, width, height, imaging.Lanczos)
}

func (p *ImageProcessor) Compress(img image.Image, quality int) ([]byte, error) {
    buf := new(bytes.Buffer)
    err := webp.Encode(buf, img, &webp.Options{Quality: float32(quality)})
    return buf.Bytes(), err
}
```

---

## Real-time Features

### WebSocket Implementation

```go
// pkg/websocket/manager.go
type Manager struct {
    clients    map[string]*Client
    broadcast  chan *Message
    register   chan *Client
    unregister chan *Client
    mu         sync.RWMutex
}

type Client struct {
    ID     string
    UserID string
    Conn   *websocket.Conn
    Send   chan *Message
}

type Message struct {
    Type    string          `json:"type"`    // "chat", "notification", "typing"
    From    string          `json:"from"`
    To      string          `json:"to"`
    Content json.RawMessage `json:"content"`
}

func NewManager() *Manager {
    return &Manager{
        clients:    make(map[string]*Client),
        broadcast:  make(chan *Message, 256),
        register:   make(chan *Client),
        unregister: make(chan *Client),
    }
}

func (m *Manager) Run() {
    for {
        select {
        case client := <-m.register:
            m.mu.Lock()
            m.clients[client.ID] = client
            m.mu.Unlock()

        case client := <-m.unregister:
            m.mu.Lock()
            if _, ok := m.clients[client.ID]; ok {
                close(client.Send)
                delete(m.clients, client.ID)
            }
            m.mu.Unlock()

        case message := <-m.broadcast:
            m.mu.RLock()
            if client, ok := m.clients[message.To]; ok {
                select {
                case client.Send <- message:
                default:
                    close(client.Send)
                    delete(m.clients, client.ID)
                }
            }
            m.mu.RUnlock()
        }
    }
}

func (m *Manager) HandleWebSocket(w http.ResponseWriter, r *http.Request, userID string) {
    conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
    if err != nil {
        return
    }

    client := &Client{
        ID:     uuid.New().String(),
        UserID: userID,
        Conn:   conn,
        Send:   make(chan *Message, 256),
    }

    m.register <- client

    go client.writePump()
    go client.readPump(m)
}

func (c *Client) readPump(m *Manager) {
    defer func() {
        m.unregister <- c
        c.Conn.Close()
    }()

    for {
        var msg Message
        if err := c.Conn.ReadJSON(&msg); err != nil {
            break
        }

        msg.From = c.UserID
        m.broadcast <- &msg
    }
}

func (c *Client) writePump() {
    defer c.Conn.Close()

    for message := range c.Send {
        if err := c.Conn.WriteJSON(message); err != nil {
            return
        }
    }
}
```

### Push Notifications (Firebase)

```go
// pkg/notification/firebase.go
type FirebaseService struct {
    app *firebase.App
    fcm *messaging.Client
}

func (s *FirebaseService) SendNotification(ctx context.Context, userID string, notification *Notification) error {
    // Get user's device tokens from DB
    tokens, err := s.getUserDeviceTokens(ctx, userID)
    if err != nil {
        return err
    }

    // Build message
    message := &messaging.MulticastMessage{
        Notification: &messaging.Notification{
            Title: notification.Title,
            Body:  notification.Message,
        },
        Data: notification.Data,
        Tokens: tokens,
    }

    // Send to FCM
    response, err := s.fcm.SendMulticast(ctx, message)
    if err != nil {
        return err
    }

    // Handle failures
    if response.FailureCount > 0 {
        // Log failed tokens, potentially mark as invalid
    }

    return nil
}
```

---

## Location & Geocoding

### Distance Calculation (Haversine)

```go
// pkg/location/distance.go
func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
    const R = 6371 // Earth radius in km

    dLat := toRadians(lat2 - lat1)
    dLon := toRadians(lon2 - lon1)

    a := math.Sin(dLat/2)*math.Sin(dLat/2) +
         math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*
         math.Sin(dLon/2)*math.Sin(dLon/2)

    c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

    return R * c // Distance in km
}

func toRadians(deg float64) float64 {
    return deg * math.Pi / 180
}
```

### Reverse Geocoding

```go
// pkg/location/geocoding.go
type GeocodingService struct {
    apiKey string
    cache  *redis.Client
}

func (s *GeocodingService) ReverseGeocode(ctx context.Context, lat, lng float64) (*Address, error) {
    // Check cache
    cacheKey := fmt.Sprintf("geocode:%f:%f", lat, lng)
    if cached, err := s.cache.Get(ctx, cacheKey).Result(); err == nil {
        var addr Address
        json.Unmarshal([]byte(cached), &addr)
        return &addr, nil
    }

    // Call geocoding API (e.g., Google Maps, Mapbox)
    url := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?latlng=%f,%f&key=%s",
        lat, lng, s.apiKey)

    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result GeocodingResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    // Extract address components
    addr := s.parseAddressComponents(result)

    // Cache for 24 hours
    cached, _ := json.Marshal(addr)
    s.cache.Set(ctx, cacheKey, cached, 24*time.Hour)

    return addr, nil
}
```

---

## Testing Strategy

### Unit Tests

```go
// internal/services/user_service_test.go
func TestUserService_CreateUser(t *testing.T) {
    // Setup
    mockUserRepo := new(mocks.MockUserRepository)
    mockProfileRepo := new(mocks.MockProfileRepository)
    mockAuthService := new(mocks.MockAuthService)

    service := &UserService{
        userRepo:    mockUserRepo,
        profileRepo: mockProfileRepo,
        authService: mockAuthService,
    }

    // Test case: successful creation
    t.Run("Success", func(t *testing.T) {
        req := &CreateUserRequest{
            Email:     "test@example.com",
            Password:  "SecurePass123!",
            FirstName: "John",
            LastName:  "Doe",
        }

        mockUserRepo.On("GetByEmail", mock.Anything, req.Email).Return(nil, nil)
        mockAuthService.On("HashPassword", req.Password).Return("hashedpass", nil)
        mockUserRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
        mockProfileRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)
        mockAuthService.On("GenerateTokenPair", mock.Anything, req.Email).Return(&TokenPair{}, nil)

        user, tokens, err := service.CreateUser(context.Background(), req)

        assert.NoError(t, err)
        assert.NotNil(t, user)
        assert.NotNil(t, tokens)
        mockUserRepo.AssertExpectations(t)
    })

    // Test case: user already exists
    t.Run("UserExists", func(t *testing.T) {
        // ...
    })
}
```

### Integration Tests

```go
// internal/handlers/auth_handler_integration_test.go
func TestAuthHandler_Register_Integration(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer db.Close()

    // Setup test server
    router := setupTestRouter(db)

    // Test registration
    payload := `{"email":"test@example.com","password":"SecurePass123!"}`
    req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusCreated, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)

    assert.Contains(t, response, "access_token")
    assert.Contains(t, response, "refresh_token")
}
```

### Load Testing (k6)

```javascript
// tests/load/api_load_test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
    stages: [
        { duration: '2m', target: 100 },
        { duration: '5m', target: 100 },
        { duration: '2m', target: 200 },
        { duration: '5m', target: 200 },
        { duration: '2m', target: 0 },
    ],
};

export default function () {
    // Test feed endpoint
    let response = http.get('http://localhost:8080/api/v1/posts', {
        headers: { 'Authorization': `Bearer ${__ENV.ACCESS_TOKEN}` },
    });

    check(response, {
        'status is 200': (r) => r.status === 200,
        'response time < 200ms': (r) => r.timings.duration < 200,
    });

    sleep(1);
}
```

---

## Deployment & DevOps

### Docker Configuration

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./main"]
```

### Docker Compose (Development)

```yaml
version: '3.8'

services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_NAME=hamsaya
      - DB_USER=postgres
      - DB_PASSWORD=postgres
      - REDIS_URL=redis:6379
      - JWT_SECRET=your-secret-key
      - MINIO_ENDPOINT=minio:9000
      - MINIO_ACCESS_KEY=minioadmin
      - MINIO_SECRET_KEY=minioadmin
    depends_on:
      - postgres
      - redis
      - minio
    volumes:
      - ./:/app
    command: air # Live reload

  postgres:
    image: postgis/postgis:15-3.3
    environment:
      - POSTGRES_DB=hamsaya
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  minio:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    command: server /data --console-address ":9001"
    volumes:
      - minio_data:/data

volumes:
  postgres_data:
  redis_data:
  minio_data:
```

### Kubernetes Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hamsaya-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hamsaya-api
  template:
    metadata:
      labels:
        app: hamsaya-api
    spec:
      containers:
      - name: api
        image: hamsaya-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: host
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### CI/CD Pipeline (GitHub Actions)

```yaml
# .github/workflows/deploy.yml
name: Build and Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run tests
        run: |
          go test -v -race -coverprofile=coverage.out ./...
          go tool cover -html=coverage.out -o coverage.html

      - name: Upload coverage
        uses: codecov/codecov-action@v3

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build Docker image
        run: docker build -t hamsaya-api:${{ github.sha }} .

      - name: Push to registry
        run: |
          echo ${{ secrets.DOCKER_PASSWORD }} | docker login -u ${{ secrets.DOCKER_USERNAME }} --password-stdin
          docker push hamsaya-api:${{ github.sha }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/hamsaya-api api=hamsaya-api:${{ github.sha }}
          kubectl rollout status deployment/hamsaya-api
```

---

## Phase-by-Phase Implementation

### Phase 1: Foundation (Weeks 1-2)
- [x] Project structure setup
- [x] Database schema migration
- [x] Basic authentication (JWT)
- [x] User registration/login
- [x] Email verification
- [x] Password reset
- [x] Health check endpoints

### Phase 2: User Management (Weeks 3-4)
- [ ] Profile CRUD
- [ ] Image upload (avatar/cover)
- [ ] OAuth integration (Google/Apple/Facebook)
- [ ] MFA implementation
- [ ] Session management
- [ ] Privacy settings

### Phase 3: Post System (Weeks 5-7)
- [ ] Post CRUD (all 4 types)
- [ ] Image attachments
- [ ] Location tagging
- [ ] Like/unlike
- [ ] Bookmark/unbookmark
- [ ] Share functionality
- [ ] Feed algorithm (recent/nearby/trending)

### Phase 4: Social Interactions (Weeks 8-9)
- [ ] Comment CRUD
- [ ] Nested comments
- [ ] Comment likes
- [ ] Poll system
- [ ] Event interests
- [ ] User blocking
- [ ] Reporting system

### Phase 5: Business Features (Weeks 10-11)
- [ ] Business profile CRUD
- [ ] Business categories
- [ ] Working hours
- [ ] Business gallery
- [ ] Follow/unfollow
- [ ] Business posts

### Phase 6: Marketplace (Week 12)
- [ ] Sell post creation
- [ ] Category management
- [ ] Advanced filtering
- [ ] Mark as sold/available
- [ ] My listings

### Phase 7: Real-time Features (Weeks 13-14)
- [ ] WebSocket implementation
- [ ] Chat system
- [ ] Push notifications
- [ ] Notification settings
- [ ] Real-time updates

### Phase 8: Discovery & Search (Week 15)
- [ ] Full-text search
- [ ] Map-based discovery
- [ ] Advanced filtering
- [ ] Search optimization

### Phase 9: Testing & Optimization (Weeks 16-17)
- [ ] Unit tests (80%+ coverage)
- [ ] Integration tests
- [ ] Load testing
- [ ] Performance optimization
- [ ] Security audit

### Phase 10: Deployment (Week 18)
- [ ] Production environment setup
- [ ] CI/CD pipeline
- [ ] Monitoring & logging
- [ ] Documentation
- [ ] Migration from Supabase

---

## Migration Strategy

### From Supabase to Go Backend

1. **Dual-Write Phase** (Week 1)
   - Deploy Go backend alongside Supabase
   - Write to both systems
   - Read from Supabase (existing behavior)

2. **Gradual Read Migration** (Weeks 2-4)
   - Route 10% of reads to Go backend
   - Monitor metrics and errors
   - Increase to 50%, then 100%
   - Keep Supabase as fallback

3. **Full Migration** (Week 5)
   - All traffic to Go backend
   - Supabase in read-only mode
   - Final data sync
   - Decommission Supabase

---

## Success Metrics

- **Performance**: 95% of requests < 100ms
- **Reliability**: 99.9% uptime
- **Scalability**: Handle 10,000 concurrent users
- **Security**: Zero critical vulnerabilities
- **Test Coverage**: > 80%
- **Documentation**: 100% API documentation

---

## Conclusion

This comprehensive plan provides a complete roadmap for building a production-ready Go backend that perfectly mirrors your Flutter app's functionality. The implementation follows industry best practices, ensures security, and provides excellent performance and scalability.

**Next Steps:**
1. Review and approve this plan
2. Set up development environment
3. Begin Phase 1 implementation
4. Establish CI/CD pipeline
5. Deploy to staging environment
6. Execute migration strategy

**Estimated Timeline:** 18 weeks to full production deployment
**Team Size:** 2-3 backend developers + 1 DevOps engineer
**Budget Considerations:** Infrastructure costs (AWS/DigitalOcean), third-party APIs (geocoding, push notifications)
