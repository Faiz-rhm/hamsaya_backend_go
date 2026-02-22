-- Add localized name columns for sell_categories (en = name, dari, pashto)
ALTER TABLE sell_categories
  ADD COLUMN IF NOT EXISTS name_dari VARCHAR(200),
  ADD COLUMN IF NOT EXISTS name_pashto VARCHAR(200);

COMMENT ON COLUMN sell_categories.name IS 'Category name in English (locale: en)';
COMMENT ON COLUMN sell_categories.name_dari IS 'Category name in Dari';
COMMENT ON COLUMN sell_categories.name_pashto IS 'Category name in Pashto';
