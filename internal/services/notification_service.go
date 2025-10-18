package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	fcmclient "github.com/hamsaya/backend/pkg/notification"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	fcmTokenPrefix = "fcm:token:"
	fcmTokenTTL    = 90 * 24 * time.Hour // 90 days
)

// NotificationService handles notification operations
type NotificationService struct {
	notificationRepo repositories.NotificationRepository
	settingsRepo     repositories.NotificationSettingsRepository
	fcmClient        *fcmclient.FCMClient
	redisClient      *redis.Client
	logger           *zap.Logger
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	notificationRepo repositories.NotificationRepository,
	settingsRepo repositories.NotificationSettingsRepository,
	fcmClient *fcmclient.FCMClient,
	redisClient *redis.Client,
	logger *zap.Logger,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		settingsRepo:     settingsRepo,
		fcmClient:        fcmClient,
		redisClient:      redisClient,
		logger:           logger,
	}
}

// CreateNotification creates a notification and sends push notification if enabled
func (s *NotificationService) CreateNotification(ctx context.Context, req *models.CreateNotificationRequest) (*models.NotificationResponse, error) {
	// Create notification in database
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

	// Send push notification asynchronously
	go s.sendPushNotification(ctx, notification)

	return notification.ToNotificationResponse(), nil
}

// GetNotifications retrieves notifications for a user
func (s *NotificationService) GetNotifications(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*models.NotificationResponse, error) {
	filter := &models.GetNotificationsFilter{
		UserID:     userID,
		UnreadOnly: unreadOnly,
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

	var responses []*models.NotificationResponse
	for _, notification := range notifications {
		responses = append(responses, notification.ToNotificationResponse())
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
	return nil
}

// GetUnreadCount gets the count of unread notifications
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	count, err := s.notificationRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get unread count",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return 0, utils.NewInternalError("Failed to get unread count", err)
	}

	return count, nil
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
		} else {
			settings, _ = s.settingsRepo.GetByProfileID(ctx, profileID)
		}
	}

	return settings, nil
}

// UpdateNotificationSetting updates a notification setting
func (s *NotificationService) UpdateNotificationSetting(ctx context.Context, profileID string, req *models.UpdateNotificationSettingsRequest) error {
	if err := s.settingsRepo.UpdateCategory(ctx, profileID, req.Category, req.PushPref); err != nil {
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

// RegisterFCMToken registers an FCM token for a user
func (s *NotificationService) RegisterFCMToken(ctx context.Context, userID, token string) error {
	key := fcmTokenPrefix + userID

	// Store token in Redis with TTL
	if err := s.redisClient.Set(ctx, key, token, fcmTokenTTL).Err(); err != nil {
		s.logger.Error("Failed to register FCM token",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return utils.NewInternalError("Failed to register device token", err)
	}

	s.logger.Info("FCM token registered", zap.String("user_id", userID))
	return nil
}

// UnregisterFCMToken removes an FCM token for a user
func (s *NotificationService) UnregisterFCMToken(ctx context.Context, userID string) error {
	key := fcmTokenPrefix + userID

	if err := s.redisClient.Del(ctx, key).Err(); err != nil {
		s.logger.Error("Failed to unregister FCM token",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return utils.NewInternalError("Failed to unregister device token", err)
	}

	s.logger.Info("FCM token unregistered", zap.String("user_id", userID))
	return nil
}

// sendPushNotification sends a push notification via FCM
func (s *NotificationService) sendPushNotification(ctx context.Context, notification *models.Notification) {
	// Get FCM token for user
	key := fcmTokenPrefix + notification.UserID
	token, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			s.logger.Debug("No FCM token found for user", zap.String("user_id", notification.UserID))
		} else {
			s.logger.Warn("Failed to get FCM token", zap.Error(err), zap.String("user_id", notification.UserID))
		}
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
		Title: title,
		Body:  body,
		Data:  data,
		Sound: "default",
	}

	// Send notification
	if err := s.fcmClient.SendNotification(ctx, token, payload); err != nil {
		s.logger.Error("Failed to send push notification",
			zap.Error(err),
			zap.String("user_id", notification.UserID),
			zap.String("notification_id", notification.ID),
		)
	} else {
		s.logger.Info("Push notification sent successfully",
			zap.String("user_id", notification.UserID),
			zap.String("notification_id", notification.ID),
		)
	}
}
