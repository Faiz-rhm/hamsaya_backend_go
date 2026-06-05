package services

import (
	"context"
	"fmt"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"go.uber.org/zap"
)

// EngagementService runs proactive re-engagement jobs that bring users back to
// the app: event reminders, dormant-user win-back, and sell expiring-soon
// nudges. All jobs are idempotent and deduped against the notifications table,
// so RunHourly is safe to call on a schedule (and from multiple instances when
// leader-elected). Notifications are created through NotificationService, which
// persists them in-app and dispatches the FCM push subject to the user's push
// preference, quiet hours, and frequency cap.
type EngagementService struct {
	db     *database.DB
	notif  *NotificationService
	logger *zap.Logger
}

// NewEngagementService constructs an EngagementService.
func NewEngagementService(db *database.DB, notif *NotificationService, logger *zap.Logger) *EngagementService {
	return &EngagementService{db: db, notif: notif, logger: logger}
}

// RunHourly runs every re-engagement job once. Intended to be invoked hourly.
// Each job logs and swallows its own errors so one failure doesn't block the
// others; the returned error is always nil to keep scheduler callers simple.
func (s *EngagementService) RunHourly(ctx context.Context) error {
	ev := s.sendEventReminders(ctx)
	wb := s.sendWinback(ctx)
	sx := s.sendSellExpiring(ctx)
	if ev+wb+sx > 0 {
		s.logger.Info("engagement run complete",
			zap.Int("event_reminders", ev),
			zap.Int("winback", wb),
			zap.Int("sell_expiring", sx),
		)
	}
	return nil
}

// sendEventReminders notifies RSVP'd users 24h and 1h before an event starts.
func (s *EngagementService) sendEventReminders(ctx context.Context) int {
	type window struct {
		key, label, fromExpr, toExpr string
	}
	windows := []window{
		{key: "24h", label: "in 24 hours", fromExpr: "23 hours", toExpr: "24 hours"},
		{key: "1h", label: "in 1 hour", fromExpr: "0 hours", toExpr: "1 hour"},
	}

	total := 0
	for _, w := range windows {
		// Event start = start_date + start_time. Notify each RSVP'd user once
		// per (event, window), skipping the event owner. NOTE: post type is
		// stored uppercase ('EVENT').
		query := fmt.Sprintf(`
			SELECT ei.user_id, p.id, COALESCE(NULLIF(TRIM(p.title), ''), 'Your event') AS title
			FROM posts p
			JOIN event_interests ei ON ei.post_id = p.id
				AND ei.event_state IN ('interested', 'going')
			WHERE p.type = 'EVENT'
			  AND p.deleted_at IS NULL
			  AND p.start_date IS NOT NULL
			  AND p.start_time IS NOT NULL
			  AND (p.start_date + p.start_time) >= NOW() + INTERVAL '%s'
			  AND (p.start_date + p.start_time) <  NOW() + INTERVAL '%s'
			  AND ei.user_id <> COALESCE(p.user_id, '00000000-0000-0000-0000-000000000000')
			  AND NOT EXISTS (
				SELECT 1 FROM notifications n
				WHERE n.user_id = ei.user_id
				  AND n.type = 'EVENT_REMINDER'
				  AND n.data->>'post_id' = p.id::text
				  AND n.data->>'window' = $1
			  )
		`, w.fromExpr, w.toExpr)

		rows, err := s.db.Pool.Query(ctx, query, w.key)
		if err != nil {
			s.logger.Error("event reminder query failed", zap.String("window", w.key), zap.Error(err))
			continue
		}
		type target struct{ userID, postID, title string }
		var targets []target
		for rows.Next() {
			var t target
			if err := rows.Scan(&t.userID, &t.postID, &t.title); err != nil {
				s.logger.Error("scan event reminder row", zap.Error(err))
				continue
			}
			targets = append(targets, t)
		}
		rows.Close()

		for _, t := range targets {
			title := "Event reminder"
			msg := fmt.Sprintf("%s starts %s", t.title, w.label)
			if _, err := s.notif.CreateNotification(ctx, &models.CreateNotificationRequest{
				UserID:  t.userID,
				Type:    models.NotificationTypeEventReminder,
				Title:   &title,
				Message: &msg,
				Data: map[string]interface{}{
					"type":    string(models.NotificationTypeEventReminder),
					"post_id": t.postID,
					"window":  w.key,
					"action":  "view_event",
				},
			}); err != nil {
				s.logger.Error("create event reminder", zap.String("user_id", t.userID), zap.Error(err))
				continue
			}
			total++
		}
	}
	return total
}

// sendWinback nudges users inactive for 14+ days with a localized message.
func (s *EngagementService) sendWinback(ctx context.Context) int {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT u.id, COALESCE(NULLIF(TRIM(pr.first_name), ''), '') AS first_name,
		       COALESCE(NULLIF(TRIM(pr.province), ''), '') AS province
		FROM users u
		LEFT JOIN profiles pr ON pr.id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.email_verified = true
		  AND (u.last_login_at IS NULL OR u.last_login_at < NOW() - INTERVAL '14 days')
		  AND NOT EXISTS (
			SELECT 1 FROM notifications n
			WHERE n.user_id = u.id
			  AND n.type = 'WINBACK'
			  AND n.created_at > NOW() - INTERVAL '7 days'
		  )
		ORDER BY u.last_login_at ASC NULLS FIRST
		LIMIT 2000
	`)
	if err != nil {
		s.logger.Error("winback query failed", zap.Error(err))
		return 0
	}
	type target struct{ userID, firstName, province string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.userID, &t.firstName, &t.province); err != nil {
			s.logger.Error("scan winback row", zap.Error(err))
			continue
		}
		targets = append(targets, t)
	}
	rows.Close()

	total := 0
	for _, t := range targets {
		recent := 0
		if t.province != "" {
			_ = s.db.Pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM posts
				WHERE deleted_at IS NULL
				  AND created_at > NOW() - INTERVAL '7 days'
				  AND province ILIKE $1
			`, t.province).Scan(&recent)
		}

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

		if _, err := s.notif.CreateNotification(ctx, &models.CreateNotificationRequest{
			UserID:  t.userID,
			Type:    models.NotificationTypeWinback,
			Title:   &title,
			Message: &msg,
			Data: map[string]interface{}{
				"type":   string(models.NotificationTypeWinback),
				"action": "open_feed",
			},
		}); err != nil {
			s.logger.Error("create winback", zap.String("user_id", t.userID), zap.Error(err))
			continue
		}
		total++
	}
	return total
}

// sendSellExpiring nudges sellers ~48h before an active, unsold listing expires
// so they can repost/renew it — keeps the marketplace fresh and brings sellers
// back. Deduped per post within 3 days.
func (s *EngagementService) sendSellExpiring(ctx context.Context) int {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT p.user_id, p.id, COALESCE(NULLIF(TRIM(p.title), ''), 'Your listing') AS title
		FROM posts p
		WHERE p.type = 'SELL'
		  AND p.deleted_at IS NULL
		  AND p.status = true
		  AND p.sold = false
		  AND p.user_id IS NOT NULL
		  AND p.expired_at IS NOT NULL
		  AND p.expired_at >= NOW() + INTERVAL '47 hours'
		  AND p.expired_at <  NOW() + INTERVAL '48 hours'
		  AND NOT EXISTS (
			SELECT 1 FROM notifications n
			WHERE n.user_id = p.user_id
			  AND n.type = 'SELL_EXPIRING'
			  AND n.data->>'post_id' = p.id::text
			  AND n.created_at > NOW() - INTERVAL '3 days'
		  )
	`)
	if err != nil {
		s.logger.Error("sell expiring query failed", zap.Error(err))
		return 0
	}
	type target struct{ userID, postID, title string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.userID, &t.postID, &t.title); err != nil {
			s.logger.Error("scan sell expiring row", zap.Error(err))
			continue
		}
		targets = append(targets, t)
	}
	rows.Close()

	total := 0
	for _, t := range targets {
		title := "Listing expiring soon"
		msg := fmt.Sprintf("\"%s\" expires in 2 days. Repost it to keep it visible.", t.title)
		if _, err := s.notif.CreateNotification(ctx, &models.CreateNotificationRequest{
			UserID:  t.userID,
			Type:    models.NotificationTypeSellExpiring,
			Title:   &title,
			Message: &msg,
			Data: map[string]interface{}{
				"type":    string(models.NotificationTypeSellExpiring),
				"post_id": t.postID,
				"action":  "view_listing",
			},
		}); err != nil {
			s.logger.Error("create sell expiring", zap.String("user_id", t.userID), zap.Error(err))
			continue
		}
		total++
	}
	return total
}
