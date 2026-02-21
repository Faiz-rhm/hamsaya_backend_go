-- Backfill data.business_id on notifications for posts that belong to a business,
-- so those notifications only appear in business-scoped notification list.
UPDATE notifications n
SET data = jsonb_set(COALESCE(n.data, '{}'::jsonb), '{business_id}', to_jsonb(p.business_id::text))
FROM posts p
WHERE (n.data->>'post_id')::uuid = p.id
  AND p.business_id IS NOT NULL
  AND (n.data->>'business_id' IS NULL OR n.data->>'business_id' = '');
