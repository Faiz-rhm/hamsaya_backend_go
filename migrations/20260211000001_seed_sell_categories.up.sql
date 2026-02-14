-- Seed sell_categories with default marketplace categories
-- Uses ON CONFLICT DO NOTHING so migration is idempotent

INSERT INTO sell_categories (id, name, icon, color, status, created_at) VALUES
  ('a1000001-0000-4000-8000-000000000001', 'All categories', '{"name":"gamepadCircle","library":"mdi"}'::jsonb, '2983A3', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000002', 'Appliance', '{"name":"washingMachine","library":"mdi"}'::jsonb, '2983A3', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000003', 'Automotive', '{"name":"car","library":"mdi"}'::jsonb, '17A69D', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000004', 'Baby & kids', '{"name":"teddyBear","library":"mdi"}'::jsonb, 'F29340', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000005', 'Bicycles', '{"name":"bicycle","library":"mdi"}'::jsonb, '43B4AD', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000006', 'Clothing & accessories', '{"name":"tshirtCrew","library":"mdi"}'::jsonb, 'F5BE2D', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000007', 'Electronics', '{"name":"televisionClassic","library":"mdi"}'::jsonb, 'F19240', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000008', 'Furniture', '{"name":"sofa","library":"mdi"}'::jsonb, 'FC5A33', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000009', 'Garden', '{"name":"palmTree","library":"mdi"}'::jsonb, '83CD29', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000a', 'Home decor', '{"name":"lamp","library":"mdi"}'::jsonb, '2B7FA2', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000b', 'Home sales', '{"name":"homeVariant","library":"mdi"}'::jsonb, '02A338', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000c', 'Musical instrument', '{"name":"guitarAcoustic","library":"mdi"}'::jsonb, 'EB87AD', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000d', 'Neighbor made', '{"name":"sword","library":"mdi"}'::jsonb, '297FA8', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000e', 'Neighbor services', '{"name":"account","library":"mdi"}'::jsonb, 'F49140', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000f', 'Other', '{"name":"silverwareForkKnife","library":"mdi"}'::jsonb, 'F95A37', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000010', 'Pet supplies', '{"name":"paw","library":"mdi"}'::jsonb, '00A539', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000011', 'Property rent', '{"name":"key","library":"mdi"}'::jsonb, 'EDC22B', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000012', 'Sports & outdoors', '{"name":"racquetball","library":"mdi"}'::jsonb, '8BCC3A', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000013', 'Tools', '{"name":"hammer","library":"mdi"}'::jsonb, 'E46497', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000014', 'Toys & games', '{"name":"controller","library":"mdi"}'::jsonb, 'FC5D2E', 'ACTIVE', NOW())
ON CONFLICT (id) DO NOTHING;
