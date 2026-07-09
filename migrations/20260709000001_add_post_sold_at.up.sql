-- When a SELL listing was marked sold — powers the "items sold" insights
-- chart. Backfill existing sold posts from updated_at (best available
-- approximation).
ALTER TABLE posts ADD COLUMN IF NOT EXISTS sold_at TIMESTAMP WITH TIME ZONE;

UPDATE posts SET sold_at = updated_at WHERE sold = true AND sold_at IS NULL;
