// Command scheduled-engagement sends proactive re-engagement push notifications:
//
//   - Event reminders: T-24h and T-1h before an event a user RSVP'd to.
//   - Dormant win-back: users inactive for 14+ days get a localized nudge.
//   - Sell expiring-soon: sellers nudged ~48h before a listing expires.
//
// The server already runs this hourly in-process (leader-elected). This binary
// is for manual/ad-hoc runs and out-of-process scheduling. Idempotent + deduped.
//
// Run manually:  go run cmd/scheduled-engagement/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/notification"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

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
	logger := utils.GetBaseLogger()

	ctx := context.Background()

	db, err := database.New(&cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() { _ = redisClient.Close() }()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// FCM is optional: without it, notifications are still persisted in-app.
	var fcmClient *notification.FCMClient
	fcmCfg := notification.FCMConfig{
		CredentialsPath: cfg.Firebase.CredentialsPath,
		ProjectID:       cfg.Firebase.ProjectID,
		PrivateKey:      cfg.Firebase.PrivateKey,
		ClientEmail:     cfg.Firebase.ClientEmail,
	}
	if fcmCfg.CredentialsPath != "" || (fcmCfg.ProjectID != "" && fcmCfg.PrivateKey != "" && fcmCfg.ClientEmail != "") {
		fcmClient, err = notification.NewFCMClient(fcmCfg, logger)
		if err != nil {
			logger.Warn("FCM client init failed; pushes will be skipped", zap.Error(err))
			fcmClient = nil
		}
	} else {
		logger.Warn("FCM not configured; pushes will be skipped")
	}

	notificationRepo := repositories.NewNotificationRepository(db)
	settingsRepo := repositories.NewNotificationSettingsRepository(db)
	userRepo := repositories.NewUserRepository(db)
	notifSvc := services.NewNotificationService(notificationRepo, settingsRepo, userRepo, fcmClient, redisClient, nil, logger)
	engagement := services.NewEngagementService(db, notifSvc, logger)

	if err := engagement.RunHourly(ctx); err != nil {
		logger.Error("engagement run failed", zap.Error(err))
	}

	// CreateNotification dispatches the FCM push on a goroutine; give those a
	// moment to flush before the process exits and closes db/redis.
	time.Sleep(8 * time.Second)
}
