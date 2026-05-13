package seedsql

// seedSellCategories inserts default marketplace categories (same as migration 20260211000001).
var SellCategoriesSQL = `
INSERT INTO sell_categories (id, name, icon, color, status, created_at) VALUES
  ('a1000001-0000-4000-8000-000000000001', 'All categories', '{"name":"gamepadCircle","library":"mdi"}'::jsonb, '2983A3', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000002', 'Appliance', '{"name":"washingMachine","library":"mdi"}'::jsonb, '2983A3', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000003', 'Automotive', '{"name":"car","library":"mdi"}'::jsonb, '17A69D', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000004', 'Baby & kids', '{"name":"teddyBear","library":"mdi"}'::jsonb, 'F29340', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000005', 'Bags & Luggage', '{"name":"bagSuitcase","library":"mdi"}'::jsonb, '9C27B0', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000006', 'Bicycles', '{"name":"bicycle","library":"mdi"}'::jsonb, '43B4AD', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000007', 'Books & Media', '{"name":"bookOpenVariant","library":"mdi"}'::jsonb, '6D4C41', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000008', 'Clothing & accessories', '{"name":"tshirtCrew","library":"mdi"}'::jsonb, 'F5BE2D', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000009', 'Electronics', '{"name":"televisionClassic","library":"mdi"}'::jsonb, 'F19240', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000a', 'Furniture', '{"name":"sofa","library":"mdi"}'::jsonb, 'FC5A33', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000b', 'Garden', '{"name":"palmTree","library":"mdi"}'::jsonb, '83CD29', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000c', 'Health & Beauty', '{"name":"lipstick","library":"mdi"}'::jsonb, 'E91E63', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000d', 'Home decor', '{"name":"lamp","library":"mdi"}'::jsonb, '2B7FA2', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000e', 'Home sales', '{"name":"homeVariant","library":"mdi"}'::jsonb, '02A338', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000000f', 'Jewelry & Watches', '{"name":"watch","library":"mdi"}'::jsonb, 'FFD700', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000010', 'Kitchen & Dining', '{"name":"pot","library":"mdi"}'::jsonb, 'FF7043', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000011', 'Motorcycles', '{"name":"motorbike","library":"mdi"}'::jsonb, '455A64', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000012', 'Musical instrument', '{"name":"guitarAcoustic","library":"mdi"}'::jsonb, 'EB87AD', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000013', 'Neighbor made', '{"name":"sword","library":"mdi"}'::jsonb, '297FA8', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000014', 'Neighbor services', '{"name":"account","library":"mdi"}'::jsonb, 'F49140', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000015', 'Other', '{"name":"silverwareForkKnife","library":"mdi"}'::jsonb, 'F95A37', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000016', 'Pet supplies', '{"name":"paw","library":"mdi"}'::jsonb, '00A539', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000017', 'Property rent', '{"name":"key","library":"mdi"}'::jsonb, 'EDC22B', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000018', 'Sports & outdoors', '{"name":"racquetball","library":"mdi"}'::jsonb, '8BCC3A', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-000000000019', 'Tools', '{"name":"hammer","library":"mdi"}'::jsonb, 'E46497', 'ACTIVE', NOW()),
  ('a1000001-0000-4000-8000-00000000001a', 'Toys & games', '{"name":"controller","library":"mdi"}'::jsonb, 'FC5D2E', 'ACTIVE', NOW())
ON CONFLICT (id) DO NOTHING;
`

// seedBusinessCategories inserts default business categories.
var BusinessCategoriesSQL = `
INSERT INTO business_categories (id, name, is_active, created_at) VALUES
  -- Business Types
  ('b2000001-0000-4000-8000-000000000001', 'Retail', true, NOW()),
  ('b2000001-0000-4000-8000-000000000002', 'Restaurant', true, NOW()),
  ('b2000001-0000-4000-8000-000000000003', 'Cafe & Coffee', true, NOW()),
  ('b2000001-0000-4000-8000-000000000004', 'Food & Beverage', true, NOW()),
  ('b2000001-0000-4000-8000-000000000005', 'Health & Beauty', true, NOW()),
  ('b2000001-0000-4000-8000-000000000006', 'Automotive', true, NOW()),
  ('b2000001-0000-4000-8000-000000000007', 'Education', true, NOW()),
  ('b2000001-0000-4000-8000-000000000008', 'Entertainment', true, NOW()),
  ('b2000001-0000-4000-8000-000000000009', 'Hospitality', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000a', 'Professional Services', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000b', 'Real Estate', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000c', 'Technology', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000d', 'Bakery', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000e', 'Bar & Pub', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000f', 'Grocery & Supermarket', true, NOW()),
  ('b2000001-0000-4000-8000-000000000010', 'Pharmacy', true, NOW()),
  ('b2000001-0000-4000-8000-000000000011', 'Clothing & Fashion', true, NOW()),
  ('b2000001-0000-4000-8000-000000000012', 'Electronics Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000013', 'Furniture Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000014', 'Gym & Fitness', true, NOW()),
  ('b2000001-0000-4000-8000-000000000015', 'Salon & Barber', true, NOW()),
  ('b2000001-0000-4000-8000-000000000016', 'Repair & Maintenance', true, NOW()),
  ('b2000001-0000-4000-8000-000000000017', 'Construction', true, NOW()),
  ('b2000001-0000-4000-8000-000000000018', 'Travel & Tourism', true, NOW()),
  ('b2000001-0000-4000-8000-000000000019', 'Event Planning', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001a', 'Photography', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001b', 'Printing & Copy', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001c', 'Cleaning Services', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001d', 'Insurance', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001e', 'Logistics & Delivery', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001f', 'Manufacturing', true, NOW()),
  ('b2000001-0000-4000-8000-000000000020', 'Agriculture', true, NOW()),
  ('b2000001-0000-4000-8000-000000000021', 'Import & Export', true, NOW()),
  ('b2000001-0000-4000-8000-000000000022', 'Wholesale', true, NOW()),
  ('b2000001-0000-4000-8000-000000000023', 'Pet Services', true, NOW()),
  ('b2000001-0000-4000-8000-000000000024', 'Jewelry Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000025', 'Hardware Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000026', 'Bookstore', true, NOW()),
  ('b2000001-0000-4000-8000-000000000027', 'Florist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000028', 'Sports & Recreation', true, NOW()),
  ('b2000001-0000-4000-8000-000000000029', 'Art & Craft', true, NOW()),
  ('b2000001-0000-4000-8000-00000000002a', 'Music & Instruments', true, NOW()),
  
  -- Healthcare Professions
  ('b2000001-0000-4000-8000-000000000030', 'Doctor', true, NOW()),
  ('b2000001-0000-4000-8000-000000000031', 'Dentist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000032', 'Nurse', true, NOW()),
  ('b2000001-0000-4000-8000-000000000033', 'Pharmacist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000034', 'Physiotherapist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000035', 'Psychologist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000036', 'Veterinarian', true, NOW()),
  ('b2000001-0000-4000-8000-000000000037', 'Optometrist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000038', 'Nutritionist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000039', 'Midwife', true, NOW()),
  ('b2000001-0000-4000-8000-00000000003a', 'Paramedic', true, NOW()),
  ('b2000001-0000-4000-8000-00000000003b', 'Lab Technician', true, NOW()),
  
  -- Legal & Finance Professions
  ('b2000001-0000-4000-8000-000000000040', 'Lawyer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000041', 'Accountant', true, NOW()),
  ('b2000001-0000-4000-8000-000000000042', 'Auditor', true, NOW()),
  ('b2000001-0000-4000-8000-000000000043', 'Tax Consultant', true, NOW()),
  ('b2000001-0000-4000-8000-000000000044', 'Financial Advisor', true, NOW()),
  ('b2000001-0000-4000-8000-000000000045', 'Notary', true, NOW()),
  ('b2000001-0000-4000-8000-000000000046', 'Banker', true, NOW()),
  
  -- Engineering & Technical
  ('b2000001-0000-4000-8000-000000000050', 'Civil Engineer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000051', 'Mechanical Engineer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000052', 'Electrical Engineer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000053', 'Software Engineer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000054', 'Architect', true, NOW()),
  ('b2000001-0000-4000-8000-000000000055', 'Surveyor', true, NOW()),
  ('b2000001-0000-4000-8000-000000000056', 'IT Specialist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000057', 'Network Engineer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000058', 'Data Analyst', true, NOW()),
  
  -- Creative & Design
  ('b2000001-0000-4000-8000-000000000060', 'Graphic Designer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000061', 'Interior Designer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000062', 'Web Designer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000063', 'Fashion Designer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000064', 'Photographer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000065', 'Videographer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000066', 'Writer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000067', 'Journalist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000068', 'Translator', true, NOW()),
  ('b2000001-0000-4000-8000-000000000069', 'Artist', true, NOW()),
  ('b2000001-0000-4000-8000-00000000006a', 'Musician', true, NOW()),
  
  -- Education
  ('b2000001-0000-4000-8000-000000000070', 'Teacher', true, NOW()),
  ('b2000001-0000-4000-8000-000000000071', 'Tutor', true, NOW()),
  ('b2000001-0000-4000-8000-000000000072', 'Professor', true, NOW()),
  ('b2000001-0000-4000-8000-000000000073', 'Trainer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000074', 'Coach', true, NOW()),
  ('b2000001-0000-4000-8000-000000000075', 'Driving Instructor', true, NOW()),
  
  -- Trades & Skilled Labor
  ('b2000001-0000-4000-8000-000000000080', 'Electrician', true, NOW()),
  ('b2000001-0000-4000-8000-000000000081', 'Plumber', true, NOW()),
  ('b2000001-0000-4000-8000-000000000082', 'Carpenter', true, NOW()),
  ('b2000001-0000-4000-8000-000000000083', 'Mason', true, NOW()),
  ('b2000001-0000-4000-8000-000000000084', 'Welder', true, NOW()),
  ('b2000001-0000-4000-8000-000000000085', 'Painter', true, NOW()),
  ('b2000001-0000-4000-8000-000000000086', 'Tiler', true, NOW()),
  ('b2000001-0000-4000-8000-000000000087', 'Roofer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000088', 'Blacksmith', true, NOW()),
  ('b2000001-0000-4000-8000-000000000089', 'Glazier', true, NOW()),
  ('b2000001-0000-4000-8000-00000000008a', 'HVAC Technician', true, NOW()),
  ('b2000001-0000-4000-8000-00000000008b', 'Auto Mechanic', true, NOW()),
  ('b2000001-0000-4000-8000-00000000008c', 'Tailor', true, NOW()),
  ('b2000001-0000-4000-8000-00000000008d', 'Cobbler', true, NOW()),
  ('b2000001-0000-4000-8000-00000000008e', 'Locksmith', true, NOW()),
  ('b2000001-0000-4000-8000-00000000008f', 'Watchmaker', true, NOW()),
  
  -- Home & Personal Services
  ('b2000001-0000-4000-8000-000000000090', 'Gardener', true, NOW()),
  ('b2000001-0000-4000-8000-000000000091', 'Handyman', true, NOW()),
  ('b2000001-0000-4000-8000-000000000092', 'Cleaner', true, NOW()),
  ('b2000001-0000-4000-8000-000000000093', 'Housekeeper', true, NOW()),
  ('b2000001-0000-4000-8000-000000000094', 'Nanny', true, NOW()),
  ('b2000001-0000-4000-8000-000000000095', 'Caregiver', true, NOW()),
  ('b2000001-0000-4000-8000-000000000096', 'Cook', true, NOW()),
  ('b2000001-0000-4000-8000-000000000097', 'Chef', true, NOW()),
  ('b2000001-0000-4000-8000-000000000098', 'Caterer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000099', 'Hairdresser', true, NOW()),
  ('b2000001-0000-4000-8000-00000000009a', 'Barber', true, NOW()),
  ('b2000001-0000-4000-8000-00000000009b', 'Beautician', true, NOW()),
  ('b2000001-0000-4000-8000-00000000009c', 'Makeup Artist', true, NOW()),
  ('b2000001-0000-4000-8000-00000000009d', 'Massage Therapist', true, NOW()),
  ('b2000001-0000-4000-8000-00000000009e', 'Personal Trainer', true, NOW()),
  
  -- Transport & Logistics
  ('b2000001-0000-4000-8000-0000000000a0', 'Driver', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000a1', 'Taxi Driver', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000a2', 'Truck Driver', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000a3', 'Courier', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000a4', 'Mover', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000a5', 'Pilot', true, NOW()),
  
  -- Security & Protection
  ('b2000001-0000-4000-8000-0000000000b0', 'Security Guard', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000b1', 'Bodyguard', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000b2', 'Private Investigator', true, NOW()),
  
  -- Marketing & Sales
  ('b2000001-0000-4000-8000-0000000000c0', 'Marketer', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000c1', 'Sales Representative', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000c2', 'Real Estate Agent', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000c3', 'Insurance Agent', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000c4', 'Social Media Manager', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000c5', 'SEO Specialist', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000c6', 'Content Creator', true, NOW()),
  
  -- Consulting & Management
  ('b2000001-0000-4000-8000-0000000000d0', 'Business Consultant', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000d1', 'HR Consultant', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000d2', 'Project Manager', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000d3', 'Event Manager', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000d4', 'Property Manager', true, NOW()),
  
  -- Religious & Community
  ('b2000001-0000-4000-8000-0000000000e0', 'Imam', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000e1', 'Mullah', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000e2', 'Religious Teacher', true, NOW()),
  ('b2000001-0000-4000-8000-0000000000e3', 'Social Worker', true, NOW()),
  
  -- Retail Specialty
  ('b2000001-0000-4000-8000-000000000100', 'Mobile Phone Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000101', 'Mobile Phone Repair', true, NOW()),
  ('b2000001-0000-4000-8000-000000000102', 'Computer Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000103', 'Computer Repair', true, NOW()),
  ('b2000001-0000-4000-8000-000000000104', 'Internet Cafe', true, NOW()),
  ('b2000001-0000-4000-8000-000000000105', 'Stationery Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000106', 'Toy Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000107', 'Gift Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000108', 'Optical & Eyewear Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000109', 'Carpet Store', true, NOW()),
  ('b2000001-0000-4000-8000-00000000010a', 'Curtain Shop', true, NOW()),
  ('b2000001-0000-4000-8000-00000000010b', 'Kitchenware Store', true, NOW()),
  ('b2000001-0000-4000-8000-00000000010c', 'Mattress Shop', true, NOW()),
  ('b2000001-0000-4000-8000-00000000010d', 'Antique Shop', true, NOW()),

  -- Food Specialty
  ('b2000001-0000-4000-8000-000000000110', 'Butcher Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000111', 'Fish Market', true, NOW()),
  ('b2000001-0000-4000-8000-000000000112', 'Sweets Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000113', 'Ice Cream Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000114', 'Juice Bar', true, NOW()),
  ('b2000001-0000-4000-8000-000000000115', 'Tea House', true, NOW()),
  ('b2000001-0000-4000-8000-000000000116', 'Spice & Dry Fruit Shop', true, NOW()),

  -- Auto Specialty
  ('b2000001-0000-4000-8000-000000000120', 'Car Wash', true, NOW()),
  ('b2000001-0000-4000-8000-000000000121', 'Tire Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000122', 'Gas Station', true, NOW()),
  ('b2000001-0000-4000-8000-000000000123', 'Auto Spare Parts', true, NOW()),
  ('b2000001-0000-4000-8000-000000000124', 'Battery Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000125', 'Motorcycle Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000126', 'Bicycle Shop', true, NOW()),

  -- Building & Construction Supplies
  ('b2000001-0000-4000-8000-000000000130', 'Cement & Building Materials', true, NOW()),
  ('b2000001-0000-4000-8000-000000000131', 'Paint Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000132', 'Steel & Iron Supplier', true, NOW()),
  ('b2000001-0000-4000-8000-000000000133', 'Tile Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000134', 'Sanitary Ware Shop', true, NOW()),
  ('b2000001-0000-4000-8000-000000000135', 'Solar Installer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000136', 'Generator Repair', true, NOW()),

  -- Money & Finance Extras
  ('b2000001-0000-4000-8000-000000000140', 'Money Exchange', true, NOW()),
  ('b2000001-0000-4000-8000-000000000141', 'Hawala / Money Transfer', true, NOW()),

  -- Travel & Religious
  ('b2000001-0000-4000-8000-000000000150', 'Hajj & Umrah Agency', true, NOW()),
  ('b2000001-0000-4000-8000-000000000151', 'Wedding Hall', true, NOW()),
  ('b2000001-0000-4000-8000-000000000152', 'Funeral Services', true, NOW()),

  -- Afghan Crafts
  ('b2000001-0000-4000-8000-000000000160', 'Carpet Weaver', true, NOW()),
  ('b2000001-0000-4000-8000-000000000161', 'Embroidery', true, NOW()),
  ('b2000001-0000-4000-8000-000000000162', 'Pottery', true, NOW()),
  ('b2000001-0000-4000-8000-000000000163', 'Calligrapher', true, NOW()),

  -- Agriculture Sub-Categories
  ('b2000001-0000-4000-8000-000000000170', 'Farmer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000171', 'Livestock & Cattle', true, NOW()),
  ('b2000001-0000-4000-8000-000000000172', 'Poultry Farm', true, NOW()),
  ('b2000001-0000-4000-8000-000000000173', 'Beekeeper', true, NOW()),
  ('b2000001-0000-4000-8000-000000000174', 'Dairy', true, NOW()),
  ('b2000001-0000-4000-8000-000000000175', 'Fishery', true, NOW()),

  -- Education Extras
  ('b2000001-0000-4000-8000-000000000180', 'Daycare', true, NOW()),
  ('b2000001-0000-4000-8000-000000000181', 'Kindergarten', true, NOW()),
  ('b2000001-0000-4000-8000-000000000182', 'Tuition Center', true, NOW()),
  ('b2000001-0000-4000-8000-000000000183', 'Driving School', true, NOW()),
  ('b2000001-0000-4000-8000-000000000184', 'Music School', true, NOW()),
  ('b2000001-0000-4000-8000-000000000185', 'Language School', true, NOW()),

  -- Beauty & Wellness Extras
  ('b2000001-0000-4000-8000-000000000190', 'Spa & Wellness', true, NOW()),
  ('b2000001-0000-4000-8000-000000000191', 'Nail Salon', true, NOW()),

  -- Other
  ('b2000001-0000-4000-8000-0000000000ff', 'Other', true, NOW())
ON CONFLICT (id) DO NOTHING;
`

var DailyPostLimitsSQL = `
INSERT INTO daily_post_limits (post_type, user_limit, business_multiplier, description)
VALUES
    ('FEED',  5,  2.0, 'Standard social posts'),
    ('EVENT', 2,  2.0, 'Event posts — naturally low frequency'),
    ('SELL',  3,  2.0, 'Marketplace listings — capped to discourage spam'),
    ('PULL',  3,  2.0, 'Polls')
ON CONFLICT (post_type) DO UPDATE
    SET user_limit           = EXCLUDED.user_limit,
        business_multiplier  = EXCLUDED.business_multiplier,
        description          = EXCLUDED.description,
        updated_at           = NOW();
`
