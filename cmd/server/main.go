package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	_ "github.com/hamsaya/backend/docs" // Import swagger docs
	"github.com/hamsaya/backend/internal/handlers"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/notification"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Hamsaya Backend API
// @version 1.0
// @description A production-ready Go backend for a social media mobile application with posts, business profiles, marketplace, real-time chat, and location services.
// @termsOfService https://hamsaya.app/terms/

// @contact.name API Support
// @contact.url https://hamsaya.app/support
// @contact.email support@hamsaya.app

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @schemes http https
// @accept json
// @produce json
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

	logger := utils.GetBaseLogger()
	sugaredLogger := utils.GetLogger()
	sugaredLogger.Info("Starting Hamsaya Backend API...")
	sugaredLogger.Infow("Configuration loaded",
		"env", cfg.Server.Env,
		"port", cfg.Server.Port,
	)

	// Initialize validator
	validator := utils.NewValidator()

	// Connect to database
	sugaredLogger.Info("Connecting to database...")
	db, err := database.New(&cfg.Database)
	if err != nil {
		sugaredLogger.Fatalw("Failed to connect to database", "error", err)
	}
	defer db.Close()
	sugaredLogger.Info("Database connected successfully")

	// Connect to Redis
	sugaredLogger.Info("Connecting to Redis...")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		sugaredLogger.Fatalw("Failed to connect to Redis", "error", err)
	}
	sugaredLogger.Info("Redis connected successfully")

	// Initialize WebSocket hub
	sugaredLogger.Info("Initializing WebSocket hub...")
	wsHub := websocket.NewHub(logger)
	go wsHub.Run()
	sugaredLogger.Info("WebSocket hub started")

	// Initialize Firebase Cloud Messaging (optional - only if credentials are provided)
	var fcmClient *notification.FCMClient
	if cfg.Firebase.CredentialsPath != "" {
		sugaredLogger.Info("Initializing Firebase Cloud Messaging...")
		fcmClient, err = notification.NewFCMClient(cfg.Firebase.CredentialsPath, logger)
		if err != nil {
			sugaredLogger.Warnw("Failed to initialize FCM client (push notifications will be disabled)", "error", err)
			fcmClient = nil
		} else {
			sugaredLogger.Info("FCM client initialized successfully")
		}
	} else {
		sugaredLogger.Info("Firebase credentials not provided, push notifications will be disabled")
	}

	// Initialize repositories
	sugaredLogger.Info("Initializing repositories...")
	userRepo := repositories.NewUserRepository(db)
	mfaRepo := repositories.NewMFARepository(db)
	relationshipsRepo := repositories.NewRelationshipsRepository(db)
	postRepo := repositories.NewPostRepository(db)
	commentRepo := repositories.NewCommentRepository(db)
	pollRepo := repositories.NewPollRepository(db)
	eventRepo := repositories.NewEventRepository(db)
	businessRepo := repositories.NewBusinessRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)
	conversationRepo := repositories.NewConversationRepository(db)
	messageRepo := repositories.NewMessageRepository(db)
	notificationRepo := repositories.NewNotificationRepository(db)
	notificationSettingsRepo := repositories.NewNotificationSettingsRepository(db)
	searchRepo := repositories.NewSearchRepository(db)
	reportRepo := repositories.NewReportRepository(db)
	adminRepo := repositories.NewAdminRepository(db)

	// Initialize services
	sugaredLogger.Info("Initializing services...")
	jwtService := services.NewJWTService(&cfg.JWT)
	passwordService := services.NewPasswordService()
	emailService := services.NewEmailService(&cfg.Email, logger)
	tokenStorage := services.NewTokenStorageService(redisClient, logger)
	mfaService := services.NewMFAService(mfaRepo, userRepo, passwordService, logger)
	oauthService := services.NewOAuthService(cfg, userRepo, logger)
	storageService := services.NewStorageService(cfg, logger)
	profileService := services.NewProfileService(userRepo, postRepo, relationshipsRepo, logger)
	relationshipsService := services.NewRelationshipsService(relationshipsRepo, userRepo, logger)
	businessService := services.NewBusinessService(businessRepo, userRepo, logger)
	categoryService := services.NewCategoryService(categoryRepo, logger)
	postService := services.NewPostService(postRepo, pollRepo, userRepo, businessRepo, categoryRepo, logger)
	commentService := services.NewCommentService(commentRepo, postRepo, userRepo, logger)
	pollService := services.NewPollService(pollRepo, postRepo, logger)
	eventService := services.NewEventService(eventRepo, postRepo, userRepo, logger)
	authService := services.NewAuthService(userRepo, passwordService, jwtService, emailService, tokenStorage, mfaService, cfg, logger)
	chatService := services.NewChatService(conversationRepo, messageRepo, userRepo, wsHub, logger)
	notificationService := services.NewNotificationService(notificationRepo, notificationSettingsRepo, fcmClient, redisClient, logger)
	searchService := services.NewSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relationshipsRepo, logger)
	reportService := services.NewReportService(reportRepo, postRepo, userRepo, validator)
	adminService := services.NewAdminService(adminRepo, logger)

	// Initialize middleware
	sugaredLogger.Info("Initializing middleware...")
	authMiddleware := middleware.NewAuthMiddleware(jwtService, userRepo, logger)
	rateLimiter := middleware.NewRateLimiter(redisClient, logger)

	// Set Gin mode
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(sugaredLogger))
	router.Use(middleware.CORS(cfg.CORS))
	router.Use(middleware.RequestID())

	// Initialize handlers
	sugaredLogger.Info("Initializing handlers...")
	healthHandler := handlers.NewHealthHandler(db, redisClient)
	authHandler := handlers.NewAuthHandler(authService, validator, logger)
	mfaHandler := handlers.NewMFAHandler(mfaService, validator, logger)
	oauthHandler := handlers.NewOAuthHandler(authService, oauthService, validator, logger)
	profileHandler := handlers.NewProfileHandler(profileService, storageService, validator, logger)
	relationshipsHandler := handlers.NewRelationshipsHandler(relationshipsService, logger)
	postHandler := handlers.NewPostHandler(postService, storageService, validator, logger)
	commentHandler := handlers.NewCommentHandler(commentService, validator, logger)
	pollHandler := handlers.NewPollHandler(pollService, validator, logger)
	eventHandler := handlers.NewEventHandler(eventService, validator, logger)
	businessHandler := handlers.NewBusinessHandler(businessService, validator, logger)
	categoryHandler := handlers.NewCategoryHandler(categoryService, validator, logger)
	chatHandler := handlers.NewChatHandler(chatService, wsHub, validator, logger, cfg)
	notificationHandler := handlers.NewNotificationHandler(notificationService, validator, logger)
	searchHandler := handlers.NewSearchHandler(searchService, validator, logger)
	reportHandler := handlers.NewReportHandler(reportService)
	adminHandler := handlers.NewAdminHandler(adminService, logger)

	// Health check routes (no versioning)
	router.GET("/health", healthHandler.Health)
	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)
	router.GET("/health/startup", healthHandler.Startup)
	router.GET("/health/db-stats", healthHandler.DBStats)
	router.GET("/health/redis-stats", healthHandler.RedisStats)
	router.GET("/health/version", healthHandler.Version)
	router.GET("/health/metrics", healthHandler.Metrics)

	// Swagger API documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	sugaredLogger.Info("Swagger UI available at /swagger/index.html")

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public auth routes (with rate limiting)
		auth := v1.Group("/auth")
		{
			// Registration and login with strict rate limiting
			auth.POST("/register", rateLimiter.LimitAuth(), authHandler.Register)
			auth.POST("/login", rateLimiter.LimitLoginAttempts(), authHandler.Login)
			auth.POST("/unified", rateLimiter.LimitLoginAttempts(), authHandler.UnifiedAuth)
			auth.POST("/refresh", rateLimiter.LimitAuth(), authHandler.RefreshToken)

			// Email and password flows
			auth.POST("/verify-email", rateLimiter.LimitAuth(), authHandler.VerifyEmail)
			auth.POST("/forgot-password", rateLimiter.LimitStrict(), authHandler.ForgotPassword)
			auth.POST("/reset-password", rateLimiter.LimitAuth(), authHandler.ResetPassword)

			// MFA verification
			auth.POST("/mfa/verify", rateLimiter.LimitAuth(), authHandler.VerifyMFA)

			// OAuth authentication
			auth.POST("/oauth/google", rateLimiter.LimitAuth(), oauthHandler.GoogleOAuth)
			auth.POST("/oauth/facebook", rateLimiter.LimitAuth(), oauthHandler.FacebookOAuth)
			auth.POST("/oauth/apple", rateLimiter.LimitAuth(), oauthHandler.AppleOAuth)

			// Protected auth routes (require authentication)
			auth.POST("/logout", authMiddleware.RequireAuth(), authHandler.Logout)
			auth.POST("/logout-all", authMiddleware.RequireAuth(), authHandler.LogoutAll)
			auth.POST("/change-password", authMiddleware.RequireAuth(), authHandler.ChangePassword)
			auth.GET("/sessions", authMiddleware.RequireAuth(), authHandler.GetActiveSessions)
		}

		// MFA routes (require authentication)
		mfaHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())

		// Profile routes
		users := v1.Group("/users")
		{
			// Protected routes (require authentication)
			users.GET("/me", authMiddleware.RequireAuth(), profileHandler.GetMyProfile)
			users.PUT("/me", authMiddleware.RequireAuth(), profileHandler.UpdateProfile)
			users.POST("/me/avatar", authMiddleware.RequireAuth(), profileHandler.UploadAvatar)
			users.DELETE("/me/avatar", authMiddleware.RequireAuth(), profileHandler.DeleteAvatar)
			users.POST("/me/cover", authMiddleware.RequireAuth(), profileHandler.UploadCover)
			users.DELETE("/me/cover", authMiddleware.RequireAuth(), profileHandler.DeleteCover)

			// Public routes (optional auth for relationship status)
			users.GET("/:user_id", authMiddleware.OptionalAuth(), profileHandler.GetUserProfile)

			// Relationship routes (require authentication)
			users.POST("/:user_id/follow", authMiddleware.RequireAuth(), relationshipsHandler.FollowUser)
			users.DELETE("/:user_id/follow", authMiddleware.RequireAuth(), relationshipsHandler.UnfollowUser)
			users.GET("/:user_id/followers", authMiddleware.OptionalAuth(), relationshipsHandler.GetFollowers)
			users.GET("/:user_id/following", authMiddleware.OptionalAuth(), relationshipsHandler.GetFollowing)
			users.POST("/:user_id/block", authMiddleware.RequireAuth(), relationshipsHandler.BlockUser)
			users.DELETE("/:user_id/block", authMiddleware.RequireAuth(), relationshipsHandler.UnblockUser)
			users.GET("/blocked", authMiddleware.RequireAuth(), relationshipsHandler.GetBlockedUsers)
			users.GET("/:user_id/relationship", authMiddleware.RequireAuth(), relationshipsHandler.GetRelationshipStatus)

			// User reporting (require authentication + rate limiting)
			users.POST("/:user_id/report", authMiddleware.RequireAuth(), rateLimiter.LimitReports(), reportHandler.ReportUser)
		}

		// Post routes
		posts := v1.Group("/posts")
		{
			// Public routes (optional auth for engagement status)
			posts.GET("", authMiddleware.OptionalAuth(), postHandler.GetFeed)
			posts.GET("/:post_id", authMiddleware.OptionalAuth(), postHandler.GetPost)

			// Protected routes (require authentication)
			posts.POST("", authMiddleware.RequireAuth(), postHandler.CreatePost)
			posts.POST("/upload-image", authMiddleware.RequireAuth(), postHandler.UploadPostImage)
			posts.PUT("/:post_id", authMiddleware.RequireAuth(), postHandler.UpdatePost)
			posts.DELETE("/:post_id", authMiddleware.RequireAuth(), postHandler.DeletePost)

			// Post interactions
			posts.POST("/:post_id/like", authMiddleware.RequireAuth(), postHandler.LikePost)
			posts.DELETE("/:post_id/like", authMiddleware.RequireAuth(), postHandler.UnlikePost)
			posts.POST("/:post_id/bookmark", authMiddleware.RequireAuth(), postHandler.BookmarkPost)
			posts.DELETE("/:post_id/bookmark", authMiddleware.RequireAuth(), postHandler.UnbookmarkPost)
			posts.POST("/:post_id/share", authMiddleware.RequireAuth(), postHandler.SharePost)
			posts.POST("/:post_id/report", authMiddleware.RequireAuth(), rateLimiter.LimitReports(), reportHandler.ReportPost)

			// Comment routes
			posts.GET("/:post_id/comments", authMiddleware.OptionalAuth(), commentHandler.GetPostComments)
			posts.POST("/:post_id/comments", authMiddleware.RequireAuth(), commentHandler.CreateComment)

			// Poll routes
			posts.GET("/:post_id/polls", authMiddleware.OptionalAuth(), pollHandler.GetPostPoll)
			posts.POST("/:post_id/polls", authMiddleware.RequireAuth(), pollHandler.CreatePoll)
		}

		// Comment routes
		comments := v1.Group("/comments")
		{
			comments.GET("/:comment_id", authMiddleware.OptionalAuth(), commentHandler.GetComment)
			comments.PUT("/:comment_id", authMiddleware.RequireAuth(), commentHandler.UpdateComment)
			comments.DELETE("/:comment_id", authMiddleware.RequireAuth(), commentHandler.DeleteComment)
			comments.GET("/:comment_id/replies", authMiddleware.OptionalAuth(), commentHandler.GetCommentReplies)
			comments.POST("/:comment_id/like", authMiddleware.RequireAuth(), commentHandler.LikeComment)
			comments.DELETE("/:comment_id/like", authMiddleware.RequireAuth(), commentHandler.UnlikeComment)
			comments.POST("/:comment_id/report", authMiddleware.RequireAuth(), rateLimiter.LimitReports(), reportHandler.ReportComment)
		}

		// Poll routes
		polls := v1.Group("/polls")
		{
			polls.GET("/:poll_id", authMiddleware.OptionalAuth(), pollHandler.GetPoll)
			polls.POST("/:poll_id/vote", authMiddleware.RequireAuth(), pollHandler.VotePoll)
			polls.DELETE("/:poll_id/vote", authMiddleware.RequireAuth(), pollHandler.DeleteVote)
		}

		// Event routes
		events := v1.Group("/events")
		{
			events.GET("/:post_id/interest", authMiddleware.OptionalAuth(), eventHandler.GetEventInterestStatus)
			events.POST("/:post_id/interest", authMiddleware.RequireAuth(), eventHandler.SetEventInterest)
			events.DELETE("/:post_id/interest", authMiddleware.RequireAuth(), eventHandler.RemoveEventInterest)
			events.GET("/:post_id/interested", authMiddleware.OptionalAuth(), eventHandler.GetInterestedUsers)
			events.GET("/:post_id/going", authMiddleware.OptionalAuth(), eventHandler.GetGoingUsers)
		}

		// User posts and bookmarks (already defined in users group above)
		users.GET("/me/posts", authMiddleware.RequireAuth(), postHandler.GetMyPosts)
		users.GET("/me/bookmarks", authMiddleware.RequireAuth(), postHandler.GetMyBookmarks)

		// Business routes
		businesses := v1.Group("/businesses")
		{
			// Public routes (optional auth for personalization)
			businesses.GET("/search", authMiddleware.OptionalAuth(), businessHandler.ListBusinesses)
			businesses.GET("/categories", authMiddleware.OptionalAuth(), businessHandler.GetCategories)
			businesses.GET("/:business_id", authMiddleware.OptionalAuth(), businessHandler.GetBusiness)

			// Protected routes (require authentication)
			businesses.GET("", authMiddleware.RequireAuth(), businessHandler.GetMyBusinesses)
			businesses.POST("", authMiddleware.RequireAuth(), businessHandler.CreateBusiness)
			businesses.PUT("/:business_id", authMiddleware.RequireAuth(), businessHandler.UpdateBusiness)
			businesses.DELETE("/:business_id", authMiddleware.RequireAuth(), businessHandler.DeleteBusiness)

			// Business media
			businesses.POST("/:business_id/avatar", authMiddleware.RequireAuth(), businessHandler.UploadAvatar)
			businesses.POST("/:business_id/cover", authMiddleware.RequireAuth(), businessHandler.UploadCover)
			businesses.POST("/:business_id/attachments", authMiddleware.RequireAuth(), businessHandler.AddGalleryImage)
			businesses.DELETE("/:business_id/attachments/:attachment_id", authMiddleware.RequireAuth(), businessHandler.DeleteGalleryImage)

			// Business hours
			businesses.POST("/:business_id/hours", authMiddleware.RequireAuth(), businessHandler.SetBusinessHours)

			// Business following
			businesses.POST("/:business_id/follow", authMiddleware.RequireAuth(), businessHandler.FollowBusiness)
			businesses.DELETE("/:business_id/follow", authMiddleware.RequireAuth(), businessHandler.UnfollowBusiness)

			// Business reporting (require authentication + rate limiting)
			businesses.POST("/:business_id/report", authMiddleware.RequireAuth(), rateLimiter.LimitReports(), reportHandler.ReportBusiness)
		}

		// Category routes (marketplace categories)
		categories := v1.Group("/categories")
		{
			// Public routes (optional auth for future personalization)
			categories.GET("", authMiddleware.OptionalAuth(), categoryHandler.ListCategories)
			categories.GET("/:category_id", authMiddleware.OptionalAuth(), categoryHandler.GetCategory)
		}

		// Admin category routes (require admin role)
		adminCategories := v1.Group("/admin/categories")
		adminCategories.Use(authMiddleware.RequireAdmin())
		{
			adminCategories.GET("", categoryHandler.GetAllCategories)
			adminCategories.POST("", categoryHandler.CreateCategory)
			adminCategories.PUT("/:category_id", categoryHandler.UpdateCategory)
			adminCategories.DELETE("/:category_id", categoryHandler.DeleteCategory)
		}

		// Admin report management routes (require admin role)
		adminReports := v1.Group("/admin/reports")
		adminReports.Use(authMiddleware.RequireAdmin())
		{
			// Post reports
			adminReports.GET("/posts", reportHandler.ListPostReports)
			adminReports.GET("/posts/:id", reportHandler.GetPostReport)
			adminReports.PUT("/posts/:id/status", reportHandler.UpdatePostReportStatus)

			// Comment reports
			adminReports.GET("/comments", reportHandler.ListCommentReports)
			adminReports.GET("/comments/:id", reportHandler.GetCommentReport)
			adminReports.PUT("/comments/:id/status", reportHandler.UpdateCommentReportStatus)

			// User reports
			adminReports.GET("/users", reportHandler.ListUserReports)
			adminReports.GET("/users/:id", reportHandler.GetUserReport)
			adminReports.PUT("/users/:id/status", reportHandler.UpdateUserReportStatus)

			// Business reports
			adminReports.GET("/businesses", reportHandler.ListBusinessReports)
			adminReports.GET("/businesses/:id", reportHandler.GetBusinessReport)
			adminReports.PUT("/businesses/:id/status", reportHandler.UpdateBusinessReportStatus)
		}

		// Admin auth routes (public, no auth middleware)
		adminAuth := v1.Group("/admin/auth")
		{
			adminAuth.POST("/login", authHandler.AdminLogin)
		}

		// Admin dashboard routes (require admin role)
		admin := v1.Group("/admin")
		admin.Use(authMiddleware.RequireAdmin())
		{
			admin.GET("/statistics", adminHandler.GetStatistics)
			admin.GET("/users", adminHandler.ListUsers)
			admin.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
			admin.PUT("/users/:id", adminHandler.UpdateUser)
			admin.GET("/posts", adminHandler.ListPosts)
			admin.GET("/posts/sell/statistics", adminHandler.GetSellPostStatistics)
			admin.PUT("/posts/:id/status", adminHandler.UpdatePostStatus)
			admin.PUT("/posts/:id", adminHandler.UpdatePost)
			admin.GET("/reports", adminHandler.ListReports)
			admin.PUT("/reports/:type/:id/status", adminHandler.UpdateReportStatus)
			admin.GET("/businesses", adminHandler.ListBusinesses)
			admin.PUT("/businesses/:id/status", adminHandler.UpdateBusinessStatus)
			admin.PUT("/businesses/:id", adminHandler.UpdateBusiness)
		}

		// Chat routes (require authentication)
		chat := v1.Group("/chat")
		chat.Use(authMiddleware.RequireAuth())
		{
			// WebSocket endpoint for real-time chat
			chat.GET("/ws", chatHandler.HandleWebSocket)

			// HTTP endpoints for chat
			chat.POST("/messages", chatHandler.SendMessage)
			chat.GET("/conversations", chatHandler.GetConversations)
			chat.GET("/conversations/:conversation_id/messages", chatHandler.GetMessages)
			chat.POST("/conversations/:conversation_id/read", chatHandler.MarkConversationAsRead)
			chat.DELETE("/messages/:message_id", chatHandler.DeleteMessage)
		}

		// Notification routes (require authentication)
		notifications := v1.Group("/notifications")
		notifications.Use(authMiddleware.RequireAuth())
		{
			// Notification management
			notifications.GET("", notificationHandler.GetNotifications)
			notifications.GET("/unread-count", notificationHandler.GetUnreadCount)
			notifications.POST("/:notification_id/read", notificationHandler.MarkAsRead)
			notifications.POST("/read-all", notificationHandler.MarkAllAsRead)
			notifications.DELETE("/:notification_id", notificationHandler.DeleteNotification)

			// Notification settings
			notifications.GET("/settings", notificationHandler.GetNotificationSettings)
			notifications.PUT("/settings", notificationHandler.UpdateNotificationSetting)

			// FCM token registration
			notifications.POST("/fcm-token", notificationHandler.RegisterFCMToken)
			notifications.DELETE("/fcm-token", notificationHandler.UnregisterFCMToken)
		}

		// Search routes (public, but optional auth for personalized results)
		v1.GET("/search", authMiddleware.OptionalAuth(), searchHandler.Search)
		v1.GET("/search/posts", authMiddleware.OptionalAuth(), searchHandler.SearchPosts)
		v1.GET("/search/users", authMiddleware.OptionalAuth(), searchHandler.SearchUsers)
		v1.GET("/search/businesses", authMiddleware.OptionalAuth(), searchHandler.SearchBusinesses)

		// Discovery routes (map-based, public, optional auth for personalization)
		v1.GET("/discover", authMiddleware.OptionalAuth(), searchHandler.Discover)

		// Placeholder for future routes
		v1.GET("/ping", func(c *gin.Context) {
			utils.SendSuccess(c, http.StatusOK, "pong", gin.H{
				"version": "1.0.0",
				"service": "hamsaya-backend",
			})
		})
	}

	sugaredLogger.Info("Routes registered successfully")

	// Create HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in a goroutine
	go func() {
		sugaredLogger.Infow("Starting HTTP server", "address", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugaredLogger.Fatalw("Failed to start server", "error", err)
		}
	}()

	sugaredLogger.Infow("Server started successfully",
		"address", addr,
		"env", cfg.Server.Env,
	)

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	sugaredLogger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		sugaredLogger.Fatalw("Server forced to shutdown", "error", err)
	}

	sugaredLogger.Info("Server exited successfully")
}
