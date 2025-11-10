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
	"github.com/jackc/pgx/v5/pgtype"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := utils.InitLogger(cfg.Server.LogLevel); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.Sync()

	logger := utils.GetLogger()
	logger.Info("Starting database seeder...")

	// Connect to database
	logger.Info("Connecting to database...")
	db, err := database.New(&cfg.Database)
	if err != nil {
		logger.Fatalw("Failed to connect to database", "error", err)
	}
	defer db.Close()
	logger.Info("Database connected successfully")

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	businessRepo := repositories.NewBusinessRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)

	// Initialize password service for hashing passwords
	passwordService := services.NewPasswordService()

	// Create context
	ctx := context.Background()

	// Run seeders in order
	logger.Info("Starting to seed database...")

	logger.Info("Seeding users...")
	users, err := seedUsers(ctx, userRepo, passwordService)
	if err != nil {
		logger.Fatalw("Failed to seed users", "error", err)
	}
	logger.Infow("Users seeded successfully", "count", len(users))

	logger.Info("Seeding categories...")
	categories, err := seedCategories(ctx, categoryRepo)
	if err != nil {
		logger.Fatalw("Failed to seed categories", "error", err)
	}
	logger.Infow("Categories seeded successfully", "count", len(categories))

	logger.Info("Seeding businesses...")
	businesses, err := seedBusinesses(ctx, businessRepo, users)
	if err != nil {
		logger.Fatalw("Failed to seed businesses", "error", err)
	}
	logger.Infow("Businesses seeded successfully", "count", len(businesses))

	logger.Info("Seeding posts...")
	posts, err := seedPosts(ctx, postRepo, users, businesses, categories)
	if err != nil {
		logger.Fatalw("Failed to seed posts", "error", err)
	}
	logger.Infow("Posts seeded successfully", "count", len(posts))

	logger.Info("Database seeding completed successfully!")
}

// seedUsers creates sample users with profiles
func seedUsers(ctx context.Context, repo repositories.UserRepository, passwordService *services.PasswordService) ([]string, error) {
	users := []struct {
		Email     string
		Password  string
		FirstName string
		LastName  string
		Role      models.UserRole
	}{
		{"admin@hamsaya.af", "Admin123!", "Admin", "User", models.RoleAdmin},
		{"john.doe@example.com", "Password123!", "John", "Doe", models.RoleUser},
		{"jane.smith@example.com", "Password123!", "Jane", "Smith", models.RoleUser},
		{"ahmad.khan@example.com", "Password123!", "Ahmad", "Khan", models.RoleUser},
		{"fatima.ali@example.com", "Password123!", "Fatima", "Ali", models.RoleUser},
		{"hassan.ahmed@example.com", "Password123!", "Hassan", "Ahmed", models.RoleUser},
		{"maryam.shah@example.com", "Password123!", "Maryam", "Shah", models.RoleUser},
		{"rashid.zaman@example.com", "Password123!", "Rashid", "Zaman", models.RoleUser},
		{"sara.hussain@example.com", "Password123!", "Sara", "Hussain", models.RoleUser},
		{"omar.farid@example.com", "Password123!", "Omar", "Farid", models.RoleUser},
	}

	var userIDs []string

	for _, userData := range users {
		// Hash password
		hashedPassword, err := passwordService.Hash(userData.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password for %s: %w", userData.Email, err)
		}

		// Create user
		now := time.Now()
		user := &models.User{
			ID:            uuid.New().String(),
			Email:         userData.Email,
			PasswordHash:  &hashedPassword,
			EmailVerified: true,
			Role:          userData.Role,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if err := repo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user %s: %w", userData.Email, err)
		}

		// Create profile for this user
		profile := &models.Profile{
			ID:        user.ID, // Profile ID matches User ID
			FirstName: &userData.FirstName,
			LastName:  &userData.LastName,
		}

		if err := repo.CreateProfile(ctx, profile); err != nil {
			return nil, fmt.Errorf("failed to create profile for %s: %w", userData.Email, err)
		}

		userIDs = append(userIDs, user.ID)
	}

	return userIDs, nil
}

// seedCategories creates sample marketplace categories
func seedCategories(ctx context.Context, repo repositories.CategoryRepository) ([]string, error) {
	categories := []struct {
		Name  string
		Icon  models.CategoryIcon
		Color string
	}{
		{"Electronics", models.CategoryIcon{Name: "devices", Library: "MaterialIcons"}, "#3B82F6"},
		{"Fashion", models.CategoryIcon{Name: "checkroom", Library: "MaterialIcons"}, "#EC4899"},
		{"Home & Garden", models.CategoryIcon{Name: "home", Library: "MaterialIcons"}, "#10B981"},
		{"Sports", models.CategoryIcon{Name: "sports_soccer", Library: "MaterialIcons"}, "#F59E0B"},
		{"Books", models.CategoryIcon{Name: "menu_book", Library: "MaterialIcons"}, "#8B5CF6"},
		{"Automotive", models.CategoryIcon{Name: "directions_car", Library: "MaterialIcons"}, "#EF4444"},
		{"Food & Beverages", models.CategoryIcon{Name: "restaurant", Library: "MaterialIcons"}, "#F97316"},
		{"Health & Beauty", models.CategoryIcon{Name: "spa", Library: "MaterialIcons"}, "#06B6D4"},
		{"Toys & Games", models.CategoryIcon{Name: "toys", Library: "MaterialIcons"}, "#84CC16"},
		{"Real Estate", models.CategoryIcon{Name: "business", Library: "MaterialIcons"}, "#6366F1"},
	}

	var categoryIDs []string

	for _, cat := range categories {
		category := &models.SellCategory{
			ID:     uuid.New().String(),
			Name:   cat.Name,
			Icon:   cat.Icon,
			Color:  cat.Color,
			Status: models.CategoryStatusActive,
		}

		if err := repo.Create(ctx, category); err != nil {
			return nil, fmt.Errorf("failed to create category %s: %w", cat.Name, err)
		}

		categoryIDs = append(categoryIDs, category.ID)
	}

	return categoryIDs, nil
}

// seedBusinesses creates sample businesses
func seedBusinesses(ctx context.Context, repo repositories.BusinessRepository, userIDs []string) ([]string, error) {
	if len(userIDs) < 5 {
		return nil, fmt.Errorf("not enough users to seed businesses")
	}

	businesses := []struct {
		UserID      string
		Name        string
		LicenseNo   string
		Email       string
		PhoneNumber string
		Province    string
		District    string
		Status      bool
	}{
		{userIDs[1], "Kabul Coffee House", "KBL-2021-001", "info@kabulcoffee.af", "+93700123456", "Kabul", "District 10", true},
		{userIDs[2], "Afghan Handicrafts", "KBL-2020-045", "contact@afghancrafts.af", "+93700234567", "Kabul", "District 2", true},
		{userIDs[3], "Mazar Restaurant", "MZR-2019-123", "info@mazarrest.af", "+93708345678", "Balkh", "Mazar-i-Sharif", true},
		{userIDs[4], "Tech Solutions AF", "KBL-2022-089", "hello@techsol.af", "+93700456789", "Kabul", "District 4", true},
		{userIDs[5], "Kandahar Textiles", "KDR-2018-056", "sales@kdrtextiles.af", "+93707567890", "Kandahar", "District 1", false},
		{userIDs[6], "Herat Carpets", "HRT-2020-078", "info@heratcarpets.af", "+93709678901", "Herat", "District 5", true},
		{userIDs[7], "Jalalabad Foods", "JLB-2021-034", "contact@jalalabadfoods.af", "+93708789012", "Nangarhar", "Jalalabad", true},
		{userIDs[8], "Ghazni Pottery", "GHZ-2019-067", "info@ghaznipottery.af", "+93700890123", "Ghazni", "District 3", true},
	}

	var businessIDs []string

	for _, biz := range businesses {
		business := &models.BusinessProfile{
			ID:          uuid.New().String(),
			UserID:      biz.UserID,
			Name:        biz.Name,
			LicenseNo:   &biz.LicenseNo,
			Email:       &biz.Email,
			PhoneNumber: &biz.PhoneNumber,
			Province:    &biz.Province,
			District:    &biz.District,
			Status:      biz.Status,
		}

		if err := repo.Create(ctx, business); err != nil {
			return nil, fmt.Errorf("failed to create business %s: %w", biz.Name, err)
		}

		businessIDs = append(businessIDs, business.ID)
	}

	return businessIDs, nil
}

// seedPosts creates sample posts of different types
func seedPosts(ctx context.Context, repo repositories.PostRepository, userIDs []string, businessIDs []string, categoryIDs []string) ([]string, error) {
	if len(userIDs) < 3 || len(categoryIDs) < 3 {
		return nil, fmt.Errorf("not enough users or categories to seed posts")
	}

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	nextWeek := now.Add(7 * 24 * time.Hour)

	posts := []struct {
		UserID      *string
		BusinessID  *string
		Type        models.PostType
		Title       string
		Description string
		Visibility  models.PostVisibility
		Status      bool
		CategoryID  *string
		StartDate   *time.Time
		EndDate     *time.Time
	}{
		// FEED posts
		{&userIDs[1], nil, models.PostTypeFeed, "Beautiful day in Kabul", "Just enjoyed a wonderful morning walk in Babur Gardens. The weather is perfect!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[2], nil, models.PostTypeFeed, "New recipe to share", "Made traditional Kabuli Pulao today. Would love to share the recipe with everyone!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[3], nil, models.PostTypeFeed, "Afghan history", "Reading about the rich history of Afghanistan. So much to learn!", models.VisibilityFriends, true, nil, nil, nil},

		// EVENT posts
		{&userIDs[1], nil, models.PostTypeEvent, "Community Gathering", "Join us for a community iftar this weekend. Everyone is welcome!", models.VisibilityPublic, true, nil, &tomorrow, &nextWeek},
		{&userIDs[4], nil, models.PostTypeEvent, "Tech Meetup Kabul", "Monthly tech meetup for developers and entrepreneurs. Free entry!", models.VisibilityPublic, true, nil, &tomorrow, &tomorrow},
		{nil, &businessIDs[0], models.PostTypeEvent, "Coffee Tasting Event", "Special coffee tasting event at Kabul Coffee House. Try our new blends!", models.VisibilityPublic, true, nil, &tomorrow, &tomorrow},

		// SELL posts
		{&userIDs[5], nil, models.PostTypeSell, "Laptop for Sale", "Dell XPS 15, excellent condition. 2 years old, barely used. Great for students!", models.VisibilityPublic, true, &categoryIDs[0], nil, nil},
		{&userIDs[6], nil, models.PostTypeSell, "Traditional Dress", "Handmade Afghan traditional dress. Beautiful embroidery. Size M.", models.VisibilityPublic, true, &categoryIDs[1], nil, nil},
		{nil, &businessIDs[4], models.PostTypeSell, "Premium Cotton Fabric", "High quality cotton fabric, various colors available. Wholesale prices!", models.VisibilityPublic, true, &categoryIDs[1], nil, nil},

		// PULL (Poll) posts
		{&userIDs[7], nil, models.PostTypePull, "Best Afghan dish?", "What's your favorite traditional Afghan dish? Vote below!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[8], nil, models.PostTypePull, "Weekend plans", "What are your plans for the weekend?", models.VisibilityFriends, true, nil, nil, nil},
	}

	var postIDs []string

	for _, postData := range posts {
		// Create location (Kabul coordinates as default)
		location := pgtype.Point{
			P:     pgtype.Vec2{X: 69.2075, Y: 34.5553},
			Valid: true,
		}

		postNow := time.Now()
		post := &models.Post{
			ID:              uuid.New().String(),
			Type:            postData.Type,
			Title:           &postData.Title,
			Description:     &postData.Description,
			Visibility:      postData.Visibility,
			Status:          postData.Status,
			AddressLocation: &location,
			CreatedAt:       postNow,
			UpdatedAt:       postNow,
		}

		if postData.UserID != nil {
			post.UserID = postData.UserID
		}
		if postData.BusinessID != nil {
			post.BusinessID = postData.BusinessID
		}
		if postData.CategoryID != nil {
			post.CategoryID = postData.CategoryID
		}
		if postData.StartDate != nil {
			post.StartDate = postData.StartDate
		}
		if postData.EndDate != nil {
			post.EndDate = postData.EndDate
		}

		if err := repo.Create(ctx, post); err != nil {
			return nil, fmt.Errorf("failed to create post %s: %w", postData.Title, err)
		}

		postIDs = append(postIDs, post.ID)
	}

	return postIDs, nil
}
