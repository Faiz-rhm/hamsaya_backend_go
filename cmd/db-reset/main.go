// Command db-reset truncates all application tables (keeps schema) and re-seeds
// sell_categories, business_categories, and daily_post_limits.
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
	"github.com/hamsaya/backend/internal/seedsql"
	"github.com/hamsaya/backend/pkg/database"
)

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

	// Only seed business_categories (truncate first for clean slate).
	if os.Getenv("SEED_BUSINESS_CATEGORIES_ONLY") == "1" {
		if _, err = db.Pool.Exec(ctx, `TRUNCATE TABLE business_categories CASCADE;`); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to truncate business_categories: %v\n", err)
			os.Exit(1)
		}
		if _, err = db.Pool.Exec(ctx, seedsql.BusinessCategoriesSQL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed business_categories: %v\n", err)
			os.Exit(1)
		}
		if _, err = db.Pool.Exec(ctx, seedsql.DailyPostLimitsSQL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed daily_post_limits: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Business categories and daily post limits seeded successfully.")
		return
	}

	// Only seed both category tables (truncate categories first for clean slate).
	if os.Getenv("SEED_CATEGORIES_ONLY") == "1" {
		if _, err = db.Pool.Exec(ctx, `TRUNCATE TABLE sell_categories, business_categories CASCADE;`); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to truncate category tables: %v\n", err)
			os.Exit(1)
		}
		if _, err = db.Pool.Exec(ctx, seedsql.SellCategoriesSQL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed sell_categories: %v\n", err)
			os.Exit(1)
		}
		if _, err = db.Pool.Exec(ctx, seedsql.BusinessCategoriesSQL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed business_categories: %v\n", err)
			os.Exit(1)
		}
		if _, err = db.Pool.Exec(ctx, seedsql.DailyPostLimitsSQL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed daily_post_limits: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Sell and business categories and daily post limits seeded successfully.")
		return
	}

	// Full reset: truncate then seed all.
	if _, err = db.Pool.Exec(ctx, `
		TRUNCATE TABLE sell_categories, business_categories CASCADE;
		TRUNCATE TABLE users, token_blacklist RESTART IDENTITY CASCADE;
	`); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to truncate database: %v\n", err)
		os.Exit(1)
	}
	if _, err = db.Pool.Exec(ctx, seedsql.SellCategoriesSQL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to seed sell_categories: %v\n", err)
		os.Exit(1)
	}
	if _, err = db.Pool.Exec(ctx, seedsql.BusinessCategoriesSQL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to seed business_categories: %v\n", err)
		os.Exit(1)
	}
	if _, err = db.Pool.Exec(ctx, seedsql.DailyPostLimitsSQL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to seed daily_post_limits: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("All data removed. Sell and business categories and daily post limits re-seeded.")
}
