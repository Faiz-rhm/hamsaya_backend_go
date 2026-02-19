// Command db-reset truncates all application tables (keeps schema) and re-seeds sell_categories and business_categories.
// With env SEED_CATEGORIES_ONLY=1 it only runs both category seeds (no truncate).
// With env SEED_BUSINESS_CATEGORIES_ONLY=1 it only runs business_categories seed (no truncate).
// Uses the same .env config as the rest of the app.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/pkg/database"
)

// seedSellCategories inserts default marketplace categories (same as migration 20260211000001).
const seedSellCategoriesSQL = `
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
`

// seedBusinessCategories inserts default business categories.
const seedBusinessCategoriesSQL = `
INSERT INTO business_categories (id, name, is_active, created_at) VALUES
  ('b2000001-0000-4000-8000-000000000001', 'Retail', true, NOW()),
  ('b2000001-0000-4000-8000-000000000002', 'Restaurant', true, NOW()),
  ('b2000001-0000-4000-8000-000000000003', 'Cafe & Coffee', true, NOW()),
  ('b2000001-0000-4000-8000-000000000004', 'Food & Beverage', true, NOW()),
  ('b2000001-0000-4000-8000-000000000005', 'Doctor & Medical', true, NOW()),
  ('b2000001-0000-4000-8000-000000000006', 'Engineer', true, NOW()),
  ('b2000001-0000-4000-8000-000000000007', 'Health & Beauty', true, NOW()),
  ('b2000001-0000-4000-8000-000000000008', 'Automotive', true, NOW()),
  ('b2000001-0000-4000-8000-000000000009', 'Education', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000a', 'Entertainment', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000b', 'Hospitality', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000c', 'Professional Services', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000d', 'Real Estate', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000e', 'Technology', true, NOW()),
  ('b2000001-0000-4000-8000-00000000000f', 'Bakery', true, NOW()),
  ('b2000001-0000-4000-8000-000000000010', 'Bar & Pub', true, NOW()),
  ('b2000001-0000-4000-8000-000000000011', 'Grocery & Supermarket', true, NOW()),
  ('b2000001-0000-4000-8000-000000000012', 'Pharmacy', true, NOW()),
  ('b2000001-0000-4000-8000-000000000013', 'Clothing & Fashion', true, NOW()),
  ('b2000001-0000-4000-8000-000000000014', 'Electronics Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000015', 'Furniture Store', true, NOW()),
  ('b2000001-0000-4000-8000-000000000016', 'Gym & Fitness', true, NOW()),
  ('b2000001-0000-4000-8000-000000000017', 'Salon & Barber', true, NOW()),
  ('b2000001-0000-4000-8000-000000000018', 'Repair & Maintenance', true, NOW()),
  ('b2000001-0000-4000-8000-000000000019', 'Construction & Contractor', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001a', 'Legal Services', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001b', 'Accounting & Finance', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001c', 'Travel & Tourism', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001d', 'Event Planning', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001e', 'Photography', true, NOW()),
  ('b2000001-0000-4000-8000-00000000001f', 'Printing & Copy', true, NOW()),
  ('b2000001-0000-4000-8000-000000000020', 'Cleaning Services', true, NOW()),
  ('b2000001-0000-4000-8000-000000000021', 'Architect', true, NOW()),
  ('b2000001-0000-4000-8000-000000000022', 'Dentist', true, NOW()),
  ('b2000001-0000-4000-8000-000000000023', 'Veterinary', true, NOW()),
  ('b2000001-0000-4000-8000-000000000024', 'Insurance', true, NOW()),
  ('b2000001-0000-4000-8000-000000000025', 'Other', true, NOW())
ON CONFLICT (id) DO NOTHING;
`

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Only seed business_categories (no truncate).
	if os.Getenv("SEED_BUSINESS_CATEGORIES_ONLY") == "1" {
		_, err = db.Pool.Exec(ctx, seedBusinessCategoriesSQL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed business_categories: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Business categories seeded successfully.")
		return
	}

	// Only seed both category tables (no truncate).
	if os.Getenv("SEED_CATEGORIES_ONLY") == "1" {
		_, err = db.Pool.Exec(ctx, seedSellCategoriesSQL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed sell_categories: %v\n", err)
			os.Exit(1)
		}
		_, err = db.Pool.Exec(ctx, seedBusinessCategoriesSQL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed business_categories: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Sell and business categories seeded successfully.")
		return
	}

	// Full reset: truncate then seed both.
	_, err = db.Pool.Exec(ctx, `
		TRUNCATE TABLE users, token_blacklist RESTART IDENTITY CASCADE;
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to truncate database: %v\n", err)
		os.Exit(1)
	}
	_, err = db.Pool.Exec(ctx, seedSellCategoriesSQL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to seed sell_categories: %v\n", err)
		os.Exit(1)
	}
	_, err = db.Pool.Exec(ctx, seedBusinessCategoriesSQL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to seed business_categories: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("All data removed. Sell and business categories re-seeded.")
}
