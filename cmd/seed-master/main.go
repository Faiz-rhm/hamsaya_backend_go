// Command seed-master applies the production-essential, idempotent seed
// data that every Hamsaya deployment needs to function:
//
//	1. The super-admin user (admin@hamsaya.af / Admin123!).
//	2. The 26 sell categories (marketplace taxonomy).
//	3. The default business categories (business directory taxonomy).
//	4. Per-post-type daily limits (FEED / EVENT / SELL / PULL).
//	5. Verifies the custom_roles starter rows from migration 20260429000001
//	   are present (re-applying the same upsert that the migration runs).
//
// All steps use UPSERT / ON CONFLICT semantics and are safe to run on
// every container boot. The entrypoint.sh wrapper calls this command
// after migrate up so a freshly initialised database is fully usable
// without any manual ops work.
//
// For sample / demo data (fake users, posts, businesses) use the
// separate `seed-demo` command instead.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/seedsql"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
)

const (
	adminEmail    = "admin@hamsaya.af"
	adminPassword = "Admin123!"
)

// starterRolesSQL keeps the two default custom roles in sync with what
// migration 20260429000001_create_custom_roles.up.sql ships. Idempotent
// via ON CONFLICT (name).
const starterRolesSQL = `
INSERT INTO custom_roles (name, description, permissions) VALUES
    (
        'Content Manager',
        'Full content moderation: posts, comments, businesses, reports, feedback.',
        '["POSTS_VIEW","POSTS_MUTATE","POSTS_DELETE","COMMENTS_VIEW","COMMENTS_MUTATE","COMMENTS_DELETE","BUSINESSES_VIEW","BUSINESSES_APPROVE","REPORTS_VIEW","REPORTS_RESOLVE"]'
    ),
    (
        'Finance Admin',
        'Monetization oversight: ads, credits, boosts.',
        '["ADS_MANAGE","CREDITS_MANAGE","BOOSTS_MANAGE"]'
    )
ON CONFLICT (name) DO NOTHING;
`

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	if err := utils.InitLogger(cfg.Server.LogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.Sync()
	logger := utils.GetLogger()

	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.Info("[seed-master] applying production seed data")

	if err := seedAdmin(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "seed-master: admin user: %v\n", err)
		os.Exit(1)
	}
	logger.Info("[seed-master] admin user ok", "email", adminEmail)

	if _, err := db.Pool.Exec(ctx, seedsql.SellCategoriesSQL); err != nil {
		fmt.Fprintf(os.Stderr, "seed-master: sell_categories: %v\n", err)
		os.Exit(1)
	}
	logger.Info("[seed-master] sell_categories ok")

	if _, err := db.Pool.Exec(ctx, seedsql.BusinessCategoriesSQL); err != nil {
		fmt.Fprintf(os.Stderr, "seed-master: business_categories: %v\n", err)
		os.Exit(1)
	}
	logger.Info("[seed-master] business_categories ok")

	if _, err := db.Pool.Exec(ctx, seedsql.DailyPostLimitsSQL); err != nil {
		fmt.Fprintf(os.Stderr, "seed-master: daily_post_limits: %v\n", err)
		os.Exit(1)
	}
	logger.Info("[seed-master] daily_post_limits ok")

	if _, err := db.Pool.Exec(ctx, starterRolesSQL); err != nil {
		// Custom_roles table may not exist on very old DBs; warn instead of fail.
		logger.Warn("[seed-master] custom_roles seed skipped", "error", err)
	} else {
		logger.Info("[seed-master] custom_roles ok")
	}

	fmt.Println("seed-master complete: admin + sell_categories + business_categories + daily_post_limits + custom_roles")
}

func seedAdmin(ctx context.Context, db *database.DB) error {
	userRepo := repositories.NewUserRepository(db)
	passwordService := services.NewPasswordService()

	hashed, err := passwordService.Hash(adminPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	existing, _ := userRepo.GetByEmail(ctx, adminEmail)
	if existing == nil {
		existing, _ = userRepo.GetByEmailIncludingDeleted(ctx, adminEmail)
	}

	if existing != nil {
		existing.Role = models.RoleSuperAdmin
		existing.PasswordHash = &hashed
		existing.EmailVerified = true
		if existing.DeletedAt != nil {
			existing.DeletedAt = nil
			if err := userRepo.Restore(ctx, existing.ID); err != nil {
				return fmt.Errorf("restore admin: %w", err)
			}
		}
		if err := userRepo.Update(ctx, existing); err != nil {
			return fmt.Errorf("update admin: %w", err)
		}
		profile, _ := userRepo.GetProfileByUserID(ctx, existing.ID)
		if profile == nil {
			firstName, lastName := "Admin", "User"
			profile = &models.Profile{ID: existing.ID, FirstName: &firstName, LastName: &lastName}
			if err := userRepo.CreateProfile(ctx, profile); err != nil {
				return fmt.Errorf("create admin profile: %w", err)
			}
		}
		return nil
	}

	now := time.Now()
	user := &models.User{
		ID:            uuid.New().String(),
		Email:         adminEmail,
		PasswordHash:  &hashed,
		EmailVerified: true,
		Role:          models.RoleSuperAdmin,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	firstName, lastName := "Admin", "User"
	if err := userRepo.CreateProfile(ctx, &models.Profile{
		ID:        user.ID,
		FirstName: &firstName,
		LastName:  &lastName,
	}); err != nil {
		return fmt.Errorf("create admin profile: %w", err)
	}
	return nil
}
