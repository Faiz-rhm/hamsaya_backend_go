// seed-admin creates or fixes the admin user for the admin panel.
// Email: admin@hamsaya.af, Password: Admin123!
// If the user already exists (e.g. auto-created by login with role=user), updates role to admin and resets password.
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
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
)

const adminEmail = "admin@hamsaya.af"
const adminPassword = "Admin123!"

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

	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	userRepo := repositories.NewUserRepository(db)
	passwordService := services.NewPasswordService()
	ctx := context.Background()

	hashedPassword, err := passwordService.Hash(adminPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	// Check existing (active or deleted) so we can fix role/password if they were auto-created as user
	existing, _ := userRepo.GetByEmail(ctx, adminEmail)
	if existing == nil {
		existing, _ = userRepo.GetByEmailIncludingDeleted(ctx, adminEmail)
	}

	if existing != nil {
		// Ensure role is admin and password is Admin123! (fixes auto-registered-as-user case)
		existing.Role = models.RoleAdmin
		existing.PasswordHash = &hashedPassword
		existing.EmailVerified = true
		if existing.DeletedAt != nil {
			existing.DeletedAt = nil
			if err := userRepo.Restore(ctx, existing.ID); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to restore user: %v\n", err)
				os.Exit(1)
			}
		}
		if err := userRepo.Update(ctx, existing); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update admin user: %v\n", err)
			os.Exit(1)
		}
		// Ensure profile exists
		profile, _ := userRepo.GetProfileByUserID(ctx, existing.ID)
		if profile == nil {
			firstName, lastName := "Admin", "User"
			profile = &models.Profile{ID: existing.ID, FirstName: &firstName, LastName: &lastName}
			if err := userRepo.CreateProfile(ctx, profile); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create admin profile: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Println("Admin user updated. Email: admin@hamsaya.af, Password: Admin123!")
		return
	}

	// Create new admin user
	now := time.Now()
	user := &models.User{
		ID:            uuid.New().String(),
		Email:         adminEmail,
		PasswordHash:  &hashedPassword,
		EmailVerified: true,
		Role:          models.RoleAdmin,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create admin user: %v\n", err)
		os.Exit(1)
	}

	firstName, lastName := "Admin", "User"
	profile := &models.Profile{
		ID:        user.ID,
		FirstName: &firstName,
		LastName:  &lastName,
	}
	if err := userRepo.CreateProfile(ctx, profile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create admin profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Admin user seeded successfully. Email: admin@hamsaya.af, Password: Admin123!")
}
