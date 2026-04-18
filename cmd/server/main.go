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
	"github.com/hamsaya/backend/pkg/observability"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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

	// Initialize OpenTelemetry observability (traces, metrics, logs) with fallback to no-op
	otelCfg := observability.Config{
		ServiceName:    "hamsaya-backend",
		ServiceVersion: "1.0.0",
		Environment:    cfg.Server.Env,
		OTLPEndpoint:   cfg.Monitoring.OTLPEndpoint,
		SamplingRate:   cfg.Monitoring.TraceSamplingRate,
		Enabled:        cfg.Monitoring.ObservabilityEnabled,
	}

	var telem observability.TelemetryProvider
	telem, err = observability.NewTelemetry(context.Background(), otelCfg, logger)
	if err != nil {
		sugaredLogger.Warnw("OpenTelemetry init failed, falling back to no-op", "error", err)
		telem = observability.NewNoopTelemetry(otelCfg, logger)
	} else if telem == nil {
		telem = observability.NewNoopTelemetry(otelCfg, logger)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := telem.Shutdown(shutdownCtx); err != nil {
			sugaredLogger.Errorw("Failed to shutdown observability", "error", err)
		}
	}()

	// Optional: wrap logger so logs are also exported via OTLP when endpoint is set
	if stack := telem.Stack(); stack != nil {
		logger = stack.WrapLoggerWithOTel(logger, "hamsaya-backend")
		sugaredLogger = logger.Sugar()
	}

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
	fcmCfg := notification.FCMConfig{
		CredentialsPath: cfg.Firebase.CredentialsPath,
		ProjectID:       cfg.Firebase.ProjectID,
		PrivateKey:      cfg.Firebase.PrivateKey,
		ClientEmail:     cfg.Firebase.ClientEmail,
	}
	if fcmCfg.CredentialsPath != "" || (fcmCfg.ProjectID != "" && fcmCfg.PrivateKey != "" && fcmCfg.ClientEmail != "") {
		sugaredLogger.Info("Initializing Firebase Cloud Messaging...")
		fcmClient, err = notification.NewFCMClient(fcmCfg, logger)
		if err != nil {
			sugaredLogger.Warnw("Failed to initialize FCM client (push notifications will be disabled)", "error", err)
			fcmClient = nil
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
	feedbackRepo := repositories.NewFeedbackRepository(db)
	adminRepo := repositories.NewAdminRepository(db)
	fanoutRepo := repositories.NewFanoutRepository(db)

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
	notificationService := services.NewNotificationService(notificationRepo, notificationSettingsRepo, userRepo, fcmClient, redisClient, wsHub, logger)
	relationshipsService := services.NewRelationshipsService(relationshipsRepo, userRepo, notificationService, logger)
	businessService := services.NewBusinessService(businessRepo, userRepo, notificationService, logger)
	categoryService := services.NewCategoryService(categoryRepo, logger)
	fanoutService := services.NewFanoutService(fanoutRepo, logger)
	postService := services.NewPostService(postRepo, pollRepo, userRepo, businessRepo, relationshipsRepo, categoryRepo, eventRepo, notificationService, fanoutService, fanoutRepo, cfg.Storage.BucketName, logger)
	commentService := services.NewCommentService(commentRepo, postRepo, userRepo, businessRepo, notificationService, logger)
	pollService := services.NewPollService(pollRepo, postRepo, userRepo, notificationService, logger)
	eventService := services.NewEventService(eventRepo, postRepo, userRepo, notificationService, logger)
	authService := services.NewAuthService(userRepo, passwordService, jwtService, emailService, tokenStorage, mfaService, cfg, logger)
	chatService := services.NewChatService(conversationRepo, messageRepo, userRepo, wsHub, logger)
	searchService := services.NewSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relationshipsRepo, logger)
	reportService := services.NewReportService(reportRepo, postRepo, userRepo, validator)
	feedbackService := services.NewFeedbackService(feedbackRepo, validator)
	adminService := services.NewAdminService(adminRepo, fcmClient, notificationService, logger)

	// Initialize middleware
	sugaredLogger.Info("Initializing middleware...")
	authMiddleware := middleware.NewAuthMiddleware(jwtService, userRepo, tokenStorage, logger)
	// verifiedAuth requires email verification; use for create/update/delete (post, comment, follow, etc.)
	verifiedAuth := authMiddleware.RequireVerifiedEmail()
	rateLimiter := middleware.NewRateLimiter(redisClient, logger)

	// Set Gin mode
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Set max multipart memory (10 MB for file uploads)
	router.MaxMultipartMemory = 10 << 20

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(sugaredLogger))
	router.Use(middleware.CORS(cfg.CORS))
	router.Use(middleware.RequestID())
	router.Use(middleware.SecurityHeaders())

	// OpenTelemetry: in-flight count, request duration/count metrics, and tracing
	router.Use(telem.MeterRequestsInFlight())
	router.Use(telem.MeterRequestDuration())
	if telem.Stack() != nil {
		router.Use(otelgin.Middleware("hamsaya-backend"))
		sugaredLogger.Info("OpenTelemetry tracing and metrics middleware enabled")
	}

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
	businessHandler := handlers.NewBusinessHandler(businessService, storageService, validator, logger)
	categoryHandler := handlers.NewCategoryHandler(categoryService, validator, logger)
	chatHandler := handlers.NewChatHandler(chatService, wsHub, validator, logger, cfg)
	notificationHandler := handlers.NewNotificationHandler(notificationService, validator, logger)
	searchHandler := handlers.NewSearchHandler(searchService, validator, logger)
	reportHandler := handlers.NewReportHandler(reportService)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackService)
	adminHandler := handlers.NewAdminHandler(adminService, validator, logger)

	// Health check routes (no versioning)
	router.GET("/health", healthHandler.Health)
	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)
	router.GET("/health/startup", healthHandler.Startup)
	router.GET("/health/db-stats", healthHandler.DBStats)
	router.GET("/health/redis-stats", healthHandler.RedisStats)
	router.GET("/health/version", healthHandler.Version)
	router.GET("/health/metrics", healthHandler.Metrics)

	// Prometheus metrics endpoint for scraping
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	sugaredLogger.Info("Prometheus metrics available at /metrics")

	// Swagger API documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	sugaredLogger.Info("Swagger UI available at /swagger/index.html")

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Explicit /users/me/* routes first so they always match (avoid 404 from param route)
		v1.GET("/users/me/posts", authMiddleware.RequireAuth(), postHandler.GetMyPosts)
		v1.GET("/users/me/bookmarks", authMiddleware.RequireAuth(), postHandler.GetMyBookmarks)
		v1.GET("/users/me/events", authMiddleware.RequireAuth(), postHandler.GetMyEvents)

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
			auth.POST("/verify-reset-code", rateLimiter.LimitPasswordReset(), authHandler.VerifyResetCode)
			auth.POST("/reset-password", rateLimiter.LimitPasswordReset(), authHandler.ResetPassword)

			// MFA verification
			auth.POST("/mfa/verify", rateLimiter.LimitAuth(), authHandler.VerifyMFA)

			// OAuth authentication
			auth.POST("/oauth/google", rateLimiter.LimitAuth(), oauthHandler.GoogleOAuth)
			auth.POST("/oauth/facebook", rateLimiter.LimitAuth(), oauthHandler.FacebookOAuth)
			auth.POST("/oauth/apple", rateLimiter.LimitAuth(), oauthHandler.AppleOAuth)

			// Protected auth routes (require authentication)
			auth.POST("/logout", authMiddleware.RequireAuth(), authHandler.Logout)
			auth.POST("/logout-all", authMiddleware.RequireAuth(), authHandler.LogoutAll)
			auth.POST("/send-verification-email", authMiddleware.RequireAuth(), authHandler.SendVerificationEmail)
			auth.POST("/change-password", authMiddleware.RequireAuth(), authHandler.ChangePassword)
			auth.GET("/sessions", authMiddleware.RequireAuth(), authHandler.GetActiveSessions)
		}

		// MFA routes (require authentication)
		mfaHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())

		// Profile routes
		users := v1.Group("/users")
		{
			// /me/posts, /me/bookmarks, /me/events are registered above on v1

			// Protected routes (require authentication)
			users.GET("/me", authMiddleware.RequireAuth(), profileHandler.GetMyProfile)
			users.PUT("/me", authMiddleware.RequireAuth(), profileHandler.UpdateProfile)
			users.DELETE("/me", authMiddleware.RequireAuth(), profileHandler.DeleteAccount)
			users.POST("/me/avatar", verifiedAuth, profileHandler.UploadAvatar)
			users.DELETE("/me/avatar", verifiedAuth, profileHandler.DeleteAvatar)
			users.POST("/me/cover", verifiedAuth, profileHandler.UploadCover)
			users.DELETE("/me/cover", verifiedAuth, profileHandler.DeleteCover)

			// Require auth for user profile and relationship views
			users.GET("/:user_id", authMiddleware.RequireAuth(), profileHandler.GetUserProfile)

			// Relationship routes (require authentication)
			users.POST("/:user_id/follow", verifiedAuth, relationshipsHandler.FollowUser)
			users.DELETE("/:user_id/follow", verifiedAuth, relationshipsHandler.UnfollowUser)
			users.GET("/:user_id/followers", authMiddleware.RequireAuth(), relationshipsHandler.GetFollowers)
			users.GET("/:user_id/following", authMiddleware.RequireAuth(), relationshipsHandler.GetFollowing)
			users.POST("/:user_id/block", verifiedAuth, relationshipsHandler.BlockUser)
			users.DELETE("/:user_id/block", verifiedAuth, relationshipsHandler.UnblockUser)
			users.GET("/blocked", authMiddleware.RequireAuth(), relationshipsHandler.GetBlockedUsers)
			users.GET("/:user_id/relationship", authMiddleware.RequireAuth(), relationshipsHandler.GetRelationshipStatus)

			// User reporting (require authentication + rate limiting)
			users.POST("/:user_id/report", verifiedAuth, rateLimiter.LimitReports(), reportHandler.ReportUser)
		}

		// Post routes
		posts := v1.Group("/posts")
		{
			// Require auth for feed and post detail so engagement fields are always user-scoped
			posts.GET("", authMiddleware.RequireAuth(), postHandler.GetFeed)
			// /posts/feed must be registered before /:post_id to avoid the param route catching it
			posts.GET("/feed", authMiddleware.RequireAuth(), postHandler.GetPersonalizedFeed)
			posts.GET("/:post_id", authMiddleware.RequireAuth(), postHandler.GetPost)

			// Protected routes (require verified email)
			posts.POST("", verifiedAuth, postHandler.CreatePost)
			posts.POST("/upload-image", verifiedAuth, postHandler.UploadPostImage)
			posts.PUT("/:post_id", verifiedAuth, postHandler.UpdatePost)
			posts.DELETE("/:post_id", verifiedAuth, postHandler.DeletePost)

			// Post interactions (require verified email)
			posts.POST("/:post_id/like", verifiedAuth, postHandler.LikePost)
			posts.DELETE("/:post_id/like", verifiedAuth, postHandler.UnlikePost)
			posts.POST("/:post_id/bookmark", verifiedAuth, postHandler.BookmarkPost)
			posts.DELETE("/:post_id/bookmark", verifiedAuth, postHandler.UnbookmarkPost)
			posts.POST("/:post_id/share", verifiedAuth, postHandler.SharePost)
			posts.POST("/:post_id/resell", verifiedAuth, postHandler.ResellPost)
			posts.POST("/:post_id/report", verifiedAuth, rateLimiter.LimitReports(), reportHandler.ReportPost)

			// Comment routes
			posts.GET("/:post_id/comments", authMiddleware.RequireAuth(), commentHandler.GetPostComments)
			posts.POST("/:post_id/comments", verifiedAuth, commentHandler.CreateComment)

			// Poll routes
			posts.GET("/:post_id/polls", authMiddleware.RequireAuth(), pollHandler.GetPostPoll)
			posts.POST("/:post_id/polls", verifiedAuth, pollHandler.CreatePoll)
		}

		// Comment routes
		comments := v1.Group("/comments")
		{
			comments.GET("/:comment_id", authMiddleware.RequireAuth(), commentHandler.GetComment)
			comments.PUT("/:comment_id", verifiedAuth, commentHandler.UpdateComment)
			comments.DELETE("/:comment_id", verifiedAuth, commentHandler.DeleteComment)
			comments.GET("/:comment_id/replies", authMiddleware.RequireAuth(), commentHandler.GetCommentReplies)
			comments.POST("/:comment_id/like", verifiedAuth, commentHandler.LikeComment)
			comments.DELETE("/:comment_id/like", verifiedAuth, commentHandler.UnlikeComment)
			comments.POST("/:comment_id/report", verifiedAuth, rateLimiter.LimitReports(), reportHandler.ReportComment)
		}

		// Poll routes
		polls := v1.Group("/polls")
		{
			polls.GET("/:poll_id", authMiddleware.RequireAuth(), pollHandler.GetPoll)
			polls.POST("/:poll_id/vote", verifiedAuth, pollHandler.VotePoll)
			polls.DELETE("/:poll_id/vote", verifiedAuth, pollHandler.DeleteVote)
		}

		// Event routes
		events := v1.Group("/events")
		{
			events.GET("/:post_id/interest", authMiddleware.RequireAuth(), eventHandler.GetEventInterestStatus)
			events.POST("/:post_id/interest", verifiedAuth, eventHandler.SetEventInterest)
			events.DELETE("/:post_id/interest", verifiedAuth, eventHandler.RemoveEventInterest)
			events.GET("/:post_id/interested", authMiddleware.RequireAuth(), eventHandler.GetInterestedUsers)
			events.GET("/:post_id/going", authMiddleware.RequireAuth(), eventHandler.GetGoingUsers)
		}

		// Business routes
		businesses := v1.Group("/businesses")
		{
			// Static and more specific routes first (before /:business_id)
			businesses.GET("/search", authMiddleware.RequireAuth(), businessHandler.ListBusinesses)
			businesses.GET("/categories", authMiddleware.RequireAuth(), businessHandler.GetCategories)
			businesses.GET("/:business_id/hours", businessHandler.GetBusinessHours)
			businesses.GET("/:business_id/attachments", authMiddleware.RequireAuth(), businessHandler.GetGallery)

			businesses.GET("/:business_id", authMiddleware.RequireAuth(), businessHandler.GetBusiness)

			// Protected routes (require verified email)
			businesses.GET("", authMiddleware.RequireAuth(), businessHandler.GetMyBusinesses)
			businesses.POST("", verifiedAuth, businessHandler.CreateBusiness)
			businesses.PUT("/:business_id", verifiedAuth, businessHandler.UpdateBusiness)
			businesses.DELETE("/:business_id", verifiedAuth, businessHandler.DeleteBusiness)

			// Business media (require verified email)
			businesses.POST("/:business_id/avatar", verifiedAuth, businessHandler.UploadAvatar)
			businesses.POST("/:business_id/cover", verifiedAuth, businessHandler.UploadCover)
			businesses.POST("/:business_id/attachments", verifiedAuth, businessHandler.AddGalleryImage)
			businesses.DELETE("/:business_id/attachments/:attachment_id", verifiedAuth, businessHandler.DeleteGalleryImage)

			// Business hours (POST requires verified email)
			businesses.POST("/:business_id/hours", verifiedAuth, businessHandler.SetBusinessHours)

			// Business following (require verified email)
			businesses.POST("/:business_id/follow", verifiedAuth, businessHandler.FollowBusiness)
			businesses.DELETE("/:business_id/follow", verifiedAuth, businessHandler.UnfollowBusiness)

			// Business reporting (require verified email + rate limiting)
			businesses.POST("/:business_id/report", verifiedAuth, rateLimiter.LimitReports(), reportHandler.ReportBusiness)
		}

		// Category routes (marketplace categories)
		categories := v1.Group("/categories")
		{
			categories.GET("", authMiddleware.RequireAuth(), categoryHandler.ListCategories)
			categories.GET("/:category_id", authMiddleware.RequireAuth(), categoryHandler.GetCategory)
		}

		// Chat routes (require verified email for sending messages, etc.)
		chat := v1.Group("/chat")
		chat.Use(verifiedAuth)
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

		// Notification routes (require auth for reads; verified email for writes)
		notifications := v1.Group("/notifications")
		{
			// Notification management (reads: auth only; writes: verified email)
			notifications.GET("", authMiddleware.RequireAuth(), notificationHandler.GetNotifications)
			notifications.GET("/unread-count", authMiddleware.RequireAuth(), notificationHandler.GetUnreadCount)
			notifications.POST("/:notification_id/read", verifiedAuth, notificationHandler.MarkAsRead)
			notifications.POST("/read-all", verifiedAuth, notificationHandler.MarkAllAsRead)
			notifications.DELETE("/:notification_id", verifiedAuth, notificationHandler.DeleteNotification)

			// Notification settings (read: auth; update: verified email)
			notifications.GET("/settings", authMiddleware.RequireAuth(), notificationHandler.GetNotificationSettings)
			notifications.PUT("/settings", verifiedAuth, notificationHandler.UpdateNotificationSetting)

			// FCM token registration (auth only — token must be registerable before email is verified)
			notifications.POST("/fcm-token", authMiddleware.RequireAuth(), notificationHandler.RegisterFCMToken)
			notifications.DELETE("/fcm-token", authMiddleware.RequireAuth(), notificationHandler.UnregisterFCMToken)
		}

		// Search and discovery routes (require auth)
		v1.GET("/search", authMiddleware.RequireAuth(), searchHandler.Search)
		v1.GET("/search/posts", authMiddleware.RequireAuth(), searchHandler.SearchPosts)
		v1.GET("/search/users", authMiddleware.RequireAuth(), searchHandler.SearchUsers)
		v1.GET("/search/businesses", authMiddleware.RequireAuth(), searchHandler.SearchBusinesses)
		v1.GET("/discover", authMiddleware.RequireAuth(), searchHandler.Discover)

		// Feedback routes (require verified email to submit)
		feedback := v1.Group("/feedback")
		{
			feedback.POST("", verifiedAuth, feedbackHandler.SubmitFeedback)
			feedback.GET("/status", authMiddleware.RequireAuth(), feedbackHandler.GetFeedbackStatus)
		}

		// Admin routes (require admin role)
		admin := v1.Group("/admin")
		admin.Use(authMiddleware.RequireAdmin())
		{
			// Dashboard & Analytics
			admin.GET("/stats", adminHandler.GetDashboardStats)
			admin.GET("/analytics/users", adminHandler.GetUserAnalytics)
			admin.GET("/analytics/posts", adminHandler.GetPostAnalytics)
			admin.GET("/analytics/engagement", adminHandler.GetEngagementAnalytics)
			admin.GET("/analytics/businesses", adminHandler.GetBusinessAnalytics)

			// User Management
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/users/:user_id", adminHandler.GetUser)
			admin.POST("/users/:user_id/suspend", adminHandler.SuspendUser)
			admin.POST("/users/:user_id/unsuspend", adminHandler.UnsuspendUser)
			admin.DELETE("/users/:user_id", adminHandler.DeleteUser)
			admin.PUT("/users/:user_id/role", adminHandler.UpdateUserRole)

			// Content Moderation
			admin.GET("/posts", adminHandler.ListAllPosts)
			admin.GET("/posts/:post_id", adminHandler.GetPostDetail)
			admin.DELETE("/posts/:post_id", adminHandler.DeletePost)
			admin.PUT("/posts/:post_id/status", adminHandler.UpdatePostStatus)
			admin.GET("/comments", adminHandler.ListAllComments)
			admin.GET("/comments/:comment_id", adminHandler.GetComment)
			admin.PUT("/comments/:comment_id/restore", adminHandler.RestoreComment)
			admin.DELETE("/comments/:comment_id", adminHandler.DeleteComment)

			// Reports
			admin.GET("/reports/posts", adminHandler.ListPostReports)
			admin.GET("/reports/posts/:report_id", adminHandler.GetPostReport)
			admin.GET("/reports/comments", adminHandler.ListCommentReports)
			admin.GET("/reports/comments/:report_id", adminHandler.GetCommentReport)
			admin.GET("/reports/users", adminHandler.ListUserReports)
			admin.GET("/reports/users/:report_id", adminHandler.GetUserReport)
			admin.GET("/reports/businesses", adminHandler.ListBusinessReports)
			admin.GET("/reports/businesses/:report_id", adminHandler.GetBusinessReport)
			admin.PUT("/reports/:report_type/:report_id/status", adminHandler.UpdateReportStatus)

			admin.GET("/feedback", adminHandler.ListFeedback)

			// Business Management
			admin.GET("/businesses", adminHandler.ListAllBusinesses)
			admin.GET("/businesses/:business_id", adminHandler.GetBusinessDetail)
			admin.PUT("/businesses/:business_id/status", adminHandler.UpdateBusinessStatus)
			admin.DELETE("/businesses/:business_id", adminHandler.DeleteBusiness)

			// Categories (wire existing handlers)
			admin.GET("/categories", categoryHandler.GetAllCategories)
			admin.POST("/categories", categoryHandler.CreateCategory)
			admin.PUT("/categories/:category_id", categoryHandler.UpdateCategory)
			admin.DELETE("/categories/:category_id", categoryHandler.DeleteCategory)

			// Push Notifications
			admin.POST("/notifications/broadcast", adminHandler.BroadcastNotification)
			admin.POST("/notifications/send", adminHandler.SendTargetedNotification)
		}

		// Placeholder for future routes
		v1.GET("/ping", func(c *gin.Context) {
			utils.SendSuccess(c, http.StatusOK, "pong", gin.H{
				"version": "1.0.0",
				"service": "hamsaya-backend",
			})
		})
	}

	sugaredLogger.Info("Routes registered successfully")

	// Bind to all interfaces when host is empty or localhost so the app can connect via LAN IP (e.g. 192.168.x.x:8080)
	host := cfg.Server.Host
	if host == "" || host == "localhost" || host == "127.0.0.1" {
		host = "0.0.0.0"
	}
	port := cfg.Server.Port
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("%s:%s", host, port)
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

	// Background job: expire unsold SELL posts and notify owners (runs every hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run once immediately on startup to catch any posts that expired while server was down
		if count, err := postService.ProcessExpiredSellPosts(context.Background()); err != nil {
			sugaredLogger.Warnw("Sell expiry job failed", "error", err)
		} else if count > 0 {
			sugaredLogger.Infow("Sell expiry job completed", "expired_count", count)
		}

		for {
			select {
			case <-ticker.C:
				if count, err := postService.ProcessExpiredSellPosts(context.Background()); err != nil {
					sugaredLogger.Warnw("Sell expiry job failed", "error", err)
				} else if count > 0 {
					sugaredLogger.Infow("Sell expiry job completed", "expired_count", count)
				}
			case <-quit:
				return
			}
		}
	}()

	<-quit

	sugaredLogger.Info("Shutting down server...")

	// Shut down WebSocket hub (close all client connections)
	sugaredLogger.Info("Shutting down WebSocket hub...")
	wsHub.Shutdown()

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		sugaredLogger.Fatalw("Server forced to shutdown", "error", err)
	}

	sugaredLogger.Info("Server exited successfully")
}
