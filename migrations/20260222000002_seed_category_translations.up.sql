-- Seed Dari and Pashto translations for sell_categories (en remains in name)
-- Fallback: when name_dari/name_pashto is NULL, API returns name (en)

UPDATE sell_categories SET name_dari = N'همه دسته‌ها', name_pashto = N'ټولې کټګورۍ' WHERE id = 'a1000001-0000-4000-8000-000000000001';
UPDATE sell_categories SET name_dari = N'لوازم خانگی', name_pashto = N'کورني وسایل' WHERE id = 'a1000001-0000-4000-8000-000000000002';
UPDATE sell_categories SET name_dari = N'موتر', name_pashto = N'موټر' WHERE id = 'a1000001-0000-4000-8000-000000000003';
UPDATE sell_categories SET name_dari = N'طفل و اطفال', name_pashto = N'ماشومان او کوچنيان' WHERE id = 'a1000001-0000-4000-8000-000000000004';
UPDATE sell_categories SET name_dari = N'دوچرخه', name_pashto = N'دوچرخې' WHERE id = 'a1000001-0000-4000-8000-000000000005';
UPDATE sell_categories SET name_dari = N'لباس و اکسسوار', name_pashto = N'جامې او لوازم' WHERE id = 'a1000001-0000-4000-8000-000000000006';
UPDATE sell_categories SET name_dari = N'الکترونیک', name_pashto = N'برېښنایی' WHERE id = 'a1000001-0000-4000-8000-000000000007';
UPDATE sell_categories SET name_dari = N'مبلمان', name_pashto = N'فرنيچر' WHERE id = 'a1000001-0000-4000-8000-000000000008';
UPDATE sell_categories SET name_dari = N'باغ', name_pashto = N'باغ' WHERE id = 'a1000001-0000-4000-8000-000000000009';
UPDATE sell_categories SET name_dari = N'تزئین خانه', name_pashto = N'کور ډیزاین' WHERE id = 'a1000001-0000-4000-8000-00000000000a';
UPDATE sell_categories SET name_dari = N'فروش خانه', name_pashto = N'کور پلور' WHERE id = 'a1000001-0000-4000-8000-00000000000b';
UPDATE sell_categories SET name_dari = N'آلات موسیقی', name_pashto = N'موسيقي الې' WHERE id = 'a1000001-0000-4000-8000-00000000000c';
UPDATE sell_categories SET name_dari = N'ساخت همسایه', name_pashto = N'ګاونډي جوړ شوی' WHERE id = 'a1000001-0000-4000-8000-00000000000d';
UPDATE sell_categories SET name_dari = N'خدمات همسایه', name_pashto = N'ګاونډي خدمتونه' WHERE id = 'a1000001-0000-4000-8000-00000000000e';
UPDATE sell_categories SET name_dari = N'سایر', name_pashto = N'نور' WHERE id = 'a1000001-0000-4000-8000-00000000000f';
UPDATE sell_categories SET name_dari = N'لوازم حیوانات', name_pashto = N'د حیواناتو لوازم' WHERE id = 'a1000001-0000-4000-8000-000000000010';
UPDATE sell_categories SET name_dari = N'اجاره ملک', name_pashto = N'په کره کور' WHERE id = 'a1000001-0000-4000-8000-000000000011';
UPDATE sell_categories SET name_dari = N'ورزش و فضای باز', name_pashto = N'ورزش او بهر' WHERE id = 'a1000001-0000-4000-8000-000000000012';
UPDATE sell_categories SET name_dari = N'ابزار', name_pashto = N'اوزار' WHERE id = 'a1000001-0000-4000-8000-000000000013';
UPDATE sell_categories SET name_dari = N'اسباب بازی و بازی', name_pashto = N'لوبې او لوبي' WHERE id = 'a1000001-0000-4000-8000-000000000014';

-- Optional: translate duplicate/alternate categories (different UUIDs, same English names)
UPDATE sell_categories SET name_dari = N'موتر', name_pashto = N'موټر' WHERE id = '19d26fe9-b5ba-4a67-a91c-8c58b7cd8043';
UPDATE sell_categories SET name_dari = N'کتاب', name_pashto = N'کتاب' WHERE id = '54c584be-cbfd-438b-9e87-586a3c1e22dd';
UPDATE sell_categories SET name_dari = N'الکترونیک', name_pashto = N'برېښنایی' WHERE id = '6e2db648-3d34-4c4a-a77d-cb9c938feed8';
UPDATE sell_categories SET name_dari = N'مد و پوشاک', name_pashto = N'فیشن' WHERE id = 'f8c5abb0-4497-41db-9245-2273673075a9';
UPDATE sell_categories SET name_dari = N'خوراک و نوشیدنی', name_pashto = N'خواړه او مشروبات' WHERE id = '2c43fb70-573c-4c10-baea-0b9fa66a109d';
UPDATE sell_categories SET name_dari = N'صحت و زیبایی', name_pashto = N'روغتیا او ښکلا' WHERE id = 'dca48855-d528-435f-963d-25debafc0180';
UPDATE sell_categories SET name_dari = N'خانه و باغ', name_pashto = N'کور او باغ' WHERE id = '1e8b0c85-85b0-44a0-959a-eff515ce6195';
UPDATE sell_categories SET name_dari = N'مِلکیت', name_pashto = N'جایداد' WHERE id = '9a350f26-8322-4038-b626-779817fe788e';
UPDATE sell_categories SET name_dari = N'ورزش', name_pashto = N'ورزش' WHERE id = '2a3f0c1c-e324-425e-8425-fd3170b60e55';
UPDATE sell_categories SET name_dari = N'اسباب بازی و بازی', name_pashto = N'لوبې او لوبي' WHERE id = '1c5e1a1b-37f1-47a2-b365-d9c843bc5f5b';
