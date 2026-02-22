-- Remove duplicate "Automotive" category (MaterialIcons version); keep canonical one (mdi).
-- Reassign any posts using the duplicate to the canonical Automotive category, then delete.

UPDATE posts
SET category_id = 'a1000001-0000-4000-8000-000000000003'
WHERE category_id = '19d26fe9-b5ba-4a67-a91c-8c58b7cd8043';

DELETE FROM sell_categories
WHERE id = '19d26fe9-b5ba-4a67-a91c-8c58b7cd8043';
