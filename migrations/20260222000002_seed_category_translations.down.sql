-- Clear localized names (columns remain; down of 20260222000001 drops them)
UPDATE sell_categories SET name_dari = NULL, name_pashto = NULL;
