-- migrate_r2_urls.sql
--
-- One-shot rewrite of stored media URLs after the MinIO → Cloudflare R2
-- migration. Two fixes happen in a single REPLACE pass:
--
--   1. Host swap. The R2 bucket is now served via the bound custom domain
--      `cdn.hamsaya.af`. URLs minted before the cutover point at either the
--      legacy MinIO host (`http://178.105.131.54:9000`) or the bucket-scoped
--      r2.dev preview hostname (`pub-d33f10cc...r2.dev`). Both are rewritten
--      to `https://cdn.hamsaya.af`.
--
--   2. Bucket prefix strip. URLs minted with STORAGE_PATH_STYLE=true contain
--      `/hamsaya-media/` (the bucket name) right after the host. R2's
--      bucket-scoped domains serve the bucket at root, so the prefix points
--      at a key that doesn't exist. This script removes the prefix wherever
--      it follows the new host.
--
-- Run once on the production DB:
--
--   docker exec -i hamsaya-postgres-prod psql -U postgres -d hamsaya \
--     < scripts/migrate_r2_urls.sql
--
-- The script is idempotent — running twice on a clean DB is a no-op (the
-- WHERE clauses gate every UPDATE).
--
-- BACKUP FIRST. Take a pg_dump (or use Dokploy's Backups tab) before
-- running. Although the SQL is conservative, JSONB rewrites on a live
-- column are difficult to reverse without a snapshot.

BEGIN;

-- ---------------------------------------------------------------------------
-- attachments.photo (JSONB) — post media
-- ---------------------------------------------------------------------------
UPDATE attachments
SET photo = jsonb_set(
  jsonb_set(
    jsonb_set(
      photo,
      '{url}', to_jsonb(
        REPLACE(
          REPLACE(
            REPLACE(photo->>'url',
              'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
            ),
            'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
          ),
          'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
        )
      )
    ),
    '{thumb_url}', to_jsonb(
      REPLACE(
        REPLACE(
          REPLACE(COALESCE(photo->>'thumb_url', ''),
            'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
          ),
          'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
        ),
        'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
      )
    )
  ),
  '{medium_url}', to_jsonb(
    REPLACE(
      REPLACE(
        REPLACE(COALESCE(photo->>'medium_url', ''),
          'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
        ),
        'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
      ),
      'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
    )
  )
)
WHERE photo->>'url'        LIKE '%178.105.131.54%'
   OR photo->>'url'        LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
   OR photo->>'url'        LIKE '%cdn.hamsaya.af/hamsaya-media/%'
   OR photo->>'thumb_url'  LIKE '%178.105.131.54%'
   OR photo->>'thumb_url'  LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
   OR photo->>'thumb_url'  LIKE '%cdn.hamsaya.af/hamsaya-media/%'
   OR photo->>'medium_url' LIKE '%178.105.131.54%'
   OR photo->>'medium_url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
   OR photo->>'medium_url' LIKE '%cdn.hamsaya.af/hamsaya-media/%';

-- ---------------------------------------------------------------------------
-- users.avatar (JSONB)
-- ---------------------------------------------------------------------------
UPDATE users
SET avatar = jsonb_set(avatar, '{url}', to_jsonb(
  REPLACE(
    REPLACE(
      REPLACE(avatar->>'url',
        'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
      ),
      'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
    ),
    'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
  )
))
WHERE avatar IS NOT NULL
  AND (
    avatar->>'url' LIKE '%178.105.131.54%'
    OR avatar->>'url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
    OR avatar->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%'
  );

-- ---------------------------------------------------------------------------
-- business_profiles.avatar (JSONB)
-- ---------------------------------------------------------------------------
UPDATE business_profiles
SET avatar = jsonb_set(avatar, '{url}', to_jsonb(
  REPLACE(
    REPLACE(
      REPLACE(avatar->>'url',
        'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
      ),
      'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
    ),
    'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
  )
))
WHERE avatar IS NOT NULL
  AND (
    avatar->>'url' LIKE '%178.105.131.54%'
    OR avatar->>'url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
    OR avatar->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%'
  );

-- ---------------------------------------------------------------------------
-- users.cover (JSONB) — optional profile cover image
-- ---------------------------------------------------------------------------
UPDATE users
SET cover = jsonb_set(cover, '{url}', to_jsonb(
  REPLACE(
    REPLACE(
      REPLACE(cover->>'url',
        'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
      ),
      'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
    ),
    'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
  )
))
WHERE cover IS NOT NULL
  AND (
    cover->>'url' LIKE '%178.105.131.54%'
    OR cover->>'url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
    OR cover->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%'
  );

-- ---------------------------------------------------------------------------
-- comment_attachments.photo (JSONB) — images attached to comments
-- ---------------------------------------------------------------------------
UPDATE comment_attachments
SET photo = jsonb_set(photo, '{url}', to_jsonb(
  REPLACE(
    REPLACE(
      REPLACE(photo->>'url',
        'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
      ),
      'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
    ),
    'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
  )
))
WHERE photo->>'url' LIKE '%178.105.131.54%'
   OR photo->>'url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
   OR photo->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%';

-- ---------------------------------------------------------------------------
-- business_attachments.photo (JSONB) — gallery images on business profiles
-- ---------------------------------------------------------------------------
UPDATE business_attachments
SET photo = jsonb_set(photo, '{url}', to_jsonb(
  REPLACE(
    REPLACE(
      REPLACE(photo->>'url',
        'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
      ),
      'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
    ),
    'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
  )
))
WHERE photo->>'url' LIKE '%178.105.131.54%'
   OR photo->>'url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
   OR photo->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%';

-- ---------------------------------------------------------------------------
-- ads.image_url (TEXT) — sponsored ad creative
-- ---------------------------------------------------------------------------
UPDATE ads
SET image_url = REPLACE(
  REPLACE(
    REPLACE(image_url,
      'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
    ),
    'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
  ),
  'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
)
WHERE image_url IS NOT NULL
  AND (
    image_url LIKE '%178.105.131.54%'
    OR image_url LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
    OR image_url LIKE '%cdn.hamsaya.af/hamsaya-media/%'
  );

-- ---------------------------------------------------------------------------
-- business_profiles.cover (JSONB)
-- ---------------------------------------------------------------------------
UPDATE business_profiles
SET cover = jsonb_set(cover, '{url}', to_jsonb(
  REPLACE(
    REPLACE(
      REPLACE(cover->>'url',
        'http://178.105.131.54:9000/', 'https://cdn.hamsaya.af/'
      ),
      'https://pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev/', 'https://cdn.hamsaya.af/'
    ),
    'https://cdn.hamsaya.af/hamsaya-media/', 'https://cdn.hamsaya.af/'
  )
))
WHERE cover IS NOT NULL
  AND (
    cover->>'url' LIKE '%178.105.131.54%'
    OR cover->>'url' LIKE '%pub-d33f10ccba6547798cfc3fa6cee65f84.r2.dev%'
    OR cover->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%'
  );

-- ---------------------------------------------------------------------------
-- Verification (informational — does not block the commit)
-- ---------------------------------------------------------------------------
SELECT 'attachments.photo'                                    AS scope,
       count(*) FILTER (WHERE photo->>'url' LIKE '%178.105.131.54%' OR photo->>'url' LIKE '%pub-d33f10cc%' OR photo->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%') AS legacy_remaining
FROM attachments
UNION ALL
SELECT 'users.avatar',
       count(*) FILTER (WHERE avatar->>'url' LIKE '%178.105.131.54%' OR avatar->>'url' LIKE '%pub-d33f10cc%' OR avatar->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM users
UNION ALL
SELECT 'users.cover',
       count(*) FILTER (WHERE cover->>'url' LIKE '%178.105.131.54%' OR cover->>'url' LIKE '%pub-d33f10cc%' OR cover->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM users
UNION ALL
SELECT 'business_profiles.avatar',
       count(*) FILTER (WHERE avatar->>'url' LIKE '%178.105.131.54%' OR avatar->>'url' LIKE '%pub-d33f10cc%' OR avatar->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM business_profiles
UNION ALL
SELECT 'business_profiles.cover',
       count(*) FILTER (WHERE cover->>'url' LIKE '%178.105.131.54%' OR cover->>'url' LIKE '%pub-d33f10cc%' OR cover->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM business_profiles
UNION ALL
SELECT 'comment_attachments.photo',
       count(*) FILTER (WHERE photo->>'url' LIKE '%178.105.131.54%' OR photo->>'url' LIKE '%pub-d33f10cc%' OR photo->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM comment_attachments
UNION ALL
SELECT 'business_attachments.photo',
       count(*) FILTER (WHERE photo->>'url' LIKE '%178.105.131.54%' OR photo->>'url' LIKE '%pub-d33f10cc%' OR photo->>'url' LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM business_attachments
UNION ALL
SELECT 'ads.image_url',
       count(*) FILTER (WHERE image_url LIKE '%178.105.131.54%' OR image_url LIKE '%pub-d33f10cc%' OR image_url LIKE '%cdn.hamsaya.af/hamsaya-media/%')
FROM ads;

COMMIT;
