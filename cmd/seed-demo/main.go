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
	logger.Info("Starting demo database seeder...")

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
	commentRepo := repositories.NewCommentRepository(db)
	pollRepo := repositories.NewPollRepository(db)
	eventRepo := repositories.NewEventRepository(db)
	relationshipsRepo := repositories.NewRelationshipsRepository(db)
	reportRepo := repositories.NewReportRepository(db)

	// Initialize password service
	passwordService := services.NewPasswordService()

	// Create context
	ctx := context.Background()

	// Run demo seeders
	logger.Info("Starting demo data seeding...")

	logger.Info("Seeding demo users...")
	users, err := seedDemoUsers(ctx, userRepo, passwordService)
	if err != nil {
		logger.Fatalw("Failed to seed demo users", "error", err)
	}
	logger.Infow("Demo users seeded", "count", len(users))

	logger.Info("Seeding demo categories...")
	categories, err := seedDemoCategories(ctx, categoryRepo)
	if err != nil {
		logger.Fatalw("Failed to seed demo categories", "error", err)
	}
	logger.Infow("Demo categories seeded", "count", len(categories))

	logger.Info("Seeding demo businesses...")
	businesses, err := seedDemoBusinesses(ctx, businessRepo, users)
	if err != nil {
		logger.Fatalw("Failed to seed demo businesses", "error", err)
	}
	logger.Infow("Demo businesses seeded", "count", len(businesses))

	logger.Info("Seeding demo posts...")
	posts, err := seedDemoPosts(ctx, postRepo, users, businesses, categories)
	if err != nil {
		logger.Fatalw("Failed to seed demo posts", "error", err)
	}
	logger.Infow("Demo posts seeded", "count", len(posts))

	logger.Info("Seeding demo polls...")
	err = seedDemoPolls(ctx, pollRepo, posts, users)
	if err != nil {
		logger.Fatalw("Failed to seed demo polls", "error", err)
	}
	logger.Info("Demo polls seeded successfully")

	logger.Info("Seeding demo comments...")
	comments, err := seedDemoComments(ctx, commentRepo, posts, users)
	if err != nil {
		logger.Fatalw("Failed to seed demo comments", "error", err)
	}
	logger.Infow("Demo comments seeded", "count", len(comments))

	logger.Info("Seeding demo likes...")
	err = seedDemoLikes(ctx, db, posts, users)
	if err != nil {
		logger.Fatalw("Failed to seed demo likes", "error", err)
	}
	logger.Info("Demo likes seeded successfully")

	logger.Info("Seeding demo event interests...")
	err = seedDemoEventInterests(ctx, eventRepo, posts, users)
	if err != nil {
		logger.Fatalw("Failed to seed demo event interests", "error", err)
	}
	logger.Info("Demo event interests seeded successfully")

	logger.Info("Seeding demo user relationships...")
	err = seedDemoRelationships(ctx, relationshipsRepo, users)
	if err != nil {
		logger.Fatalw("Failed to seed demo relationships", "error", err)
	}
	logger.Info("Demo relationships seeded successfully")

	logger.Info("Seeding demo reports...")
	err = seedDemoReports(ctx, reportRepo, posts, comments, users, businesses)
	if err != nil {
		logger.Fatalw("Failed to seed demo reports", "error", err)
	}
	logger.Info("Demo reports seeded successfully")

	logger.Info("Demo database seeding completed successfully!")
	logger.Info("---")
	logger.Info("Demo Admin Credentials:")
	logger.Info("  Email: demo@hamsaya.af")
	logger.Info("  Password: Demo123!")
}

func seedDemoUsers(ctx context.Context, repo repositories.UserRepository, passwordService *services.PasswordService) ([]string, error) {
	users := []struct {
		Email      string
		Password   string
		FirstName  string
		LastName   string
		Role       models.UserRole
		IsInactive bool // Mark some users as inactive for demo purposes
	}{
		{"demo@hamsaya.af", "Demo123!", "Demo", "Admin", models.RoleAdmin, false},
		{"alice.wilson@demo.af", "Demo123!", "Alice", "Wilson", models.RoleUser, false},
		{"bob.anderson@demo.af", "Demo123!", "Bob", "Anderson", models.RoleUser, false},
		{"carol.martinez@demo.af", "Demo123!", "Carol", "Martinez", models.RoleUser, true}, // Inactive
		{"david.garcia@demo.af", "Demo123!", "David", "Garcia", models.RoleUser, false},
		{"emma.rodriguez@demo.af", "Demo123!", "Emma", "Rodriguez", models.RoleUser, false},
		{"frank.wilson@demo.af", "Demo123!", "Frank", "Wilson", models.RoleUser, true}, // Inactive
		{"grace.lee@demo.af", "Demo123!", "Grace", "Lee", models.RoleUser, false},
		{"henry.brown@demo.af", "Demo123!", "Henry", "Brown", models.RoleUser, false},
		{"iris.davis@demo.af", "Demo123!", "Iris", "Davis", models.RoleUser, true}, // Inactive
		{"jack.miller@demo.af", "Demo123!", "Jack", "Miller", models.RoleUser, false},
		{"kate.moore@demo.af", "Demo123!", "Kate", "Moore", models.RoleUser, false},
		{"leo.taylor@demo.af", "Demo123!", "Leo", "Taylor", models.RoleUser, false},
		{"maya.thomas@demo.af", "Demo123!", "Maya", "Thomas", models.RoleUser, true}, // Inactive
		{"noah.jackson@demo.af", "Demo123!", "Noah", "Jackson", models.RoleUser, false},
	}

	var userIDs []string

	for _, userData := range users {
		// Check if user exists
		existingUser, _ := repo.GetByEmail(ctx, userData.Email)
		if existingUser != nil {
			userIDs = append(userIDs, existingUser.ID)
			continue
		}

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

		// Set deleted_at for inactive users (soft delete)
		if userData.IsInactive {
			deletedTime := now.Add(-30 * 24 * time.Hour) // Marked as inactive 30 days ago
			user.DeletedAt = &deletedTime
		}

		if err := repo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user %s: %w", userData.Email, err)
		}

		// Create profile
		profile := &models.Profile{
			ID:        user.ID,
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

func seedDemoCategories(ctx context.Context, repo repositories.CategoryRepository) ([]string, error) {
	categories := []struct {
		Name  string
		Icon  models.CategoryIcon
		Color string
	}{
		{"Electronics", models.CategoryIcon{Name: "devices", Library: "MaterialIcons"}, "#3B82F6"},
		{"Fashion & Clothing", models.CategoryIcon{Name: "checkroom", Library: "MaterialIcons"}, "#EC4899"},
		{"Home & Garden", models.CategoryIcon{Name: "home", Library: "MaterialIcons"}, "#10B981"},
		{"Sports & Outdoors", models.CategoryIcon{Name: "sports_soccer", Library: "MaterialIcons"}, "#F59E0B"},
		{"Books & Education", models.CategoryIcon{Name: "menu_book", Library: "MaterialIcons"}, "#8B5CF6"},
		{"Automotive", models.CategoryIcon{Name: "directions_car", Library: "MaterialIcons"}, "#EF4444"},
		{"Food & Beverages", models.CategoryIcon{Name: "restaurant", Library: "MaterialIcons"}, "#F97316"},
		{"Health & Beauty", models.CategoryIcon{Name: "spa", Library: "MaterialIcons"}, "#06B6D4"},
		{"Toys & Kids", models.CategoryIcon{Name: "toys", Library: "MaterialIcons"}, "#84CC16"},
		{"Real Estate", models.CategoryIcon{Name: "business", Library: "MaterialIcons"}, "#6366F1"},
		{"Furniture", models.CategoryIcon{Name: "chair", Library: "MaterialIcons"}, "#14B8A6"},
		{"Services", models.CategoryIcon{Name: "build", Library: "MaterialIcons"}, "#A855F7"},
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
			// If category already exists, try to get it
			if existing, getErr := repo.GetByID(ctx, category.ID); getErr == nil && existing != nil {
				categoryIDs = append(categoryIDs, existing.ID)
				continue
			}
			return nil, fmt.Errorf("failed to create category %s: %w", cat.Name, err)
		}

		categoryIDs = append(categoryIDs, category.ID)
	}

	return categoryIDs, nil
}

func seedDemoBusinesses(ctx context.Context, repo repositories.BusinessRepository, userIDs []string) ([]string, error) {
	if len(userIDs) < 10 {
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
		{userIDs[1], "Kabul Tech Hub", "KBL-2023-001", "info@kabultechhub.af", "+93700111222", "Kabul", "District 10", true},
		{userIDs[2], "Afghan Heritage Gallery", "KBL-2023-002", "contact@heritage.af", "+93700222333", "Kabul", "District 2", true},
		{userIDs[3], "Balkh Cuisine Restaurant", "MZR-2023-003", "info@balkhcuisine.af", "+93700333444", "Balkh", "Mazar-i-Sharif", true},
		{userIDs[4], "Digital Solutions Afghanistan", "KBL-2023-004", "hello@digitalsol.af", "+93700444555", "Kabul", "District 4", true},
		{userIDs[5], "Kandahar Fashion House", "KDR-2023-005", "sales@kdfashion.af", "+93700555666", "Kandahar", "District 1", true},
		{userIDs[6], "Herat Artisan Crafts", "HRT-2023-006", "info@heratcrafts.af", "+93700666777", "Herat", "District 5", true},
		{userIDs[7], "Jalalabad Fresh Foods", "JLB-2023-007", "contact@jfresh.af", "+93700777888", "Nangarhar", "Jalalabad", true},
		{userIDs[8], "Ghazni Traditional Arts", "GHZ-2023-008", "info@ghazniarts.af", "+93700888999", "Ghazni", "District 3", true},
		{userIDs[9], "Kabul Coffee Lounge", "KBL-2023-009", "hello@coffeelounge.af", "+93700999000", "Kabul", "District 15", true},
		{userIDs[10], "Afghan Bookstore", "KBL-2023-010", "info@bookstore.af", "+93701000111", "Kabul", "District 8", true},
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

func seedDemoPosts(ctx context.Context, repo repositories.PostRepository, userIDs []string, businessIDs []string, categoryIDs []string) ([]string, error) {
	if len(userIDs) < 5 || len(categoryIDs) < 3 {
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
		{&userIDs[1], nil, models.PostTypeFeed, "Welcome to Hamsaya!", "Excited to join this amazing community platform. Looking forward to connecting with everyone!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[2], nil, models.PostTypeFeed, "Beautiful sunrise in Kabul", "Captured this amazing sunrise from Bibi Mahro. The city looks stunning in the morning light!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[3], nil, models.PostTypeFeed, "Afghan traditional recipe: Qabili Palau", "Sharing my grandmother's authentic Qabili Palau recipe. It's been in our family for generations!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[4], nil, models.PostTypeFeed, "Tech innovation in Afghanistan", "Great to see the growing tech ecosystem in Afghanistan. The future is bright!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[5], nil, models.PostTypeFeed, "Weekend at Bagh-e Babur", "Spent a wonderful afternoon at Babur Gardens with family. Perfect weather!", models.VisibilityFriends, true, nil, nil, nil},

		// EVENT posts
		{&userIDs[1], nil, models.PostTypeEvent, "Tech Meetup Kabul 2024", "Join us for a networking event for tech enthusiasts and entrepreneurs. Guest speakers from leading tech companies!", models.VisibilityPublic, true, nil, &tomorrow, &tomorrow},
		{&userIDs[6], nil, models.PostTypeEvent, "Community Iftar Dinner", "Everyone is welcome to join our community iftar. Let's break bread together!", models.VisibilityPublic, true, nil, &tomorrow, &tomorrow},
		{nil, &businessIDs[0], models.PostTypeEvent, "Coffee Tasting Workshop", "Learn about different coffee brewing methods. Free samples and certificates provided!", models.VisibilityPublic, true, nil, &nextWeek, &nextWeek},
		{nil, &businessIDs[2], models.PostTypeEvent, "Grand Opening Celebration", "Join us for the grand opening of our new location! Special discounts and prizes!", models.VisibilityPublic, true, nil, &tomorrow, &tomorrow},
		{&userIDs[7], nil, models.PostTypeEvent, "Charity Run for Education", "5K charity run to support local schools. All proceeds go to buying books and supplies!", models.VisibilityPublic, true, nil, &nextWeek, &nextWeek},

		// SELL posts
		{&userIDs[8], nil, models.PostTypeSell, "MacBook Pro 2021 - Like New", "MacBook Pro M1 Pro, 16GB RAM, 512GB SSD. Barely used, comes with original box and charger. Perfect for students and professionals!", models.VisibilityPublic, true, &categoryIDs[0], nil, nil},
		{&userIDs[9], nil, models.PostTypeSell, "Traditional Afghan Dress - Handmade", "Beautiful handmade Afghan dress with intricate embroidery. Size M. Perfect for weddings and special occasions!", models.VisibilityPublic, true, &categoryIDs[1], nil, nil},
		{nil, &businessIDs[4], models.PostTypeSell, "Designer Clothing Collection", "New arrival of premium designer clothing. Limited stock available. Visit our showroom for exclusive deals!", models.VisibilityPublic, true, &categoryIDs[1], nil, nil},
		{&userIDs[10], nil, models.PostTypeSell, "Complete Book Collection - Dari Literature", "Selling my personal collection of classic Dari literature. Over 50 books in excellent condition!", models.VisibilityPublic, true, &categoryIDs[4], nil, nil},
		{&userIDs[11], nil, models.PostTypeSell, "iPhone 14 Pro Max", "iPhone 14 Pro Max, 256GB, Deep Purple. 10/10 condition, still under warranty. Comes with all accessories!", models.VisibilityPublic, true, &categoryIDs[0], nil, nil},

		// PULL (Poll) posts
		{&userIDs[12], nil, models.PostTypePull, "Best Afghan dish poll", "What's your favorite traditional Afghan dish? Let's see which one wins!", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[13], nil, models.PostTypePull, "Weekend activity preferences", "How do you prefer to spend your weekends in Kabul?", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[14], nil, models.PostTypePull, "Tech platform preference", "Which platform do you use most for staying connected?", models.VisibilityPublic, true, nil, nil, nil},
		{nil, &businessIDs[8], models.PostTypePull, "Coffee preference survey", "Help us serve you better! What's your favorite coffee style?", models.VisibilityPublic, true, nil, nil, nil},
		{&userIDs[2], nil, models.PostTypePull, "Best time for community events", "When is the best time for community gatherings?", models.VisibilityPublic, true, nil, nil, nil},
	}

	var postIDs []string

	for _, postData := range posts {
		postNow := now.Add(time.Duration(len(postIDs)-10) * time.Hour) // Spread posts over time
		post := &models.Post{
			ID:              uuid.New().String(),
			Type:            postData.Type,
			Title:           &postData.Title,
			Description:     &postData.Description,
			Visibility:      postData.Visibility,
			Status:          postData.Status,
			AddressLocation: nil, // Set to nil for demo data (PostGIS GEOGRAPHY compatibility)
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

func seedDemoPolls(ctx context.Context, repo repositories.PollRepository, postIDs []string, userIDs []string) error {
	// Find PULL type posts (last 5 posts in our seed data)
	pollPosts := postIDs[len(postIDs)-5:]

	pollOptions := [][]string{
		{"Kabuli Pulao", "Mantu", "Bolani", "Qorma"},
		{"Outdoor activities", "Reading at home", "Visiting family", "Shopping"},
		{"WhatsApp", "Instagram", "Facebook", "Twitter"},
		{"Espresso", "Cappuccino", "Latte", "Turkish Coffee"},
		{"Morning (8-11 AM)", "Afternoon (2-5 PM)", "Evening (6-9 PM)", "Weekend"},
	}

	for i, postID := range pollPosts {
		// Create poll
		now := time.Now()
		poll := &models.Poll{
			ID:        uuid.New().String(),
			PostID:    postID,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := repo.Create(ctx, poll); err != nil {
			return fmt.Errorf("failed to create poll for post %s: %w", postID, err)
		}

		// Create poll options
		var optionIDs []string
		for _, optionText := range pollOptions[i] {
			option := &models.PollOption{
				ID:        uuid.New().String(),
				PollID:    poll.ID,
				Option:    optionText,
				VoteCount: 0,
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := repo.CreateOption(ctx, option); err != nil {
				return fmt.Errorf("failed to create poll option: %w", err)
			}
			optionIDs = append(optionIDs, option.ID)
		}

		// Add some votes to make it realistic
		numVoters := len(userIDs) / 3
		for j := 0; j < numVoters && j < len(userIDs); j++ {
			// Distribute votes across different options
			optionIndex := (i + j) % len(optionIDs)
			vote := &models.UserPoll{
				ID:           uuid.New().String(),
				UserID:       userIDs[j],
				PollID:       poll.ID,
				PollOptionID: optionIDs[optionIndex],
				CreatedAt:    now,
			}
			_ = repo.VotePoll(ctx, vote)
		}
	}

	return nil
}

func seedDemoComments(ctx context.Context, repo repositories.CommentRepository, postIDs []string, userIDs []string) ([]string, error) {
	comments := []struct {
		PostIdx int
		UserIdx int
		Text    string
	}{
		{0, 2, "Welcome! Glad to have you here!"},
		{0, 3, "This platform is amazing, you'll love it!"},
		{1, 4, "Stunning photo! Kabul is beautiful in the morning."},
		{2, 5, "Would love to try this recipe! Can you share the full details?"},
		{2, 6, "My grandmother makes the best Qabili Palau too!"},
		{3, 7, "Completely agree! The tech scene is growing rapidly."},
		{4, 8, "Bagh-e Babur is one of my favorite places!"},
		{5, 9, "Count me in for the tech meetup!"},
		{6, 10, "What time does the iftar start?"},
		{7, 11, "This sounds amazing! Will definitely attend."},
	}

	var commentIDs []string

	for _, comment := range comments {
		if comment.PostIdx >= len(postIDs) || comment.UserIdx >= len(userIDs) {
			continue
		}

		now := time.Now()
		commentModel := &models.PostComment{
			ID:        uuid.New().String(),
			PostID:    postIDs[comment.PostIdx],
			UserID:    userIDs[comment.UserIdx],
			Text:      comment.Text,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := repo.Create(ctx, commentModel); err != nil {
			return nil, fmt.Errorf("failed to create comment: %w", err)
		}

		commentIDs = append(commentIDs, commentModel.ID)
	}

	return commentIDs, nil
}

func seedDemoLikes(ctx context.Context, db *database.DB, postIDs []string, userIDs []string) error {
	// Add likes to first 10 posts
	for i := 0; i < 10 && i < len(postIDs); i++ {
		// Each post gets liked by 3-7 users
		numLikes := 3 + (i % 5)
		for j := 0; j < numLikes && j < len(userIDs); j++ {
			query := `
				INSERT INTO post_likes (id, user_id, post_id, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING
			`
			_, _ = db.Pool.Exec(ctx, query,
				uuid.New().String(),
				userIDs[j],
				postIDs[i],
				time.Now(),
			)
		}
	}

	return nil
}

func seedDemoEventInterests(ctx context.Context, repo repositories.EventRepository, postIDs []string, userIDs []string) error {
	// EVENT posts are indices 5-9 in our seed data
	eventPosts := []int{5, 6, 7, 8, 9}

	for _, eventIdx := range eventPosts {
		if eventIdx >= len(postIDs) {
			continue
		}

		now := time.Now()

		// Add interested users
		for j := 0; j < 5 && j < len(userIDs); j++ {
			interest := &models.EventInterest{
				ID:         uuid.New().String(),
				PostID:     postIDs[eventIdx],
				UserID:     userIDs[j],
				EventState: models.EventInterestInterested,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			_ = repo.SetInterest(ctx, interest)
		}

		// Add going users
		for j := 5; j < 8 && j < len(userIDs); j++ {
			interest := &models.EventInterest{
				ID:         uuid.New().String(),
				PostID:     postIDs[eventIdx],
				UserID:     userIDs[j],
				EventState: models.EventInterestGoing,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			_ = repo.SetInterest(ctx, interest)
		}
	}

	return nil
}

func seedDemoRelationships(ctx context.Context, repo repositories.RelationshipsRepository, userIDs []string) error {
	// Create some follow relationships
	relationships := []struct {
		FollowerIdx int
		FolloweeIdx int
	}{
		{1, 2}, {1, 3}, {1, 4},
		{2, 1}, {2, 3}, {2, 5},
		{3, 1}, {3, 2}, {3, 4}, {3, 6},
		{4, 1}, {4, 3}, {4, 7},
		{5, 1}, {5, 2}, {5, 8},
		{6, 3}, {6, 9},
		{7, 4}, {7, 10},
	}

	for _, rel := range relationships {
		if rel.FollowerIdx >= len(userIDs) || rel.FolloweeIdx >= len(userIDs) {
			continue
		}

		_ = repo.FollowUser(ctx, userIDs[rel.FollowerIdx], userIDs[rel.FolloweeIdx])
	}

	return nil
}

func seedDemoReports(ctx context.Context, repo repositories.ReportRepository, postIDs []string, commentIDs []string, userIDs []string, businessIDs []string) error {
	now := time.Now()

	// Seed Post Reports
	postReports := []struct {
		PostIdx            int
		ReporterIdx        int
		Reason             string
		AdditionalComments string
		Status             models.ReportStatus
	}{
		{0, 5, "Spam", "This post appears to be spam content", models.ReportStatusPending},
		{2, 6, "Inappropriate content", "Contains offensive language", models.ReportStatusReviewing},
		{10, 7, "Misleading information", "False advertising for the product", models.ReportStatusResolved},
		{12, 8, "Harassment", "Post targets a specific individual", models.ReportStatusRejected},
		{15, 9, "Spam", "Repetitive promotional content", models.ReportStatusPending},
	}

	for _, report := range postReports {
		if report.PostIdx >= len(postIDs) || report.ReporterIdx >= len(userIDs) {
			continue
		}

		additionalComments := report.AdditionalComments
		postReport := &models.PostReport{
			ID:                 uuid.New().String(),
			UserID:             userIDs[report.ReporterIdx],
			PostID:             postIDs[report.PostIdx],
			Reason:             report.Reason,
			AdditionalComments: &additionalComments,
			ReportStatus:       report.Status,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		if err := repo.CreatePostReport(ctx, postReport); err != nil {
			return fmt.Errorf("failed to create post report: %w", err)
		}
	}

	// Seed Comment Reports
	commentReports := []struct {
		CommentIdx         int
		ReporterIdx        int
		Reason             string
		AdditionalComments string
		Status             models.ReportStatus
	}{
		{0, 4, "Harassment", "Rude and disrespectful comment", models.ReportStatusPending},
		{2, 5, "Spam", "Comment contains spam links", models.ReportStatusReviewing},
		{5, 6, "Inappropriate content", "Contains offensive language", models.ReportStatusResolved},
	}

	for _, report := range commentReports {
		if report.CommentIdx >= len(commentIDs) || report.ReporterIdx >= len(userIDs) {
			continue
		}

		additionalComments := report.AdditionalComments
		commentReport := &models.CommentReport{
			ID:                 uuid.New().String(),
			UserID:             userIDs[report.ReporterIdx],
			CommentID:          commentIDs[report.CommentIdx],
			Reason:             report.Reason,
			AdditionalComments: &additionalComments,
			ReportStatus:       report.Status,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		if err := repo.CreateCommentReport(ctx, commentReport); err != nil {
			return fmt.Errorf("failed to create comment report: %w", err)
		}
	}

	// Seed User Reports
	userReports := []struct {
		ReportedUserIdx int
		ReporterIdx     int
		Reason          string
		Description     string
		Resolved        bool
	}{
		{11, 3, "Harassment", "User has been sending threatening messages", false},
		{12, 4, "Fake account", "This appears to be an impersonation account", false},
		{13, 5, "Spam", "User is posting excessive promotional content", true},
	}

	for _, report := range userReports {
		if report.ReportedUserIdx >= len(userIDs) || report.ReporterIdx >= len(userIDs) {
			continue
		}

		description := report.Description
		userReport := &models.UserReport{
			ID:           uuid.New().String(),
			ReportedUser: userIDs[report.ReportedUserIdx],
			ReportedByID: userIDs[report.ReporterIdx],
			Reason:       report.Reason,
			Description:  &description,
			Resolved:     report.Resolved,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := repo.CreateUserReport(ctx, userReport); err != nil {
			return fmt.Errorf("failed to create user report: %w", err)
		}
	}

	// Seed Business Reports
	businessReports := []struct {
		BusinessIdx        int
		ReporterIdx        int
		Reason             string
		AdditionalComments string
		Status             models.ReportStatus
	}{
		{2, 10, "Fake business", "Business does not exist at this location", models.ReportStatusPending},
		{5, 11, "Misleading information", "Business hours are incorrect", models.ReportStatusReviewing},
		{7, 12, "Inappropriate content", "Business profile contains offensive images", models.ReportStatusResolved},
	}

	for _, report := range businessReports {
		if report.BusinessIdx >= len(businessIDs) || report.ReporterIdx >= len(userIDs) {
			continue
		}

		additionalComments := report.AdditionalComments
		businessReport := &models.BusinessReport{
			ID:                 uuid.New().String(),
			BusinessID:         businessIDs[report.BusinessIdx],
			UserID:             userIDs[report.ReporterIdx],
			Reason:             report.Reason,
			AdditionalComments: &additionalComments,
			ReportStatus:       report.Status,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		if err := repo.CreateBusinessReport(ctx, businessReport); err != nil {
			return fmt.Errorf("failed to create business report: %w", err)
		}
	}

	return nil
}
