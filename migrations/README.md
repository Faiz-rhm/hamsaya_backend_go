# Database Migrations

**Last Updated:** 2025-01-24

## Overview

This directory contains SQL migrations to optimize the Hamsaya database schema for production deployment.

## New Migrations (2025-01-24)

### Performance & Security Enhancements

1. **20250124120000_add_is_active_indexes** - Partial indexes on is_active columns
2. **20250124120100_add_composite_indexes** - Composite indexes for common queries
3. **20250124120200_add_post_type_constraints** - Type-specific validation (EVENT, SELL, PULL)
4. **20250124120300_add_unique_constraints** - Unique constraints and data validation
5. **20250124120400_optimize_postgis** - PostGIS spatial query optimization
6. **20250124120500_fix_conversations** - Conversation ordering and unread counts
7. **20250124120600_add_foreign_key_constraints** - Proper CASCADE rules

## Running Migrations

### Using Make (Recommended)

```bash
# Run all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check migration status
make migrate-status

# Force version (if stuck)
make migrate-force VERSION=20250124120000
```

### Manual Execution

```bash
# Using golang-migrate CLI
migrate -path migrations \
  -database "postgresql://user:pass@localhost:5432/hamsaya?sslmode=disable" \
  up

# Using psql directly
psql -U postgres -d hamsaya \
  -f migrations/20250124120000_add_is_active_indexes.up.sql
```

## Migration Order

**IMPORTANT:** Execute migrations in sequence (they have dependencies):

1. **Indexes first** (migrations 1-2) - Can run CONCURRENTLY without blocking
2. **Constraints** (migrations 3-4) - May fail if existing data violates constraints
3. **PostGIS optimization** (migration 5)
4. **Conversation fixes** (migration 6)
5. **Foreign keys** (migration 7)

## Pre-Migration Checklist

Before running migrations in production:

- [ ] Backup database
- [ ] Check for constraint violations (see below)
- [ ] Verify no long-running transactions
- [ ] Monitor database performance during execution
- [ ] Test rollback on staging first

## Checking for Constraint Violations

Run these queries **before** applying migrations to identify data that would violate new constraints:

```sql
-- Check EVENT posts missing required fields
SELECT id, title, start_date, start_time
FROM posts
WHERE type = 'EVENT'
  AND (title IS NULL OR title = '' OR start_date IS NULL OR start_time IS NULL);

-- Check SELL posts missing required fields
SELECT id, title, price, category
FROM posts
WHERE type = 'SELL'
  AND (title IS NULL OR title = '' OR price IS NULL OR price <= 0 OR category IS NULL);

-- Check duplicate emails
SELECT LOWER(email), COUNT(*)
FROM users
GROUP BY LOWER(email)
HAVING COUNT(*) > 1;

-- Check duplicate phones
SELECT LOWER(phone), COUNT(*)
FROM users
WHERE phone IS NOT NULL AND phone != ''
GROUP BY LOWER(phone)
HAVING COUNT(*) > 1;

-- Check invalid email formats
SELECT id, email
FROM users
WHERE email !~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$';

-- Check users under 13
SELECT p.id, p.date_of_birth, AGE(p.date_of_birth)
FROM profiles p
WHERE p.date_of_birth > CURRENT_DATE - INTERVAL '13 years';
```

## Expected Duration

**Total: ~4-5 minutes**

- Migration 1: ~30 seconds (CONCURRENTLY indexes)
- Migration 2: ~1-2 minutes (multiple CONCURRENTLY indexes)
- Migration 3: ~10 seconds (constraints are fast)
- Migration 4: ~5 seconds (validation rules)
- Migration 5: ~30 seconds (PostGIS functions and indexes)
- Migration 6: ~15 seconds (conversation fixes)
- Migration 7: ~20 seconds (foreign key constraints)

## Performance Impact

- **CONCURRENTLY indexes**: No table locks, safe for production
- **CHECK constraints**: Validated on existing data, may take time
- **UNIQUE indexes**: Must scan entire table to verify uniqueness
- **PostGIS functions**: No impact, created instantly
- **Triggers**: Minimal overhead, only fire on INSERT/UPDATE

## Features Added

### 1. Is Active Indexes
- Partial indexes on `is_active` columns for faster filtering
- Only indexes active records (smaller index size)
- Composite indexes for common query patterns

### 2. Composite Indexes
- Prevents duplicate likes/bookmarks/follows
- Optimizes comment threading queries
- Speeds up notification feed queries
- Improves chat message retrieval

### 3. Post Type Constraints
- EVENT posts require: title, start_date, start_time
- SELL posts require: title, price > 0, category
- PULL posts require: title
- Validates post types and visibility values

### 4. Unique Constraints
- Case-insensitive unique email/phone
- One active TOTP factor per user
- Unique business category names
- Email format validation
- Minimum age validation (13 years)

### 5. PostGIS Optimization
- Spatial indexes for location-based queries
- Helper functions: `calculate_distance_km()`
- Query functions: `find_nearby_posts()`, `find_nearby_businesses()`
- Optimized for radius searches

### 6. Conversation Fixes
- `last_message_at` column for proper ordering
- `unread_count` tracking per participant
- Automatic triggers to update counts
- `mark_conversation_read()` function

### 7. Foreign Key Constraints
- Proper CASCADE DELETE rules
- Prevents orphaned records
- Ensures referential integrity

## Rollback Strategy

Each migration has a `.down.sql` file for rollback:

```bash
# Rollback last migration
make migrate-down

# Rollback to specific version
migrate -path migrations \
  -database "..." \
  goto 20250124120000
```

**Warning:** Rolling back foreign key constraints (migration 7) is not recommended as it may cause data integrity issues.

## Monitoring During Migration

Watch for:
- Lock contention during constraint creation
- Long-running queries blocking migrations
- Disk space for new indexes (~10-20% increase)
- CPU usage during index builds

## Troubleshooting

### Migration Hangs

```sql
-- Check for blocking queries
SELECT pid, usename, state, query, query_start
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY query_start;

-- Kill blocking query (if necessary)
SELECT pg_terminate_backend(pid);
```

### Constraint Violation

```sql
-- Fix data before re-running migration
UPDATE posts
SET title = 'Untitled Event'
WHERE type = 'EVENT' AND (title IS NULL OR title = '');

UPDATE posts
SET title = 'Untitled Item', price = 0.01
WHERE type = 'SELL' AND (title IS NULL OR title = '' OR price IS NULL);
```

### Out of Disk Space

```sql
-- Check index sizes
SELECT
  schemaname,
  tablename,
  indexname,
  pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes
ORDER BY pg_relation_size(indexrelid) DESC;
```

## Production Deployment

### Step-by-Step Process

1. **Backup Database**
   ```bash
   pg_dump -U postgres -d hamsaya -F c -f backup_$(date +%Y%m%d).dump
   ```

2. **Test on Staging**
   ```bash
   # Restore production data to staging
   pg_restore -U postgres -d hamsaya_staging backup.dump

   # Run migrations
   make migrate-up

   # Verify no errors
   ```

3. **Check for Violations**
   ```bash
   # Run all validation queries above
   psql -U postgres -d hamsaya -f scripts/check_violations.sql
   ```

4. **Run on Production**
   ```bash
   # During low-traffic window
   make migrate-up

   # Monitor logs
   tail -f /var/log/postgresql/postgresql-*.log
   ```

5. **Verify Success**
   ```bash
   # Check migration version
   make migrate-status

   # Verify indexes created
   psql -U postgres -d hamsaya -c "\di+"

   # Check application health
   curl https://api.hamsaya.com/health
   ```

## Contact

For migration issues, contact the backend team or database administrator.
