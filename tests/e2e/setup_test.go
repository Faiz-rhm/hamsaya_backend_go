// Package e2e contains end-to-end tests that require real infrastructure
// (PostgreSQL + Redis). Run with: make docker-up && go test ./tests/e2e/...
//
// Tests are automatically skipped when the database is unreachable.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/handlers"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// testEnv holds all wired-up infrastructure for one E2E test run.
type testEnv struct {
	db          *database.DB
	redisClient *redis.Client
	server      *httptest.Server
	router      *gin.Engine
	cfg         *config.Config
}

// setupE2E creates a full application stack backed by real Postgres and an
// in-process miniredis. It skips the test if Postgres is unreachable.
func setupE2E(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	_ = utils.InitLogger("error")

	cfg := testConfig()

	// --- Postgres ---
	db, err := database.New(&cfg.Database)
	if err != nil {
		t.Skipf("E2E: postgres unavailable (%v) — skipping", err)
	}
	t.Cleanup(func() { db.Pool.Close() })

	// Run migrations so schema is current.
	migrationsPath := migrationsDir()
	if migrationsPath != "" {
		migrator := database.NewMigrator(db, migrationsPath)
		if migrErr := migrator.Up(context.Background()); migrErr != nil {
			t.Fatalf("E2E: migration failed: %v", migrErr)
		}
	}

	// --- Redis (miniredis, no real Redis needed) ---
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	// --- Service + handler wiring ---
	logger := zap.NewNop()
	router := buildRouter(t, db, redisClient, cfg, logger)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testEnv{
		db:          db,
		redisClient: redisClient,
		server:      srv,
		router:      router,
		cfg:         cfg,
	}
}

// url returns a full URL for the given path on the test server.
func (e *testEnv) url(path string) string {
	return e.server.URL + path
}

// do executes an HTTP request and returns the response.
func (e *testEnv) do(req *http.Request) *http.Response {
	resp, err := e.server.Client().Do(req)
	if err != nil {
		panic(fmt.Sprintf("E2E request failed: %v", err))
	}
	return resp
}

// cleanupTestData removes rows created by tests (identified by email prefix).
func (e *testEnv) cleanupTestData(t *testing.T, emailPattern string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = e.db.Pool.Exec(ctx,
		`DELETE FROM users WHERE email LIKE $1`, emailPattern)
}

// makeAdmin promotes a user to admin role directly in the DB.
// The JWT middleware checks role from DB on every request, so no re-login needed.
func (e *testEnv) makeAdmin(t *testing.T, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.db.Pool.Exec(ctx,
		`UPDATE users SET role = 'admin' WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("makeAdmin: failed to promote user %s: %v", userID, err)
	}
}

// makeSuperAdmin promotes a user to super_admin role.
func (e *testEnv) makeSuperAdmin(t *testing.T, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.db.Pool.Exec(ctx,
		`UPDATE users SET role = 'super_admin' WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("makeSuperAdmin: failed to promote user %s: %v", userID, err)
	}
}

// makeModerator promotes a user to moderator role.
func (e *testEnv) makeModerator(t *testing.T, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.db.Pool.Exec(ctx,
		`UPDATE users SET role = 'moderator' WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("makeModerator: failed to promote user %s: %v", userID, err)
	}
}

// testConfig returns config pointing at the Docker test database.
func testConfig() *config.Config {
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5433")
	name := getEnvOrDefault("DB_NAME", "hamsaya")
	user := getEnvOrDefault("DB_USER", "postgres")
	pass := getEnvOrDefault("DB_PASSWORD", "postgres")

	return &config.Config{
		Database: config.DatabaseConfig{
			Host:            host,
			Port:            port,
			Name:            name,
			User:            user,
			Password:        pass,
			SSLMode:         "disable",
			MaxConns:        5,
			MinConns:        1,
			MaxConnLifetime: time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
		},
		JWT: config.JWTConfig{
			Secret:               "e2e-test-secret-key-at-least-32-chars-long",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenDuration: 7 * 24 * time.Hour,
		},
		Email: config.EmailConfig{},
	}
}

// migrationsDir returns the path to the migrations directory relative to the
// test binary's working directory.
func migrationsDir() string {
	candidates := []string{
		"../../migrations",
		"../migrations",
		"migrations",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// buildRouter wires up services and handlers into a Gin router — the same
// dependency graph as cmd/server/main.go but without observability/FCM/MinIO.
func buildRouter(
	t *testing.T,
	db *database.DB,
	redisClient *redis.Client,
	cfg *config.Config,
	logger *zap.Logger,
) *gin.Engine {
	t.Helper()

	// Repositories
	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	commentRepo := repositories.NewCommentRepository(db)
	notificationRepo := repositories.NewNotificationRepository(db)
	notifSettingsRepo := repositories.NewNotificationSettingsRepository(db)
	relationshipsRepo := repositories.NewRelationshipsRepository(db)
	businessRepo := repositories.NewBusinessRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)
	eventRepo := repositories.NewEventRepository(db)
	pollRepo := repositories.NewPollRepository(db)
	fanoutRepo := repositories.NewFanoutRepository(db)
	adminRepo := repositories.NewAdminRepository(db)
	conversationRepo := repositories.NewConversationRepository(db)
	messageRepo := repositories.NewMessageRepository(db)
	searchRepo := repositories.NewSearchRepository(db)
	reportRepo := repositories.NewReportRepository(db)
	feedbackRepo := repositories.NewFeedbackRepository(db)
	helpChatRepo := repositories.NewHelpChatRepository(db)
	mfaRepo := repositories.NewMFARepository(db, nil)

	// Services
	jwtSvc := services.NewJWTService(&cfg.JWT)
	passwordSvc := services.NewPasswordServiceWithCost(bcrypt.MinCost)
	tokenStorage := services.NewTokenStorageService(redisClient, logger)
	emailSvc := services.NewEmailService(&cfg.Email, logger)
	fanoutSvc := services.NewFanoutService(fanoutRepo, logger)
	validator := utils.NewValidator()
	wsHub := websocket.NewHub(logger)
	go wsHub.Run()
	t.Cleanup(wsHub.Shutdown)

	oauthSvc := services.NewOAuthService(cfg, userRepo, logger)
	mfaSvc := services.NewMFAService(mfaRepo, userRepo, passwordSvc, logger)

	notifSvc := services.NewNotificationService(
		notificationRepo, notifSettingsRepo, userRepo, nil, redisClient, wsHub, logger,
	)
	authSvc := services.NewAuthService(
		userRepo, adminRepo, passwordSvc, jwtSvc, emailSvc, tokenStorage, mfaSvc, cfg, logger,
	)
	dailyLimitRepo := repositories.NewDailyLimitRepository(db)
	dailyLimitSvc := services.NewDailyLimitService(dailyLimitRepo, redisClient, logger)
	postSvc := services.NewPostService(
		postRepo, pollRepo, userRepo, businessRepo, relationshipsRepo,
		categoryRepo, eventRepo, notifSvc, fanoutSvc, fanoutRepo, dailyLimitSvc, "", logger,
	)
	commentSvc := services.NewCommentService(
		commentRepo, postRepo, userRepo, businessRepo, notifSvc, logger,
	)
	chatSvc := services.NewChatService(conversationRepo, messageRepo, userRepo, businessRepo, relationshipsRepo, notifSvc, wsHub, logger)
	searchSvc := services.NewSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relationshipsRepo, logger)
	profileSvc := services.NewProfileService(userRepo, postRepo, commentRepo, relationshipsRepo, emailSvc, tokenStorage, jwtSvc, logger)
	relationshipsSvc := services.NewRelationshipsService(relationshipsRepo, userRepo, notifSvc, logger)
	businessSvc := services.NewBusinessService(businessRepo, userRepo, notifSvc, logger)
	categorySvc := services.NewCategoryService(categoryRepo, logger)
	eventSvc := services.NewEventService(eventRepo, postRepo, userRepo, notifSvc, logger)
	pollSvc := services.NewPollService(pollRepo, postRepo, userRepo, notifSvc, logger)
	reportSvc := services.NewReportService(reportRepo, postRepo, userRepo, validator)
	feedbackSvc := services.NewFeedbackService(feedbackRepo, validator)
	adminSvc := services.NewAdminService(adminRepo, nil, notifSvc, logger)
	helpChatSvc := services.NewHelpChatService(helpChatRepo, logger)

	// Middleware
	authMW := middleware.NewAuthMiddleware(jwtSvc, userRepo, tokenStorage, logger)
	requireAuth := authMW.RequireAuth()

	// Handlers
	authHandler := handlers.NewAuthHandler(authSvc, validator, logger)
	adminCookieCfg := utils.NewCookieConfig(cfg.Server.Env, cfg.Server.AdminCookieDomain)
	customRoleRepo := repositories.NewCustomRoleRepository(db)
	adminAuthHandler := handlers.NewAdminAuthHandler(authSvc, customRoleRepo, validator, logger, adminCookieCfg, cfg.JWT)
	featureFlagRepo := repositories.NewFeatureFlagRepository(db)
	systemHandler := handlers.NewSystemHandler(db, redisClient, featureFlagRepo, logger)
	postHandler := handlers.NewPostHandler(postSvc, nil, validator, logger)
	commentHandler := handlers.NewCommentHandler(commentSvc, validator, logger)
	chatHandler := handlers.NewChatHandler(chatSvc, wsHub, validator, logger, cfg)
	searchHandler := handlers.NewSearchHandler(searchSvc, validator, logger)
	profileHandler := handlers.NewProfileHandler(profileSvc, nil, validator, logger)
	relationshipsHandler := handlers.NewRelationshipsHandler(relationshipsSvc, logger)
	businessHandler := handlers.NewBusinessHandler(businessSvc, nil, validator, logger)
	categoryHandler := handlers.NewCategoryHandler(categorySvc, validator, logger)
	notificationHandler := handlers.NewNotificationHandler(notifSvc, validator, logger)
	eventHandler := handlers.NewEventHandler(eventSvc, validator, logger)
	pollHandler := handlers.NewPollHandler(pollSvc, validator, logger)
	reportHandler := handlers.NewReportHandler(reportSvc)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackSvc)
	helpChatHandler := handlers.NewHelpChatHandler(helpChatSvc, validator, logger)
	adminHandler := handlers.NewAdminHandler(adminSvc, mfaSvc, validator, logger)
	oauthHandler := handlers.NewOAuthHandler(authSvc, oauthSvc, validator, logger)
	mfaHandler := handlers.NewMFAHandler(mfaSvc, validator, logger)

	// Router
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")

	// Auth routes
	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/unified", authHandler.UnifiedAuth)
	auth.POST("/refresh", authHandler.RefreshToken)
	auth.POST("/mfa/verify", authHandler.VerifyMFA)
	mfaHandler.RegisterRoutes(v1, requireAuth)
	auth.POST("/oauth/google", oauthHandler.GoogleOAuth)
	auth.POST("/oauth/facebook", oauthHandler.FacebookOAuth)
	auth.POST("/oauth/apple", oauthHandler.AppleOAuth)
	auth.POST("/admin/login", adminAuthHandler.AdminLogin)
	auth.POST("/admin/refresh", adminAuthHandler.AdminRefresh)
	auth.POST("/admin/mfa/verify", adminAuthHandler.AdminMFAVerify)
	auth.POST("/admin/logout", requireAuth, middleware.CSRF(), adminAuthHandler.AdminLogout)
	auth.POST("/logout", requireAuth, authHandler.Logout)
	auth.POST("/verify-email", authHandler.VerifyEmail)
	auth.POST("/forgot-password", authHandler.ForgotPassword)
	auth.POST("/verify-reset-code", authHandler.VerifyResetCode)
	auth.POST("/reset-password", authHandler.ResetPassword)
	auth.POST("/change-password", requireAuth, authHandler.ChangePassword)
	auth.POST("/logout-all", requireAuth, authHandler.LogoutAll)
	auth.POST("/send-verification-email", requireAuth, authHandler.SendVerificationEmail)
	auth.GET("/sessions", requireAuth, authHandler.GetActiveSessions)

	// User content shortcuts (top-level)
	v1.GET("/users/me/posts", requireAuth, postHandler.GetMyPosts)
	v1.GET("/users/me/bookmarks", requireAuth, postHandler.GetMyBookmarks)
	v1.GET("/users/me/events", requireAuth, postHandler.GetMyEvents)

	// Profile / user routes (skip email-verification gate for tests)
	users := v1.Group("/users", requireAuth)
	users.GET("/me", profileHandler.GetMyProfile)
	users.PUT("/me", profileHandler.UpdateProfile)
	users.DELETE("/me", profileHandler.DeleteAccount)
	users.POST("/me/avatar", profileHandler.UploadAvatar)
	users.DELETE("/me/avatar", profileHandler.DeleteAvatar)
	users.POST("/me/cover", profileHandler.UploadCover)
	users.DELETE("/me/cover", profileHandler.DeleteCover)
	users.GET("/:user_id", profileHandler.GetUserProfile)
	users.POST("/:user_id/follow", relationshipsHandler.FollowUser)
	users.DELETE("/:user_id/follow", relationshipsHandler.UnfollowUser)
	users.GET("/:user_id/followers", relationshipsHandler.GetFollowers)
	users.GET("/:user_id/following", relationshipsHandler.GetFollowing)
	users.POST("/:user_id/block", relationshipsHandler.BlockUser)
	users.DELETE("/:user_id/block", relationshipsHandler.UnblockUser)
	users.GET("/blocked", relationshipsHandler.GetBlockedUsers)
	users.GET("/:user_id/relationship", relationshipsHandler.GetRelationshipStatus)
	users.POST("/:user_id/report", reportHandler.ReportUser)

	// Post routes
	posts := v1.Group("/posts", requireAuth)
	posts.GET("", postHandler.GetFeed)
	posts.GET("/feed", postHandler.GetPersonalizedFeed)
	posts.POST("", postHandler.CreatePost)
	posts.GET("/:post_id", postHandler.GetPost)
	posts.DELETE("/:post_id", postHandler.DeletePost)
	posts.POST("/:post_id/like", postHandler.LikePost)
	posts.DELETE("/:post_id/like", postHandler.UnlikePost)
	posts.POST("/:post_id/bookmark", postHandler.BookmarkPost)
	posts.DELETE("/:post_id/bookmark", postHandler.UnbookmarkPost)
	posts.POST("/:post_id/share", postHandler.SharePost)
	posts.PUT("/:post_id", postHandler.UpdatePost)
	posts.POST("/:post_id/resell", postHandler.ResellPost)
	posts.POST("/upload-image", postHandler.UploadPostImage)
	posts.GET("/:post_id/comments", commentHandler.GetPostComments)
	posts.POST("/:post_id/comments", commentHandler.CreateComment)
	posts.POST("/:post_id/report", reportHandler.ReportPost)
	posts.GET("/:post_id/polls", pollHandler.GetPostPoll)
	posts.POST("/:post_id/polls", pollHandler.CreatePoll)

	// Comment routes
	comments := v1.Group("/comments", requireAuth)
	comments.GET("/:comment_id", commentHandler.GetComment)
	comments.PUT("/:comment_id", commentHandler.UpdateComment)
	comments.DELETE("/:comment_id", commentHandler.DeleteComment)
	comments.GET("/:comment_id/replies", commentHandler.GetCommentReplies)
	comments.POST("/:comment_id/like", commentHandler.LikeComment)
	comments.DELETE("/:comment_id/like", commentHandler.UnlikeComment)
	comments.POST("/:comment_id/report", reportHandler.ReportComment)

	// Poll routes
	polls := v1.Group("/polls", requireAuth)
	polls.GET("/:poll_id", pollHandler.GetPoll)
	polls.POST("/:poll_id/vote", pollHandler.VotePoll)
	polls.DELETE("/:poll_id/vote", pollHandler.DeleteVote)

	// Event routes
	events := v1.Group("/events", requireAuth)
	events.GET("/:post_id/interest", eventHandler.GetEventInterestStatus)
	events.POST("/:post_id/interest", eventHandler.SetEventInterest)
	events.DELETE("/:post_id/interest", eventHandler.RemoveEventInterest)
	events.GET("/:post_id/interested", eventHandler.GetInterestedUsers)
	events.GET("/:post_id/going", eventHandler.GetGoingUsers)

	// Business routes
	businesses := v1.Group("/businesses", requireAuth)
	businesses.GET("", businessHandler.GetMyBusinesses)
	businesses.GET("/search", businessHandler.ListBusinesses)
	businesses.GET("/categories", businessHandler.GetCategories)
	businesses.POST("", businessHandler.CreateBusiness)
	businesses.GET("/:business_id", businessHandler.GetBusiness)
	businesses.PUT("/:business_id", businessHandler.UpdateBusiness)
	businesses.DELETE("/:business_id", businessHandler.DeleteBusiness)
	businesses.GET("/:business_id/hours", businessHandler.GetBusinessHours)
	businesses.GET("/:business_id/attachments", businessHandler.GetGallery)
	businesses.POST("/:business_id/hours", businessHandler.SetBusinessHours)
	businesses.POST("/:business_id/report", reportHandler.ReportBusiness)
	businesses.DELETE("/:business_id/attachments/:attachment_id", businessHandler.DeleteGalleryImage)
	businesses.POST("/:business_id/attachments", businessHandler.AddGalleryImage)
	businesses.POST("/:business_id/avatar", businessHandler.UploadAvatar)
	businesses.POST("/:business_id/cover", businessHandler.UploadCover)
	businesses.POST("/:business_id/follow", businessHandler.FollowBusiness)
	businesses.DELETE("/:business_id/follow", businessHandler.UnfollowBusiness)

	// Chat routes
	chat := v1.Group("/chat", requireAuth)
	chat.POST("/messages", chatHandler.SendMessage)
	chat.GET("/conversations", chatHandler.GetConversations)
	chat.GET("/conversations/:conversation_id/messages", chatHandler.GetMessages)
	chat.POST("/conversations/:conversation_id/read", chatHandler.MarkConversationAsRead)
	chat.DELETE("/messages/:message_id", chatHandler.DeleteMessage)

	// Notification routes
	notifications := v1.Group("/notifications", requireAuth)
	notifications.GET("", notificationHandler.GetNotifications)
	notifications.GET("/unread-count", notificationHandler.GetUnreadCount)
	notifications.POST("/:notification_id/read", notificationHandler.MarkAsRead)
	notifications.POST("/read-all", notificationHandler.MarkAllAsRead)
	notifications.DELETE("/:notification_id", notificationHandler.DeleteNotification)
	notifications.GET("/settings", notificationHandler.GetNotificationSettings)
	notifications.PUT("/settings", notificationHandler.UpdateNotificationSetting)
	notifications.POST("/fcm-token", notificationHandler.RegisterFCMToken)
	notifications.DELETE("/fcm-token", notificationHandler.UnregisterFCMToken)

	// Public category listing
	categories := v1.Group("/categories", requireAuth)
	categories.GET("", categoryHandler.ListCategories)
	categories.GET("/:category_id", categoryHandler.GetCategory)

	// Search routes
	v1.GET("/search", requireAuth, searchHandler.Search)
	v1.GET("/search/users", requireAuth, searchHandler.SearchUsers)
	v1.GET("/search/posts", requireAuth, searchHandler.SearchPosts)
	v1.GET("/search/businesses", requireAuth, searchHandler.SearchBusinesses)
	v1.GET("/discover", requireAuth, searchHandler.Discover)

	// Feedback routes
	feedback := v1.Group("/feedback", requireAuth)
	feedback.POST("", feedbackHandler.SubmitFeedback)
	feedback.GET("/status", feedbackHandler.GetFeedbackStatus)

	// Help-chat routes
	helpChat := v1.Group("/help-chat", requireAuth)
	helpChat.POST("/messages", helpChatHandler.SendMessage)
	helpChat.GET("/messages", helpChatHandler.GetMessages)

	// Admin routes
	admin := v1.Group("/admin", authMW.RequireAdmin())
	admin.GET("/stats", adminHandler.GetDashboardStats)
	admin.GET("/users", adminHandler.ListUsers)
	admin.GET("/users/:user_id", adminHandler.GetUser)
	admin.POST("/users/:user_id/suspend", adminHandler.SuspendUser)
	admin.POST("/users/:user_id/unsuspend", adminHandler.UnsuspendUser)
	admin.DELETE("/users/:user_id", adminHandler.DeleteUser)
	admin.PUT("/users/:user_id/role", adminHandler.UpdateUserRole)
	admin.GET("/posts", adminHandler.ListAllPosts)
	admin.GET("/posts/:post_id", adminHandler.GetPostDetail)
	admin.DELETE("/posts/:post_id", adminHandler.DeletePost)
	admin.PUT("/posts/:post_id/status", adminHandler.UpdatePostStatus)
	admin.GET("/comments", adminHandler.ListAllComments)
	admin.GET("/comments/:comment_id", adminHandler.GetComment)
	admin.PUT("/comments/:comment_id/restore", adminHandler.RestoreComment)
	admin.DELETE("/comments/:comment_id", adminHandler.DeleteComment)
	admin.GET("/categories", categoryHandler.GetAllCategories)
	admin.POST("/categories", categoryHandler.CreateCategory)
	admin.PUT("/categories/:category_id", categoryHandler.UpdateCategory)
	admin.DELETE("/categories/:category_id", categoryHandler.DeleteCategory)
	admin.GET("/reports/posts", adminHandler.ListPostReports)
	admin.GET("/reports/posts/:report_id", adminHandler.GetPostReport)
	admin.GET("/reports/comments", adminHandler.ListCommentReports)
	admin.GET("/reports/comments/:report_id", adminHandler.GetCommentReport)
	admin.GET("/reports/users", adminHandler.ListUserReports)
	admin.GET("/reports/users/:report_id", adminHandler.GetUserReport)
	admin.GET("/reports/businesses", adminHandler.ListBusinessReports)
	admin.GET("/reports/businesses/:report_id", adminHandler.GetBusinessReport)
	admin.PUT("/reports/:report_type/:report_id/status", adminHandler.UpdateReportStatus)
	admin.GET("/analytics/users", adminHandler.GetUserAnalytics)
	admin.GET("/analytics/posts", adminHandler.GetPostAnalytics)
	admin.GET("/analytics/engagement", adminHandler.GetEngagementAnalytics)
	admin.GET("/analytics/businesses", adminHandler.GetBusinessAnalytics)
	admin.GET("/bans/ip", adminHandler.ListIPBans)
	admin.POST("/bans/ip", adminHandler.CreateIPBan)
	admin.DELETE("/bans/ip/:ban_id", adminHandler.DeleteIPBan)
	admin.GET("/bans/devices", adminHandler.ListDeviceBans)
	admin.POST("/bans/devices", adminHandler.CreateDeviceBan)
	admin.DELETE("/bans/devices/:ban_id", adminHandler.DeleteDeviceBan)
	admin.GET("/audit-logs", adminHandler.ListAuditLogs)
	admin.POST("/notifications/broadcast", adminHandler.BroadcastNotification)
	admin.POST("/notifications/send", adminHandler.SendTargetedNotification)
	admin.GET("/feedback", adminHandler.ListFeedback)
	admin.PUT("/feedback/:feedback_id/resolve", adminHandler.ResolveFeedback)
	admin.GET("/accounts", adminHandler.ListAdmins)
	admin.GET("/accounts/invites", adminHandler.ListAdminInvites)
	admin.POST("/accounts/invites", adminHandler.CreateAdminInvite)
	admin.DELETE("/accounts/invites/:invite_id", adminHandler.RevokeAdminInvite)
	admin.GET("/help-chat", helpChatHandler.AdminGetThreads)
	admin.GET("/help-chat/:user_id", helpChatHandler.AdminGetUserThread)
	admin.POST("/help-chat/:user_id/reply", helpChatHandler.AdminReply)
	admin.GET("/businesses", adminHandler.ListAllBusinesses)
	admin.GET("/businesses/:business_id", adminHandler.GetBusinessDetail)
	admin.PUT("/businesses/:business_id/status", adminHandler.UpdateBusinessStatus)
	admin.DELETE("/businesses/:business_id", adminHandler.DeleteBusiness)

	// System endpoints — super_admin only.
	superOnly := authMW.RequireSuperAdmin()
	admin.GET("/system/build-info", superOnly, systemHandler.BuildInfo)
	admin.GET("/system/health", superOnly, systemHandler.ServiceHealth)
	admin.GET("/system/table-stats", superOnly, systemHandler.TableStats)
	admin.GET("/system/sessions", superOnly, systemHandler.SessionsList)
	admin.POST("/system/sessions/:session_id/revoke", superOnly, systemHandler.SessionRevoke)
	admin.GET("/system/flags", superOnly, systemHandler.FlagsList)
	admin.PUT("/system/flags/:key", superOnly, systemHandler.FlagsToggle)
	admin.GET("/system/denylist-stats", superOnly, systemHandler.DenylistStats)

	return r
}

