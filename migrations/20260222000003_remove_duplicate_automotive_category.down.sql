-- Re-insert the duplicate Automotive category (down only restores the row; posts stay on canonical).
INSERT INTO sell_categories (id, name, name_dari, name_pashto, icon, color, status, created_at)
VALUES (
  '19d26fe9-b5ba-4a67-a91c-8c58b7cd8043',
  'Automotive',
  N'موتر',
  N'موټر',
  '{"name":"directions_car","library":"MaterialIcons"}'::jsonb,
  '#EF4444',
  'ACTIVE',
  '0001-01-01 04:36:48+04:36'
)
ON CONFLICT (id) DO NOTHING;
