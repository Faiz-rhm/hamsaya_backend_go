package services

import (
	"context"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/redis/go-redis/v9"
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

	// Optional — set via WithEmail to enable the unread-activity digest email.
	email *EmailService
	rdb   *redis.Client
}

// NewEngagementService constructs an EngagementService.
func NewEngagementService(db *database.DB, notif *NotificationService, logger *zap.Logger) *EngagementService {
	return &EngagementService{db: db, notif: notif, logger: logger}
}

// WithEmail enables the unread-activity digest email job. Requires both an
// EmailService (to send) and a Redis client (to dedupe sends). No-op job
// without it — the other re-engagement jobs are unaffected.
func (s *EngagementService) WithEmail(email *EmailService, rdb *redis.Client) *EngagementService {
	s.email = email
	s.rdb = rdb
	return s
}

// RunHourly runs every re-engagement job once. Intended to be invoked hourly.
// Each job logs and swallows its own errors so one failure doesn't block the
// others; the returned error is always nil to keep scheduler callers simple.
func (s *EngagementService) RunHourly(ctx context.Context) error {
	ev := s.sendEventReminders(ctx)
	wb := s.sendWinback(ctx)
	sx := s.sendSellExpiring(ctx)
	ud := s.sendUnreadDigest(ctx)
	if ev+wb+sx+ud > 0 {
		s.logger.Info("engagement run complete",
			zap.Int("event_reminders", ev),
			zap.Int("winback", wb),
			zap.Int("sell_expiring", sx),
			zap.Int("unread_digest", ud),
		)
	}
	return nil
}

// sendUnreadDigest emails users who have messages and/or notifications that have
// sat unread for 2+ days. Deduped via Redis so a user gets at most one digest
// per 2 days. No-op unless WithEmail wired an EmailService + Redis client.
//
// Why email (not push): push can be unreliable for this audience (Google
// blocked in Afghanistan), so an email is the dependable re-engagement channel
// for someone who hasn't opened the app in a couple of days.
func (s *EngagementService) sendUnreadDigest(ctx context.Context) int {
	if s.email == nil || s.rdb == nil {
		return 0
	}

	// Qualify: verified, real email (exclude OAuth placeholder inboxes), not
	// deleted, with at least one unread message OR notification ≥2 days old.
	// Counts are total-unread for the email copy. Bounded per run.
	const query = `
		SELECT u.id, u.email,
		       COALESCE(NULLIF(TRIM(pr.first_name), ''), '') AS first_name,
		       (SELECT COUNT(*) FROM notifications n
		          WHERE n.user_id = u.id AND n.read = false) AS unread_notifs,
		       (SELECT COUNT(*) FROM messages m
		          JOIN conversations c ON c.id = m.conversation_id
		          WHERE (c.participant1_id = u.id OR c.participant2_id = u.id)
		            AND m.sender_id <> u.id AND m.read_at IS NULL) AS unread_msgs
		FROM users u
		LEFT JOIN profiles pr ON pr.id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.email_verified = true
		  AND u.email NOT LIKE '%@no-email.hamsaya.af'
		  AND (
		    EXISTS (
		      SELECT 1 FROM notifications n
		      WHERE n.user_id = u.id AND n.read = false
		        AND n.created_at <= NOW() - INTERVAL '2 days'
		    )
		    OR EXISTS (
		      SELECT 1 FROM messages m
		      JOIN conversations c ON c.id = m.conversation_id
		      WHERE (c.participant1_id = u.id OR c.participant2_id = u.id)
		        AND m.sender_id <> u.id AND m.read_at IS NULL
		        AND m.created_at <= NOW() - INTERVAL '2 days'
		    )
		  )
		LIMIT 500
	`

	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		s.logger.Error("unread digest query failed", zap.Error(err))
		return 0
	}
	defer rows.Close()

	type target struct {
		userID, email, firstName string
		notifs, msgs             int
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.userID, &t.email, &t.firstName, &t.notifs, &t.msgs); err != nil {
			s.logger.Error("scan unread digest row", zap.Error(err))
			continue
		}
		targets = append(targets, t)
	}

	sent := 0
	for _, t := range targets {
		// Dedup: at most one digest per user per 2 days. SetNX returns false if
		// the key already exists (already emailed within the window) → skip.
		key := "engagement:unread-digest:" + t.userID
		ok, err := s.rdb.SetNX(ctx, key, "1", 48*time.Hour).Result()
		if err != nil {
			s.logger.Warn("unread digest dedup failed", zap.String("user_id", t.userID), zap.Error(err))
			continue
		}
		if !ok {
			continue // already emailed recently
		}

		if err := s.email.SendUnreadDigestEmail(t.email, t.firstName, t.notifs, t.msgs); err != nil {
			s.logger.Error("send unread digest email", zap.String("user_id", t.userID), zap.Error(err))
			// Release the dedup key so the next run can retry this user.
			_ = s.rdb.Del(ctx, key).Err()
			continue
		}
		sent++
	}
	return sent
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
