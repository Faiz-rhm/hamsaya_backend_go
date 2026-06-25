// Command winback-blast runs a one-time re-engagement campaign over EXISTING
// dormant users: each gets a win-back email (the reliable channel in
// Afghanistan, where FCM is DNS-blocked) plus a win-back push (direct APNs for
// iOS, FCM for Android — delivered only to live tokens).
//
// Unlike the recurring hourly win-back (14-day threshold, push-only), this is a
// deliberate manual blast you point at your current user base. It is:
//
//   - Dry-run by DEFAULT. Prints who WOULD be contacted and exits without
//     sending. Re-run with -dry-run=false to actually send. Sending is
//     irreversible — you cannot unsend an email or push.
//   - Deduped via Redis (winback:blast:<userID>, 30-day TTL), so re-running
//     after a crash/partial send never double-contacts anyone.
//   - Throttled (-rate) to stay under provider rate limits and protect sender
//     reputation on a sudden bulk send.
//   - Verified-only and dormant-only (-days), so currently-active users and
//     unverified/garbage addresses are skipped.
//
// Examples:
//
//	go run cmd/winback-blast/main.go                      # dry-run, 7-day dormancy
//	go run cmd/winback-blast/main.go -days 7 -dry-run=false
//	go run cmd/winback-blast/main.go -days 30 -limit 500 -dry-run=false
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/notification"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	var (
		dormantDays = flag.Int("days", 7, "contact users whose last login is older than this many days (or who never logged in)")
		limit       = flag.Int("limit", 50000, "max users to contact in this run")
		rateMs      = flag.Int("rate", 120, "milliseconds to wait between users (throttle; 120ms ≈ 8/s)")
		dryRun      = flag.Bool("dry-run", true, "when true (default), only print what would be sent and exit without sending")
		noEmail     = flag.Bool("no-email", false, "skip the email channel")
		noPush      = flag.Bool("no-push", false, "skip the push channel")
	)
	flag.Parse()

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

	// FCM (Android) — optional.
	var fcmClient *notification.FCMClient
	fcmCfg := notification.FCMConfig{
		CredentialsPath: cfg.Firebase.CredentialsPath,
		ProjectID:       cfg.Firebase.ProjectID,
		PrivateKey:      cfg.Firebase.PrivateKey,
		ClientEmail:     cfg.Firebase.ClientEmail,
	}
	if fcmCfg.CredentialsPath != "" || (fcmCfg.ProjectID != "" && fcmCfg.PrivateKey != "" && fcmCfg.ClientEmail != "") {
		if fcmClient, err = notification.NewFCMClient(fcmCfg, logger); err != nil {
			logger.Warn("FCM init failed; Android pushes skipped", zap.Error(err))
			fcmClient = nil
		}
	} else {
		logger.Warn("FCM not configured; Android pushes skipped")
	}

	// Direct APNs (iOS) — optional but the only iOS channel that works in AF.
	var apnsClient *notification.APNsClient
	apnsCfg := notification.APNsConfig{
		KeyP8:      cfg.APNs.KeyP8,
		KeyID:      cfg.APNs.KeyID,
		TeamID:     cfg.APNs.TeamID,
		BundleID:   cfg.APNs.BundleID,
		Production: cfg.APNs.Production,
	}
	if apnsCfg.KeyP8 != "" && apnsCfg.KeyID != "" && apnsCfg.TeamID != "" && apnsCfg.BundleID != "" {
		if apnsClient, err = notification.NewAPNsClient(apnsCfg, logger); err != nil {
			logger.Warn("APNs init failed; iOS pushes skipped", zap.Error(err))
			apnsClient = nil
		}
	} else {
		logger.Warn("APNs not configured; iOS pushes skipped")
	}

	notificationRepo := repositories.NewNotificationRepository(db)
	settingsRepo := repositories.NewNotificationSettingsRepository(db)
	userRepo := repositories.NewUserRepository(db)
	notifSvc := services.NewNotificationService(notificationRepo, settingsRepo, userRepo, fcmClient, redisClient, nil, logger).
		WithAPNs(apnsClient)
	emailSvc := services.NewEmailService(&cfg.Email, logger)

	// Dormant, verified, non-deleted users with a usable email address.
	rows, err := db.Pool.Query(ctx, `
		SELECT u.id, u.email,
		       COALESCE(NULLIF(TRIM(pr.first_name), ''), '') AS first_name,
		       COALESCE(NULLIF(TRIM(pr.province),   ''), '') AS province
		FROM users u
		LEFT JOIN profiles pr ON pr.id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.email_verified = true
		  AND u.email IS NOT NULL AND TRIM(u.email) <> ''
		  AND (u.last_login_at IS NULL OR u.last_login_at < NOW() - make_interval(days => $1))
		ORDER BY u.last_login_at ASC NULLS FIRST
		LIMIT $2
	`, *dormantDays, *limit)
	if err != nil {
		logger.Fatal("winback-blast query failed", zap.Error(err))
	}

	type target struct{ userID, email, firstName, province string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.userID, &t.email, &t.firstName, &t.province); err != nil {
			logger.Error("scan row", zap.Error(err))
			continue
		}
		targets = append(targets, t)
	}
	rows.Close()

	mode := "LIVE"
	if *dryRun {
		mode = "DRY-RUN (no sends — pass -dry-run=false to send)"
	}
	logger.Info("winback-blast starting",
		zap.String("mode", mode),
		zap.Int("dormant_days", *dormantDays),
		zap.Int("candidates", len(targets)),
		zap.Bool("email", !*noEmail),
		zap.Bool("push", !*noPush),
	)

	var emailed, pushed, skipped, failed int
	for _, t := range targets {
		// Dedup: claim this user for the campaign. SetNX false → already done.
		dedupKey := "winback:blast:" + t.userID
		if !*dryRun {
			ok, derr := redisClient.SetNX(ctx, dedupKey, "1", 30*24*time.Hour).Result()
			if derr != nil {
				logger.Warn("dedup setnx failed; sending anyway", zap.String("user_id", t.userID), zap.Error(derr))
			} else if !ok {
				skipped++
				continue
			}
		}

		// Neighborhood activity hook.
		recent := 0
		if t.province != "" {
			_ = db.Pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM posts
				WHERE deleted_at IS NULL
				  AND created_at > NOW() - INTERVAL '7 days'
				  AND province ILIKE $1
			`, t.province).Scan(&recent)
		}

		if *dryRun {
			logger.Info("would contact",
				zap.String("email", t.email),
				zap.String("province", t.province),
				zap.Int("recent_posts", recent),
			)
			emailed++
			pushed++
			continue
		}

		if !*noEmail {
			if err := emailSvc.SendWinbackEmail(t.email, t.firstName, t.province, recent); err != nil {
				logger.Error("winback email failed", zap.String("user_id", t.userID), zap.Error(err))
				failed++
			} else {
				emailed++
			}
		}

		if !*noPush {
			title := "Your neighborhood missed you"
			if t.firstName != "" {
				title = fmt.Sprintf("%s, your neighborhood missed you", t.firstName)
			}
			var msg string
			switch {
			case recent > 0 && t.province != "":
				msg = fmt.Sprintf("%d new posts in %s this week. Come see what's happening.", recent, t.province)
			case t.province != "":
				msg = fmt.Sprintf("See what's new in %s on Hamsaya.", t.province)
			default:
				msg = "See what's new in your neighborhood on Hamsaya."
			}
			if _, err := notifSvc.CreateNotification(ctx, &models.CreateNotificationRequest{
				UserID:  t.userID,
				Type:    models.NotificationTypeWinback,
				Title:   &title,
				Message: &msg,
				Data: map[string]interface{}{
					"type":   string(models.NotificationTypeWinback),
					"action": "open_feed",
				},
			}); err != nil {
				logger.Error("winback push failed", zap.String("user_id", t.userID), zap.Error(err))
			} else {
				pushed++
			}
		}

		time.Sleep(time.Duration(*rateMs) * time.Millisecond)
	}

	logger.Info("winback-blast done",
		zap.String("mode", mode),
		zap.Int("emailed", emailed),
		zap.Int("pushed", pushed),
		zap.Int("skipped_already_sent", skipped),
		zap.Int("failed", failed),
	)

	// CreateNotification dispatches push on a goroutine; let them flush.
	if !*dryRun {
		time.Sleep(8 * time.Second)
	}
}
