-- Migration: Add indexes on is_active columns for performance
-- Only creates indexes on tables that have is_active column

-- Business categories table
CREATE INDEX IF NOT EXISTS idx_business_categories_is_active
ON business_categories(is_active)
WHERE is_active = true;

-- Add comment for documentation
COMMENT ON INDEX idx_business_categories_is_active IS 'Partial index for active business categories lookup';
