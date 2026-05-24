package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/cache"
	fcmclient "github.com/hamsaya/backend/pkg/notification"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// fcmTokensPrefix keys a Redis SET of every FCM token registered for a
	// user. One entry per active device (iOS, Android, web). Using a set
	// instead of a single string fixes the "Android push works, iOS does
	// not" symptom: the previous string key let the most-recently-
	// registered token clobber the others, so users signed into both
	// platforms only received pushes on whichever device registered last.
	fcmTokensPrefix = "fcm:tokens:"
	fcmTokenTTL     = 90 * 24 * time.Hour // 90 days

	// unreadCountTTL keeps the badge counter cached briefly. The mobile
	// app polls the unread-count endpoint frequently for the bell badge;
	// 30 seconds is long enough to drop most of that load and short
	// enough that staleness is invisible (a new notification arrives via
	// WS, which also invalidates the cache).
	unreadCountTTL = 30 * time.Second
)

// NotificationService handles notification operations
type NotificationService struct {
	notificationRepo repositories.NotificationRepository
	settingsRepo     repositories.NotificationSettingsRepository
	userRepo         repositories.UserRepository
	fcmClient        *fcmclient.FCMClient
	redisClient      *redis.Client
	wsHub            *websocket.Hub
	logger           *zap.Logger
	cache            *cache.Cache // optional; nil = no caching for unread-count
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	notificationRepo repositories.NotificationRepository,
	settingsRepo repositories.NotificationSettingsRepository,
	userRepo repositories.UserRepository,
	fcmClient *fcmclient.FCMClient,
	redisClient *redis.Client,
	wsHub *websocket.Hub,
	logger *zap.Logger,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		settingsRepo:     settingsRepo,
		userRepo:         userRepo,
		fcmClient:        fcmClient,
		redisClient:      redisClient,
		wsHub:            wsHub,
		logger:           logger,
	}
}

// WithCache attaches a cache namespace. Call once at startup. Optional —
// without it, every unread-count poll hits Postgres directly.
func (s *NotificationService) WithCache(c *cache.Cache) *NotificationService {
	s.cache = c
	return s
}

// unreadCountKey builds a per-(user, businessScope) cache key. Empty
// business scope = personal notifications.
func unreadCountKey(userID string, businessID *string) string {
	scope := "user"
	if businessID != nil && *businessID != "" {
		scope = "biz:" + *businessID
	}
	return "unread:" + userID + ":" + scope
}

// invalidateUnreadForUser drops cached counts for every scope variant a
// given user might query. Called after any write that could change the
// unread state for that user.
func (s *NotificationService) invalidateUnreadForUser(ctx context.Context, userID string) {
	if s.cache == nil {
		return
	}
	s.cache.DelPattern(ctx, "unread:"+userID+":*")
}

// channelForType returns the Android notification channel ID for the type.
func channelForType(t models.NotificationType) string {
	switch t {
	case models.NotificationTypeMessage:
		return "messages"
	case models.NotificationTypeEventInterest, models.NotificationTypeEventGoing:
		return "events"
	case models.NotificationTypeWelcome,
		models.NotificationTypePasswordChanged,
		models.NotificationTypeEmailVerified,
		models.NotificationTypeAccountSuspended,
		models.NotificationTypeAccountUnsuspended,
		models.NotificationTypePostDeletedByAdmin,
		models.NotificationTypeCommentDeletedByAdmin,
		models.NotificationTypeBusinessDeletedByAdmin:
		return "account"
	case models.NotificationTypeSellExpired,
		models.NotificationTypeSellInterested,
		models.NotificationTypeSellSold:
		return "sales"
	default:
		return "general"
	}
}

// typeToCategory maps a notification type to its preference category.
func typeToCategory(t models.NotificationType) models.NotificationCategory {
	switch t {
	case models.NotificationTypeLike, models.NotificationTypeComment,
		models.NotificationTypeCommentReply, models.NotificationTypeCommentLike,
		models.NotificationTypeMention, models.NotificationTypePostShare,
		models.NotificationTypePollVote, models.NotificationTypeFollow,
		models.NotificationTypeNewPost, models.NotificationTypeAdmin:
		return models.NotificationCategoryPosts
	case models.NotificationTypeMessage:
		return models.NotificationCategoryMessages
	case models.NotificationTypeEventInterest, models.NotificationTypeEventGoing:
		return models.NotificationCategoryEvents
	case models.NotificationTypeBusinessFollow,
		models.NotificationTypeBusinessDeletedByAdmin:
		return models.NotificationCategoryBusiness
	case models.NotificationTypeSellExpired,
		models.NotificationTypeSellInterested,
		models.NotificationTypeSellSold:
		return models.NotificationCategorySales
	case models.NotificationTypeWelcome,
		models.NotificationTypePasswordChanged,
		models.NotificationTypeEmailVerified,
		models.NotificationTypeAccountSuspended,
		models.NotificationTypeAccountUnsuspended,
		models.NotificationTypePostDeletedByAdmin,
		models.NotificationTypeCommentDeletedByAdmin:
		return models.NotificationCategoryAccount
	default:
		return models.NotificationCategoryPosts
	}
}

// CreateNotification creates a notification and optionally sends a push via FCM.
// The notification is always saved to the database so the user sees it in the in-app
// notification list regardless of push being enabled, FCM token presence, or push preferences.
// It skips self-notifications and only sends push if the user's per-category push preference allows.
func (s *NotificationService) CreateNotification(ctx context.Context, req *models.CreateNotificationRequest) (*models.NotificationResponse, error) {
	// Don't notify the actor themselves
	if actorID, ok := req.Data["actor_id"]; ok {
		if actorStr, isStr := actorID.(string); isStr && actorStr == req.UserID {
			return nil, nil
		}
	}

	// Always persist so it appears in the notification list (even when push is disabled)
	notificationID := uuid.New().String()
	notification := &models.Notification{
		ID:        notificationID,
		UserID:    req.UserID,
		Type:      req.Type,
		Title:     req.Title,
		Message:   req.Message,
		Data:      req.Data,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		s.logger.Error("Failed to create notification",
			zap.Error(err),
			zap.String("user_id", req.UserID),
		)
		return nil, utils.NewInternalError("Failed to create notification", err)
	}

	s.logger.Info("Notification created",
		zap.String("notification_id", notificationID),
		zap.String("user_id", req.UserID),
		zap.String("type", string(req.Type)),
	)

	// Send real-time notification via WebSocket. We also include the new
	// unread count so the mobile badge updates instantly without an extra
	// API call — same pattern as X/Twitter and Facebook.
	if s.wsHub != nil {
		go func() {
			ctxWS := context.WithoutCancel(ctx)
			unread, _ := s.notificationRepo.GetUnreadCount(ctxWS, req.UserID, nil)
			wsPayload := map[string]interface{}{
				"type":          "notification",
				"payload":       notification.ToNotificationResponse(),
				"unread_count":  unread,
			}
			if err := s.wsHub.SendToUser(req.UserID, wsPayload); err != nil {
				s.logger.Debug("Failed to send WebSocket notification",
					zap.Error(err),
					zap.String("user_id", req.UserID),
				)
			}
		}()
	}

	// Check user push preference before sending push
	sendPush := true
	category := typeToCategory(req.Type)
	settings, err := s.settingsRepo.GetByProfileID(ctx, req.UserID)
	if err == nil {
		for _, setting := range settings {
			if setting.Category == category {
				sendPush = setting.PushPref
				break
			}
		}
	}

	if sendPush {
		go s.sendPushNotification(context.WithoutCancel(ctx), notification)
	}

	// New unread notification → drop cached counts for this recipient so
	// the badge bumps up on the next poll instead of waiting for the
	// 30-second TTL.
	s.invalidateUnreadForUser(ctx, notification.UserID)

	return notification.ToNotificationResponse(), nil
}

// GetNotifications retrieves notifications for a user. businessID is optional; when set, only notifications with data.business_id equal to it are returned.
// Enriches each notification's data with actor_avatar_color from the actor's profile when missing (e.g. for notifications created before the field existed).
func (s *NotificationService) GetNotifications(ctx context.Context, userID string, unreadOnly bool, limit, offset int, businessID *string) ([]*models.NotificationResponse, error) {
	filter := &models.GetNotificationsFilter{
		UserID:     userID,
		UnreadOnly: unreadOnly,
		BusinessID: businessID,
		Limit:      limit,
		Offset:     offset,
	}

	notifications, err := s.notificationRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get notifications",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return nil, utils.NewInternalError("Failed to get notifications", err)
	}

	responses := make([]*models.NotificationResponse, 0, len(notifications))
	for _, notification := range notifications {
		resp := notification.ToNotificationResponse()
		// Enrich with actor_avatar_color when missing (e.g. old notifications)
		if s.userRepo != nil && resp.Data != nil {
			if actorID, ok := resp.Data["actor_id"]; ok {
				if actorStr, ok := actorID.(string); ok && actorStr != "" {
					existing := resp.Data["actor_avatar_color"]
					if existing == nil || existing == "" {
						if profile, err := s.userRepo.GetProfileByUserID(ctx, actorStr); err == nil && profile.AvatarColor != nil && *profile.AvatarColor != "" {
							// Clone data so we don't mutate the stored notification
							newData := make(map[string]interface{}, len(resp.Data)+1)
							for k, v := range resp.Data {
								newData[k] = v
							}
							newData["actor_avatar_color"] = *profile.AvatarColor
							resp.Data = newData
						}
					}
				}
			}
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	// Verify notification belongs to user
	notification, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return utils.NewNotFoundError("Notification not found", err)
	}

	if notification.UserID != userID {
		return utils.NewForbiddenError("You don't have access to this notification", nil)
	}

	// Mark as read
	if err := s.notificationRepo.MarkAsRead(ctx, notificationID); err != nil {
		s.logger.Error("Failed to mark notification as read",
			zap.Error(err),
			zap.String("notification_id", notificationID),
		)
		return utils.NewInternalError("Failed to mark notification as read", err)
	}

	s.invalidateUnreadForUser(ctx, userID)
	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	if err := s.notificationRepo.MarkAllAsRead(ctx, userID); err != nil {
		s.logger.Error("Failed to mark all notifications as read",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return utils.NewInternalError("Failed to mark all notifications as read", err)
	}

	s.logger.Info("All notifications marked as read", zap.String("user_id", userID))
	s.invalidateUnreadForUser(ctx, userID)
	return nil
}

// DeleteNotification deletes a notification
func (s *NotificationService) DeleteNotification(ctx context.Context, userID, notificationID string) error {
	// Verify notification belongs to user
	notification, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return utils.NewNotFoundError("Notification not found", err)
	}

	if notification.UserID != userID {
		return utils.NewForbiddenError("You don't have access to this notification", nil)
	}

	// Delete notification
	if err := s.notificationRepo.Delete(ctx, notificationID); err != nil {
		s.logger.Error("Failed to delete notification",
			zap.Error(err),
			zap.String("notification_id", notificationID),
		)
		return utils.NewInternalError("Failed to delete notification", err)
	}

	s.logger.Info("Notification deleted", zap.String("notification_id", notificationID))
	s.invalidateUnreadForUser(ctx, userID)
	return nil
}

// GetUnreadCount gets the count of unread notifications. When businessID is set, counts only that business's notifications.
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string, businessID *string) (int, error) {
	key := unreadCountKey(userID, businessID)

	if s.cache != nil {
		var cached int
		if hit, _ := s.cache.Get(ctx, key, &cached); hit {
			return cached, nil
		}
	}

	count, err := s.notificationRepo.GetUnreadCount(ctx, userID, businessID)
	if err != nil {
		s.logger.Error("Failed to get unread count",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return 0, utils.NewInternalError("Failed to get unread count", err)
	}

	if s.cache != nil {
		_ = s.cache.Set(ctx, key, count, unreadCountTTL)
	}
	return count, nil
}

// Default notification categories (all push enabled)
var defaultNotificationCategories = []models.NotificationCategory{
	models.NotificationCategoryPosts,
	models.NotificationCategoryMessages,
	models.NotificationCategoryEvents,
	models.NotificationCategorySales,
	models.NotificationCategoryBusiness,
}

// GetNotificationSettings retrieves notification settings for a user
func (s *NotificationService) GetNotificationSettings(ctx context.Context, profileID string) ([]*models.NotificationSetting, error) {
	settings, err := s.settingsRepo.GetByProfileID(ctx, profileID)
	if err != nil {
		s.logger.Error("Failed to get notification settings",
			zap.Error(err),
			zap.String("profile_id", profileID),
		)
		return nil, utils.NewInternalError("Failed to get notification settings", err)
	}

	// Initialize defaults if no settings exist
	if len(settings) == 0 {
		if err := s.settingsRepo.InitializeDefaults(ctx, profileID); err != nil {
			s.logger.Warn("Failed to initialize default settings", zap.Error(err))
		}
		refetched, err2 := s.settingsRepo.GetByProfileID(ctx, profileID)
		if err2 == nil && len(refetched) > 0 {
			settings = refetched
		}
	}

	// If still empty (e.g. init failed or no profile), return default list so client shows all enabled
	if len(settings) == 0 {
		now := time.Now()
		settings = make([]*models.NotificationSetting, 0, len(defaultNotificationCategories))
		for _, cat := range defaultNotificationCategories {
			settings = append(settings, &models.NotificationSetting{
				ID:        fmt.Sprintf("%s-%s", profileID, cat),
				ProfileID: profileID,
				Category:  cat,
				PushPref:  true,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}

	// Ensure we never return nil (JSON would serialize as null)
	if settings == nil {
		settings = []*models.NotificationSetting{}
	}
	return settings, nil
}

// UpdateNotificationSetting updates a notification setting (upserts so it works when no row exists yet)
func (s *NotificationService) UpdateNotificationSetting(ctx context.Context, profileID string, req *models.UpdateNotificationSettingsRequest) error {
	if err := s.settingsRepo.UpsertCategory(ctx, profileID, req.Category, req.PushPref); err != nil {
		s.logger.Error("Failed to update notification setting",
			zap.Error(err),
			zap.String("profile_id", profileID),
			zap.String("category", string(req.Category)),
		)
		return utils.NewInternalError("Failed to update notification setting", err)
	}

	s.logger.Info("Notification setting updated",
		zap.String("profile_id", profileID),
		zap.String("category", string(req.Category)),
		zap.Bool("push_pref", req.PushPref),
	)

	return nil
}

// RegisterFCMToken adds an FCM token to the user's device-token set. Multiple
// devices (iOS, Android, web) coexist for the same user; previously this was
// a single STRING key per user, which caused the most-recently-registered
// device to silently win and pushes to vanish on every other device.
func (s *NotificationService) RegisterFCMToken(ctx context.Context, userID, token string) error {
	key := fcmTokensPrefix + userID

	if _, err := s.redisClient.SAdd(ctx, key, token).Result(); err != nil {
		s.logger.Error("Failed to register FCM token",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return utils.NewInternalError("Failed to register device token", err)
	}
	// Refresh the set's TTL on every register so an active user keeps
	// their tokens alive. Tokens older than 90 days without a re-register
	// expire alongside the set.
	if err := s.redisClient.Expire(ctx, key, fcmTokenTTL).Err(); err != nil {
		s.logger.Warn("Failed to refresh FCM token set TTL",
			zap.Error(err),
			zap.String("user_id", userID),
		)
	}

	s.logger.Info("FCM token registered", zap.String("user_id", userID))
	return nil
}

// UnregisterFCMToken removes a specific FCM token from the user's device set.
// When `token` is empty (legacy / full sign-out broadcast) the whole set is
// dropped; otherwise only the calling device's entry is removed so other
// active devices keep receiving pushes.
func (s *NotificationService) UnregisterFCMToken(ctx context.Context, userID, token string) error {
	key := fcmTokensPrefix + userID

	if token == "" {
		if err := s.redisClient.Del(ctx, key).Err(); err != nil {
			s.logger.Error("Failed to unregister FCM tokens",
				zap.Error(err),
				zap.String("user_id", userID),
			)
			return utils.NewInternalError("Failed to unregister device tokens", err)
		}
		s.logger.Info("All FCM tokens unregistered", zap.String("user_id", userID))
		return nil
	}

	if err := s.redisClient.SRem(ctx, key, token).Err(); err != nil {
		s.logger.Error("Failed to unregister FCM token",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return utils.NewInternalError("Failed to unregister device token", err)
	}

	s.logger.Info("FCM token unregistered", zap.String("user_id", userID))
	return nil
}

// sendPushNotification sends a push notification via FCM to every device the
// user has registered. Each token is sent individually so failures are
// scoped to a single device — a stale iOS token doesn't suppress an active
// Android device, and vice versa.
func (s *NotificationService) sendPushNotification(ctx context.Context, notification *models.Notification) {
	if s.fcmClient == nil {
		return
	}

	// Get every FCM token for the user (one per active device).
	key := fcmTokensPrefix + notification.UserID
	tokens, err := s.redisClient.SMembers(ctx, key).Result()
	if err != nil {
		s.logger.Warn("Failed to get FCM tokens", zap.Error(err), zap.String("user_id", notification.UserID))
		return
	}
	if len(tokens) == 0 {
		s.logger.Debug("No FCM token found for user", zap.String("user_id", notification.UserID))
		return
	}

	// Prepare push payload
	title := "Notification"
	if notification.Title != nil {
		title = *notification.Title
	}

	body := ""
	if notification.Message != nil {
		body = *notification.Message
	}

	// Convert notification data to string map for FCM
	data := make(map[string]string)
	if notification.Data != nil {
		for k, v := range notification.Data {
			data[k] = fmt.Sprintf("%v", v)
		}
	}
	data["notification_id"] = notification.ID
	data["type"] = string(notification.Type)

	payload := &fcmclient.PushPayload{
		Title:     title,
		Body:      body,
		Data:      data,
		Sound:     "default",
		ChannelID: channelForType(notification.Type),
	}

	for _, token := range tokens {
		if err := s.fcmClient.SendNotification(ctx, token, payload); err != nil {
			if errors.Is(err, fcmclient.ErrTokenInvalid) {
				s.logger.Info("FCM token invalid, pruning",
					zap.String("user_id", notification.UserID),
				)
				if delErr := s.redisClient.SRem(ctx, key, token).Err(); delErr != nil {
					s.logger.Warn("Failed to prune stale FCM token",
						zap.Error(delErr),
						zap.String("user_id", notification.UserID),
					)
				}
				continue
			}
			s.logger.Error("Failed to send push notification",
				zap.Error(err),
				zap.String("user_id", notification.UserID),
				zap.String("notification_id", notification.ID),
			)
			continue
		}
		s.logger.Info("Push notification sent successfully",
			zap.String("user_id", notification.UserID),
			zap.String("notification_id", notification.ID),
		)
	}
}
