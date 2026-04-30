package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	_ "github.com/hamsaya/backend/docs" // Import swagger docs
	"github.com/hamsaya/backend/internal/handlers"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	pkgcrypto "github.com/hamsaya/backend/pkg/crypto"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/secrets"
	"github.com/hamsaya/backend/pkg/transcode"
	"github.com/hamsaya/backend/pkg/notification"
	"github.com/hamsaya/backend/pkg/observability"
	"github.com/hamsaya/backend/pkg/redislock"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap/zapcore"
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
	// Initialize the secrets source. Default backend is process env (no
	// behavior change). SECRETS_BACKEND=ssm switches to AWS SSM Parameter
	// Store (stub today — see pkg/secrets/ssm.go header for wiring).
	secretsCtx, secretsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	secretsSource, secretsLabel, secretsErr := secrets.FromEnvOrBackend(secretsCtx)
	secretsCancel()
	if secretsErr != nil {
		fmt.Printf("Failed to initialize secrets source: %v\n", secretsErr)
		os.Exit(1)
	}
	_ = secretsSource // reserved for future use by config.Load (overlay)

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
	sugaredLogger.Infow("Secrets backend", "source", secretsLabel)
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
	concrete, err := observability.NewTelemetry(context.Background(), otelCfg, logger)
	switch {
	case err != nil:
		sugaredLogger.Warnw("OpenTelemetry init failed, falling back to no-op",
			"error", err, "endpoint", otelCfg.OTLPEndpoint, "enabled", otelCfg.Enabled)
		telem = observability.NewNoopTelemetry(otelCfg, logger)
	case concrete == nil:
		// Init returned (nil, nil) — observability disabled or endpoint unreachable.
		// Emit a clear startup signal instead of silently degrading.
		if otelCfg.Enabled {
			sugaredLogger.Warnw("OpenTelemetry enabled but exporter unreachable, falling back to no-op",
				"endpoint", otelCfg.OTLPEndpoint)
		} else {
			sugaredLogger.Infow("OpenTelemetry disabled (set OBSERVABILITY_ENABLED=true to enable)")
		}
		telem = observability.NewNoopTelemetry(otelCfg, logger)
	default:
		telem = concrete
		sugaredLogger.Infow("OpenTelemetry initialized",
			"endpoint", otelCfg.OTLPEndpoint, "sampling_rate", otelCfg.SamplingRate)
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

	// Mirror warn+ log entries to the app_logs table so the admin /logs page
	// can surface them. The sink runs in a background goroutine bounded by a
	// 256-entry channel; oversize bursts evict oldest rather than block.
	dbLogSink := observability.NewDBLogSink(db, zapcore.WarnLevel, 256)
	dbLogSink.Start(context.Background())
	logger = utils.WrapWithCore(logger, dbLogSink)
	sugaredLogger = logger.Sugar()

	// Connect to Redis
	sugaredLogger.Info("Connecting to Redis...")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() { _ = redisClient.Close() }()

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

	// Cross-instance fanout via Redis pub/sub. Enabled when WS_FANOUT=true
	// (multi-pod deployments). Single-pod runs leave it disabled — the local
	// shards handle everything.
	if os.Getenv("WS_FANOUT") == "true" {
		hostname, _ := os.Hostname()
		fanout := websocket.NewFanout(redisClient, wsHub, hostname, logger)
		fanout.Start()
		wsHub.AttachFanout(fanout)
		sugaredLogger.Infow("WebSocket pub/sub fanout enabled", "process_id", hostname)
		defer fanout.Stop()
	}

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

	// At-rest encryption for MFA secrets. Plaintext fallback is logged loudly.
	var mfaCipher *pkgcrypto.SecretCipher
	if cfg.Crypto.MFASecretKey != "" {
		c, cErr := pkgcrypto.NewSecretCipher(cfg.Crypto.MFASecretKey)
		if cErr != nil {
			sugaredLogger.Fatalf("invalid MFA_SECRET_ENCRYPTION_KEY: %v", cErr)
		}
		mfaCipher = c
		sugaredLogger.Info("MFA secret at-rest encryption: enabled")
	} else {
		sugaredLogger.Warn("MFA_SECRET_ENCRYPTION_KEY not set — MFA secrets stored plaintext (NOT compliant for prod)")
	}

	// Initialize repositories
	sugaredLogger.Info("Initializing repositories...")
	userRepo := repositories.NewUserRepository(db)
	mfaRepo := repositories.NewMFARepository(db, mfaCipher)
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
	helpChatRepo := repositories.NewHelpChatRepository(db)
	dailyLimitRepo := repositories.NewDailyLimitRepository(db)
	monetizationRepo := repositories.NewMonetizationRepository(db)
	appLogRepo := repositories.NewAppLogRepository(db)

	// Initialize services
	sugaredLogger.Info("Initializing services...")
	jwtService := services.NewJWTService(&cfg.JWT)
	passwordService := services.NewPasswordService()
	emailService := services.NewEmailService(&cfg.Email, logger)
	tokenStorage := services.NewTokenStorageService(redisClient, logger)
	mfaService := services.NewMFAService(mfaRepo, userRepo, passwordService, logger)
	oauthService := services.NewOAuthService(cfg, userRepo, logger)
	storageService := services.NewStorageService(cfg, logger)

	// Async WebP transcode pool. Opt-in via TRANSCODE_ASYNC=true so the
	// existing synchronous-encode upload path keeps working until handlers
	// are migrated to enqueue jobs. Pool runs only when storage is real.
	if os.Getenv("TRANSCODE_ASYNC") == "true" && storageService.Client() != nil {
		transcodeQueue := transcode.NewQueue(redisClient, "")
		transcodePool := transcode.NewPool(transcodeQueue, storageService.Client(), logger, 4)
		transcodeCtx, transcodeCancel := context.WithCancel(context.Background())
		go transcodePool.Run(transcodeCtx)
		defer transcodeCancel()
		sugaredLogger.Info("Transcode pool started (4 workers)")
	}
	profileService := services.NewProfileService(userRepo, postRepo, commentRepo, relationshipsRepo, logger)
	notificationService := services.NewNotificationService(notificationRepo, notificationSettingsRepo, userRepo, fcmClient, redisClient, wsHub, logger)
	relationshipsService := services.NewRelationshipsService(relationshipsRepo, userRepo, notificationService, logger)
	businessService := services.NewBusinessService(businessRepo, userRepo, notificationService, logger)
	categoryService := services.NewCategoryService(categoryRepo, logger)
	fanoutService := services.NewFanoutService(fanoutRepo, logger)
	dailyLimitService := services.NewDailyLimitService(dailyLimitRepo, redisClient, logger)
	monetizationService := services.NewMonetizationService(monetizationRepo, storageService, logger)
	postService := services.NewPostService(postRepo, pollRepo, userRepo, businessRepo, relationshipsRepo, categoryRepo, eventRepo, notificationService, fanoutService, fanoutRepo, dailyLimitService, cfg.Storage.BucketName, logger)
	commentService := services.NewCommentService(commentRepo, postRepo, userRepo, businessRepo, notificationService, logger)
	pollService := services.NewPollService(pollRepo, postRepo, userRepo, notificationService, logger)
	eventService := services.NewEventService(eventRepo, postRepo, userRepo, notificationService, logger)
	authService := services.NewAuthService(userRepo, adminRepo, passwordService, jwtService, emailService, tokenStorage, mfaService, cfg, logger)
	authService.SetNotificationService(notificationService)
	chatService := services.NewChatService(conversationRepo, messageRepo, userRepo, businessRepo, notificationService, wsHub, logger)
	searchService := services.NewSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relationshipsRepo, logger)
	reportService := services.NewReportService(reportRepo, postRepo, userRepo, validator)
	feedbackService := services.NewFeedbackService(feedbackRepo, validator)
	adminService := services.NewAdminService(adminRepo, fcmClient, notificationService, logger)
	helpChatService := services.NewHelpChatService(helpChatRepo, logger)
	helpChatService.SetNotificationService(notificationService)

	// Initialize middleware
	sugaredLogger.Info("Initializing middleware...")
	authMiddleware := middleware.NewAuthMiddleware(jwtService, userRepo, tokenStorage, logger)
	// verifiedAuth requires email verification; use for create/update/delete (post, comment, follow, etc.)
	verifiedAuth := authMiddleware.RequireVerifiedEmail()
	rateLimiter := middleware.NewRateLimiter(redisClient, logger)
	banMiddleware := middleware.NewBanMiddleware(adminRepo, logger)

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
	router.Use(middleware.BodyLimit(middleware.DefaultMaxBodyBytes))
	router.Use(middleware.Timeout(middleware.DefaultRequestTimeout))
	router.Use(banMiddleware.Enforce())

	// gzip JSON responses (excludes uploads, websocket, metrics)
	router.Use(gzip.Gzip(
		gzip.DefaultCompression,
		gzip.WithExcludedPaths([]string{
			"/api/v1/posts/upload-image",
			"/api/v1/users/me/avatar",
			"/api/v1/users/me/cover",
			"/api/v1/businesses/", // covers /:id/avatar /cover /attachments
			"/api/v1/chat/ws",
			"/metrics",
			"/health",
		}),
	))

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
	adminCookieCfg := utils.NewCookieConfig(cfg.Server.Env, cfg.Server.AdminCookieDomain)
	featureFlagRepo := repositories.NewFeatureFlagRepository(db)
	systemHandler := handlers.NewSystemHandler(db, redisClient, featureFlagRepo, logger)
	customRoleRepo := repositories.NewCustomRoleRepository(db)
	adminAuthHandler := handlers.NewAdminAuthHandler(authService, customRoleRepo, validator, logger, adminCookieCfg, cfg.JWT)
	customRoleHandler := handlers.NewCustomRoleHandler(customRoleRepo, logger)
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
	adminHandler := handlers.NewAdminHandler(adminService, mfaService, validator, logger)
	helpChatHandler := handlers.NewHelpChatHandler(helpChatService, validator, logger)
	dailyLimitHandler := handlers.NewDailyLimitHandler(dailyLimitService, userRepo, validator, logger)
	monetizationHandler := handlers.NewMonetizationHandler(monetizationService, storageService, validator, logger)
	appLogHandler := handlers.NewAppLogHandler(appLogRepo, logger)

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
			auth.POST("/device/login", rateLimiter.LimitAuth(), authHandler.DeviceLogin)

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

			// Admin SPA cookie-auth flow (HttpOnly + CSRF). Parallel to the
			// JSON-token endpoints above; mobile clients keep using /login.
			auth.POST("/admin/login", rateLimiter.LimitLoginAttempts(), adminAuthHandler.AdminLogin)
			auth.POST("/admin/refresh", rateLimiter.LimitAuth(), adminAuthHandler.AdminRefresh)
			auth.POST("/admin/mfa/verify", rateLimiter.LimitAuth(), adminAuthHandler.AdminMFAVerify)
			auth.POST("/admin/logout", authMiddleware.RequireAuth(), middleware.CSRF(), adminAuthHandler.AdminLogout)

			// Protected auth routes (require authentication)
			auth.POST("/logout", authMiddleware.RequireAuth(), authHandler.Logout)
			auth.POST("/logout-all", authMiddleware.RequireAuth(), authHandler.LogoutAll)
			auth.POST("/send-verification-email", authMiddleware.RequireAuth(), authHandler.SendVerificationEmail)
			auth.POST("/change-password", verifiedAuth, authHandler.ChangePassword)
			auth.GET("/sessions", authMiddleware.RequireAuth(), authHandler.GetActiveSessions)
			auth.POST("/device/register", authMiddleware.RequireAuth(), authHandler.RegisterDevice)
			auth.DELETE("/device/:id", authMiddleware.RequireAuth(), authHandler.RevokeDevice)
		}

		// MFA routes (require verified email — enrolling/disabling MFA on an unverified account is an account-takeover vector)
		mfaHandler.RegisterRoutes(v1, verifiedAuth)

		// Profile routes
		users := v1.Group("/users")
		{
			// /me/posts, /me/bookmarks, /me/events are registered above on v1

			// Protected routes (require authentication)
			users.GET("/me", authMiddleware.RequireAuth(), profileHandler.GetMyProfile)
			users.PUT("/me", authMiddleware.RequireAuth(), profileHandler.UpdateProfile)
			users.DELETE("/me", verifiedAuth, profileHandler.DeleteAccount)
			users.POST("/me/avatar", verifiedAuth, profileHandler.UploadAvatar)
			users.DELETE("/me/avatar", verifiedAuth, profileHandler.DeleteAvatar)
			users.POST("/me/cover", verifiedAuth, profileHandler.UploadCover)
			users.DELETE("/me/cover", verifiedAuth, profileHandler.DeleteCover)
			// GDPR Article 20: per-user data export. 1 / 24h. Requires verified email so unverified accounts can't exfiltrate data.
			users.GET("/me/export", verifiedAuth, rateLimiter.LimitDataExport(), profileHandler.ExportData)

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
			// Daily limit usage — must come before /:post_id for the same reason.
			posts.GET("/daily-limits", authMiddleware.RequireAuth(), dailyLimitHandler.GetMyDailyLimits)
			posts.GET("/:post_id", authMiddleware.RequireAuth(), postHandler.GetPost)

			// Protected routes (require verified email)
			posts.POST("", verifiedAuth, rateLimiter.LimitPostsCreate(), postHandler.CreatePost)
			posts.POST("/upload-image", verifiedAuth, rateLimiter.LimitPostsCreate(), postHandler.UploadPostImage)
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

		// Chat routes — WS uses plain auth (only needs to receive frames, no
		// email verification required). Send/write endpoints use verifiedAuth.
		chat := v1.Group("/chat")
		{
			// WebSocket: plain auth — unverified users still need real-time
			// notification + chat frames.
			chat.GET("/ws", authMiddleware.RequireAuth(), chatHandler.HandleWebSocket)

			// HTTP endpoints — write operations still require verified email
			chat.POST("/messages", verifiedAuth, chatHandler.SendMessage)
			chat.GET("/conversations", authMiddleware.RequireAuth(), chatHandler.GetConversations)
			chat.GET("/conversations/:conversation_id/messages", authMiddleware.RequireAuth(), chatHandler.GetMessages)
			chat.POST("/conversations/:conversation_id/read", authMiddleware.RequireAuth(), chatHandler.MarkConversationAsRead)
			chat.DELETE("/messages/:message_id", verifiedAuth, chatHandler.DeleteMessage)
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

		// Help chat routes (user ↔ support)
		helpChat := v1.Group("/help-chat")
		helpChat.Use(authMiddleware.RequireAuth())
		{
			helpChat.POST("/messages", helpChatHandler.SendMessage)
			helpChat.GET("/messages", helpChatHandler.GetMessages)
		}

		// Admin routes — base group requires moderator-or-above. Per-endpoint
		// middleware tightens this where the action exceeds moderator scope.
		// Tier semantics:
		//   RequireAdmin      → moderator, admin, super_admin
		//   RequireAdminOnly  → admin, super_admin (excludes moderator)
		//   RequireSuperAdmin → super_admin only
		adminOnly := authMiddleware.RequireAdminOnly()
		superOnly := authMiddleware.RequireSuperAdmin()

		admin := v1.Group("/admin")
		admin.Use(authMiddleware.RequireAdmin())
		{
			// Dashboard & Analytics — admin-tier (mods don't see analytics).
			admin.GET("/stats", adminOnly, adminHandler.GetDashboardStats)
			admin.GET("/inbox-counts", adminHandler.GetInboxCounts)
			admin.GET("/analytics/users", adminOnly, adminHandler.GetUserAnalytics)
			admin.GET("/analytics/posts", adminOnly, adminHandler.GetPostAnalytics)
			admin.GET("/analytics/engagement", adminOnly, adminHandler.GetEngagementAnalytics)
			admin.GET("/analytics/businesses", adminOnly, adminHandler.GetBusinessAnalytics)

			// User Management — read for all admins; suspend/unsuspend admin-only;
			// delete admin-only; role change super_admin-only.
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/users/:user_id", adminHandler.GetUser)
			admin.POST("/users/:user_id/suspend", adminOnly, adminHandler.SuspendUser)
			admin.POST("/users/:user_id/unsuspend", adminOnly, adminHandler.UnsuspendUser)
			admin.DELETE("/users/:user_id", adminOnly, adminHandler.DeleteUser)
			admin.PUT("/users/:user_id/role", superOnly, adminHandler.UpdateUserRole)
			admin.POST("/users/:user_id/force-disable-mfa", adminOnly, adminHandler.ForceDisableUserMFA)

			// Content Moderation — moderator-and-above.
			admin.GET("/posts", adminHandler.ListAllPosts)
			admin.GET("/posts/:post_id", adminHandler.GetPostDetail)
			admin.DELETE("/posts/:post_id", adminHandler.DeletePost)
			admin.POST("/posts/bulk-delete", adminHandler.BulkDeletePosts)
			admin.PUT("/posts/:post_id/status", adminHandler.UpdatePostStatus)
			admin.PATCH("/posts/:post_id", adminHandler.UpdatePost)
			admin.GET("/comments", adminHandler.ListAllComments)
			admin.GET("/comments/:comment_id", adminHandler.GetComment)
			admin.PATCH("/comments/:comment_id", adminHandler.UpdateCommentContent)
			admin.PUT("/comments/:comment_id/restore", adminHandler.RestoreComment)
			admin.DELETE("/comments/:comment_id", adminHandler.DeleteComment)
			admin.POST("/comments/bulk-delete", adminHandler.BulkDeleteComments)

			// Reports — moderator-and-above.
			admin.GET("/reports/posts", adminHandler.ListPostReports)
			admin.GET("/reports/posts/:report_id", adminHandler.GetPostReport)
			admin.GET("/reports/comments", adminHandler.ListCommentReports)
			admin.GET("/reports/comments/:report_id", adminHandler.GetCommentReport)
			admin.GET("/reports/users", adminHandler.ListUserReports)
			admin.GET("/reports/users/:report_id", adminHandler.GetUserReport)
			admin.GET("/reports/businesses", adminHandler.ListBusinessReports)
			admin.GET("/reports/businesses/:report_id", adminHandler.GetBusinessReport)
			admin.PUT("/reports/:report_type/:report_id/status", adminHandler.UpdateReportStatus)

			// Feedback — list for all admins; resolve admin-only.
			admin.GET("/feedback", adminHandler.ListFeedback)
			admin.PUT("/feedback/:feedback_id/resolve", adminOnly, adminHandler.ResolveFeedback)

			// Business Management — read+approve for all admins; delete admin-only.
			admin.GET("/businesses", adminHandler.ListAllBusinesses)
			admin.GET("/businesses/:business_id", adminHandler.GetBusinessDetail)
			admin.PUT("/businesses/:business_id/status", adminHandler.UpdateBusinessStatus)
			admin.DELETE("/businesses/:business_id", adminOnly, adminHandler.DeleteBusiness)

			// Categories — admin-only (platform config).
			admin.GET("/categories", adminOnly, categoryHandler.GetAllCategories)
			admin.POST("/categories", adminOnly, categoryHandler.CreateCategory)
			admin.PUT("/categories/:category_id", adminOnly, categoryHandler.UpdateCategory)
			admin.DELETE("/categories/:category_id", adminOnly, categoryHandler.DeleteCategory)

			// Push Notifications — broadcast admin-only; targeted super_admin-only
			// (named-user push has higher abuse potential than mass broadcast).
			admin.POST("/notifications/broadcast", adminOnly, adminHandler.BroadcastNotification)
			admin.POST("/notifications/send", superOnly, adminHandler.SendTargetedNotification)
			admin.GET("/notifications/history", adminOnly, adminHandler.ListBroadcastHistory)

			// Audit Logs — admin-and-above. Mods don't audit other admins.
			admin.GET("/audit-logs", adminOnly, adminHandler.ListAuditLogs)

			// Custom named roles — super_admin only for mutations, admin+ for reads.
			admin.GET("/custom-roles", adminOnly, customRoleHandler.List)
			admin.POST("/custom-roles", superOnly, customRoleHandler.Create)
			admin.GET("/custom-roles/:role_id", adminOnly, customRoleHandler.Get)
			admin.PUT("/custom-roles/:role_id", superOnly, customRoleHandler.Update)
			admin.DELETE("/custom-roles/:role_id", superOnly, customRoleHandler.Delete)
			admin.GET("/custom-roles/:role_id/users", adminOnly, customRoleHandler.ListRoleUsers)
			admin.POST("/users/:user_id/custom-role", superOnly, customRoleHandler.Assign)
			admin.GET("/users/:user_id/custom-role", adminOnly, customRoleHandler.GetUserCustomRole)

			// Admin Account Management — list admin-only; mutations super-only.
			admin.GET("/accounts", adminOnly, adminHandler.ListAdmins)
			admin.GET("/accounts/invites", adminOnly, adminHandler.ListAdminInvites)
			admin.POST("/accounts/invites", superOnly, adminHandler.CreateAdminInvite)
			admin.DELETE("/accounts/invites/:invite_id", superOnly, adminHandler.RevokeAdminInvite)

			// IP / Device Bans — admin-only (safety hammer).
			admin.GET("/bans/ip", adminOnly, adminHandler.ListIPBans)
			admin.POST("/bans/ip", adminOnly, adminHandler.CreateIPBan)
			admin.DELETE("/bans/ip/:ban_id", adminOnly, adminHandler.DeleteIPBan)
			admin.GET("/bans/devices", adminOnly, adminHandler.ListDeviceBans)
			admin.POST("/bans/devices", adminOnly, adminHandler.CreateDeviceBan)
			admin.DELETE("/bans/devices/:ban_id", adminOnly, adminHandler.DeleteDeviceBan)

			// Help chat — moderator-and-above.
			admin.GET("/help-chat", helpChatHandler.AdminGetThreads)
			admin.GET("/help-chat/:user_id", helpChatHandler.AdminGetUserThread)
			admin.POST("/help-chat/:user_id/reply", helpChatHandler.AdminReply)

			// Daily-post-limit management — admin-only.
			admin.GET("/daily-limits", adminOnly, dailyLimitHandler.AdminListLimits)
			admin.PUT("/daily-limits/:post_type", adminOnly, dailyLimitHandler.AdminUpdateLimit)

			// Monetization — admin-only. The user-facing surface (advertiser
			// submission, boost purchase, credit topup) lives elsewhere; these
			// routes are oversight + ad review only.
			admin.GET("/ads", adminOnly, monetizationHandler.ListAds)
			admin.POST("/ads", adminOnly, monetizationHandler.CreateAd)
			admin.GET("/ads/:ad_id", adminOnly, monetizationHandler.GetAd)
			admin.PUT("/ads/:ad_id/approve", adminOnly, monetizationHandler.ApproveAd)
			admin.PUT("/ads/:ad_id/reject", adminOnly, monetizationHandler.RejectAd)
			admin.DELETE("/ads/:ad_id", adminOnly, monetizationHandler.DeleteAd)

			admin.GET("/credits", adminOnly, monetizationHandler.ListBalances)
			admin.GET("/credits/:user_id", adminOnly, monetizationHandler.GetUserCredits)
			admin.POST("/credits/:user_id/adjust", adminOnly, monetizationHandler.AdjustUserCredits)

			admin.GET("/boosts", adminOnly, monetizationHandler.ListBoosts)
			admin.PUT("/boosts/:boost_id/cancel", adminOnly, monetizationHandler.CancelBoost)

			// /admin/system/* — super_admin exclusive platform telemetry +
			// feature-flag controls. RequireSuperAdmin replaces (not stacks
			// with) the group middleware here, but Gin runs both — moderator
			// requests are rejected by RequireSuperAdmin before reaching the
			// handler, which is the desired behavior.
			admin.GET("/system/build-info", superOnly, systemHandler.BuildInfo)
			admin.GET("/system/health", superOnly, systemHandler.ServiceHealth)
			admin.GET("/system/table-stats", superOnly, systemHandler.TableStats)
			admin.GET("/system/sessions", superOnly, systemHandler.SessionsList)
			admin.POST("/system/sessions/:session_id/revoke", superOnly, systemHandler.SessionRevoke)
			admin.GET("/system/flags", superOnly, systemHandler.FlagsList)
			admin.PUT("/system/flags/:key", superOnly, systemHandler.FlagsToggle)
			admin.GET("/system/denylist-stats", superOnly, systemHandler.DenylistStats)

			// Application logs — super_admin only. Backed by the DBLogSink
			// (pkg/observability) which mirrors warn+ entries to app_logs.
			admin.GET("/logs", superOnly, appLogHandler.List)
		}

		// Public-facing ads — mobile feed fetches active placements without
		// authentication so logged-out browsing still serves ads. Impression
		// and click counters are best-effort, fire-and-forget.
		v1.GET("/ads/active", monetizationHandler.ListActiveAdsPublic)
		v1.POST("/ads/:ad_id/impression", monetizationHandler.RecordAdImpression)
		v1.POST("/ads/:ad_id/click", monetizationHandler.RecordAdClick)

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

	// Boost expiry sweeper — flips ACTIVE boosts past their end_at to
	// EXPIRED every 15 minutes so the admin panel and public queries stay
	// accurate without relying on lazy filtering alone.
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				res, err := db.Pool.Exec(context.Background(), `
					UPDATE boosts SET status = 'EXPIRED'
					WHERE status = 'ACTIVE' AND expires_at < NOW()
				`)
				if err != nil {
					sugaredLogger.Warnw("boost expiry sweep failed", "error", err)
				} else if res.RowsAffected() > 0 {
					sugaredLogger.Infow("boost expiry sweep", "expired", res.RowsAffected())
				}
			}
		}
	}()

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

	// runIfLeader executes fn only when this instance holds the named Redis lock.
	// Lock TTL is shorter than the job interval, so a crashed leader's lock expires
	// before the next tick and another instance can take over.
	runIfLeader := func(jobName, lockKey string, lockTTL time.Duration, fn func(context.Context) error) {
		runCtx, cancel := context.WithTimeout(context.Background(), lockTTL)
		defer cancel()
		lock, err := redislock.Acquire(runCtx, redisClient, lockKey, lockTTL)
		if err != nil {
			if errors.Is(err, redislock.ErrNotAcquired) {
				return // another instance is running the job
			}
			sugaredLogger.Warnw("Background job lock error", "job", jobName, "error", err)
			return
		}
		defer func() {
			if err := lock.Release(context.Background()); err != nil {
				sugaredLogger.Warnw("Background job lock release error", "job", jobName, "error", err)
			}
		}()
		if err := fn(runCtx); err != nil {
			sugaredLogger.Warnw("Background job failed", "job", jobName, "error", err)
		}
	}

	// Background job: expire unsold SELL posts and notify owners (runs every hour).
	// Leader-elected via Redis lock so only one instance executes per tick.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		expireSellPosts := func(ctx context.Context) error {
			count, err := postService.ProcessExpiredSellPosts(ctx)
			if err != nil {
				return err
			}
			if count > 0 {
				sugaredLogger.Infow("Sell expiry job completed", "expired_count", count)
			}
			return nil
		}

		runIfLeader("sell-expiry", "lock:job:sell-expiry", 30*time.Minute, expireSellPosts)

		for {
			select {
			case <-ticker.C:
				runIfLeader("sell-expiry", "lock:job:sell-expiry", 30*time.Minute, expireSellPosts)
			case <-quit:
				return
			}
		}
	}()

	// Background job: purge expired and revoked sessions (runs every 24 hours).
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		purgeSessions := func(ctx context.Context) error {
			count, err := userRepo.DeleteExpiredSessions(ctx)
			if err != nil {
				return err
			}
			if count > 0 {
				sugaredLogger.Infow("Session cleanup completed", "deleted_count", count)
			}
			return nil
		}

		runIfLeader("session-cleanup", "lock:job:session-cleanup", 12*time.Hour, purgeSessions)

		for {
			select {
			case <-ticker.C:
				runIfLeader("session-cleanup", "lock:job:session-cleanup", 12*time.Hour, purgeSessions)
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
