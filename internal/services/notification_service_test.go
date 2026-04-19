package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	fcmclient "github.com/hamsaya/backend/pkg/notification"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestNotificationService creates a NotificationService with nil external deps.
func newTestNotificationService(
	notifRepo *mocks.MockNotificationRepository,
	settingsRepo *mocks.MockNotificationSettingsRepository,
	userRepo *mocks.MockUserRepository,
) *NotificationService {
	return NewNotificationService(
		notifRepo,
		settingsRepo,
		userRepo,
		(*fcmclient.FCMClient)(nil),
		(*redis.Client)(nil),
		(*websocket.Hub)(nil),
		zap.NewNop(),
	)
}

// ---------------------------------------------------------------------------
// TestNotificationService_GetNotifications
// ---------------------------------------------------------------------------

func TestNotificationService_GetNotifications(t *testing.T) {
	title := "Hello"
	msg := "World"

	tests := []struct {
		name          string
		userID        string
		setupMocks    func(*mocks.MockNotificationRepository, *mocks.MockUserRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:   "success",
			userID: "user-1",
			setupMocks: func(nr *mocks.MockNotificationRepository, ur *mocks.MockUserRepository) {
				notifs := []*models.Notification{
					{
						ID:        "notif-1",
						UserID:    "user-1",
						Type:      models.NotificationTypeLike,
						Title:     &title,
						Message:   &msg,
						Read:      false,
						CreatedAt: time.Now(),
					},
				}
				nr.On("List", mock.Anything, mock.AnythingOfType("*models.GetNotificationsFilter")).Return(notifs, nil)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:   "empty",
			userID: "user-2",
			setupMocks: func(nr *mocks.MockNotificationRepository, ur *mocks.MockUserRepository) {
				nr.On("List", mock.Anything, mock.AnythingOfType("*models.GetNotificationsFilter")).Return([]*models.Notification{}, nil)
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(notifRepo, userRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			resp, err := svc.GetNotifications(context.Background(), tt.userID, false, 20, 0, nil)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp, tt.expectedCount)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNotificationService_MarkAsRead
// ---------------------------------------------------------------------------

func TestNotificationService_MarkAsRead(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		notificationID string
		setupMocks     func(*mocks.MockNotificationRepository)
		expectError    bool
		expectedError  string
	}{
		{
			name:           "success",
			userID:         "user-1",
			notificationID: "notif-1",
			setupMocks: func(nr *mocks.MockNotificationRepository) {
				notif := &models.Notification{
					ID:        "notif-1",
					UserID:    "user-1",
					Type:      models.NotificationTypeLike,
					Read:      false,
					CreatedAt: time.Now(),
				}
				nr.On("GetByID", mock.Anything, "notif-1").Return(notif, nil)
				nr.On("MarkAsRead", mock.Anything, "notif-1").Return(nil)
			},
			expectError: false,
		},
		{
			name:           "failure — not found",
			userID:         "user-1",
			notificationID: "notif-999",
			setupMocks: func(nr *mocks.MockNotificationRepository) {
				nr.On("GetByID", mock.Anything, "notif-999").Return(nil, errors.New("not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(notifRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			err := svc.MarkAsRead(context.Background(), tt.userID, tt.notificationID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNotificationService_MarkAllAsRead
// ---------------------------------------------------------------------------

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		setupMocks  func(*mocks.MockNotificationRepository)
		expectError bool
	}{
		{
			name:   "success",
			userID: "user-1",
			setupMocks: func(nr *mocks.MockNotificationRepository) {
				nr.On("MarkAllAsRead", mock.Anything, "user-1").Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(notifRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			err := svc.MarkAllAsRead(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNotificationService_DeleteNotification
// ---------------------------------------------------------------------------

func TestNotificationService_DeleteNotification(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		notificationID string
		setupMocks     func(*mocks.MockNotificationRepository)
		expectError    bool
		expectedError  string
	}{
		{
			name:           "success",
			userID:         "user-1",
			notificationID: "notif-1",
			setupMocks: func(nr *mocks.MockNotificationRepository) {
				notif := &models.Notification{
					ID:        "notif-1",
					UserID:    "user-1",
					Type:      models.NotificationTypeLike,
					CreatedAt: time.Now(),
				}
				nr.On("GetByID", mock.Anything, "notif-1").Return(notif, nil)
				nr.On("Delete", mock.Anything, "notif-1").Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(notifRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			err := svc.DeleteNotification(context.Background(), tt.userID, tt.notificationID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNotificationService_GetUnreadCount
// ---------------------------------------------------------------------------

func TestNotificationService_GetUnreadCount(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		businessID    *string
		setupMocks    func(*mocks.MockNotificationRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:       "success",
			userID:     "user-1",
			businessID: nil,
			setupMocks: func(nr *mocks.MockNotificationRepository) {
				nr.On("GetUnreadCount", mock.Anything, "user-1", (*string)(nil)).Return(5, nil)
			},
			expectError:   false,
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(notifRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			count, err := svc.GetUnreadCount(context.Background(), tt.userID, tt.businessID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNotificationService_GetNotificationSettings
// ---------------------------------------------------------------------------

func TestNotificationService_GetNotificationSettings(t *testing.T) {
	tests := []struct {
		name          string
		profileID     string
		setupMocks    func(*mocks.MockNotificationSettingsRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:      "success",
			profileID: "profile-1",
			setupMocks: func(sr *mocks.MockNotificationSettingsRepository) {
				settings := []*models.NotificationSetting{
					{
						ID:        "setting-1",
						ProfileID: "profile-1",
						Category:  models.NotificationCategoryPosts,
						PushPref:  true,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						ID:        "setting-2",
						ProfileID: "profile-1",
						Category:  models.NotificationCategoryMessages,
						PushPref:  true,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
				sr.On("GetByProfileID", mock.Anything, "profile-1").Return(settings, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(settingsRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			settings, err := svc.GetNotificationSettings(context.Background(), tt.profileID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, settings, tt.expectedCount)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNotificationService_UpdateNotificationSetting
// ---------------------------------------------------------------------------

func TestNotificationService_UpdateNotificationSetting(t *testing.T) {
	tests := []struct {
		name        string
		profileID   string
		req         *models.UpdateNotificationSettingsRequest
		setupMocks  func(*mocks.MockNotificationSettingsRepository)
		expectError bool
	}{
		{
			name:      "success",
			profileID: "profile-1",
			req: &models.UpdateNotificationSettingsRequest{
				Category: models.NotificationCategoryPosts,
				PushPref: false,
			},
			setupMocks: func(sr *mocks.MockNotificationSettingsRepository) {
				sr.On("UpsertCategory", mock.Anything, "profile-1", models.NotificationCategoryPosts, false).Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifRepo := new(mocks.MockNotificationRepository)
			settingsRepo := new(mocks.MockNotificationSettingsRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(settingsRepo)

			svc := newTestNotificationService(notifRepo, settingsRepo, userRepo)
			err := svc.UpdateNotificationSetting(context.Background(), tt.profileID, tt.req)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			notifRepo.AssertExpectations(t)
			settingsRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestChannelForType(t *testing.T) {
	assert.Equal(t, "messages", channelForType(models.NotificationTypeMessage))
	assert.Equal(t, "events", channelForType(models.NotificationTypeEventInterest))
	assert.Equal(t, "events", channelForType(models.NotificationTypeEventGoing))
	assert.Equal(t, "general", channelForType(models.NotificationTypeLike))
}

func TestTypeToCategory(t *testing.T) {
	assert.Equal(t, models.NotificationCategoryMessages, typeToCategory(models.NotificationTypeMessage))
	assert.Equal(t, models.NotificationCategoryEvents, typeToCategory(models.NotificationTypeEventInterest))
	assert.Equal(t, models.NotificationCategoryBusiness, typeToCategory(models.NotificationTypeBusinessFollow))
	assert.Equal(t, models.NotificationCategorySales, typeToCategory(models.NotificationTypeSellExpired))
	assert.Equal(t, models.NotificationCategoryPosts, typeToCategory(models.NotificationTypeLike))
}

func TestNotificationService_CreateNotification(t *testing.T) {
	t.Run("self notification skipped", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		settingsRepo := &mocks.MockNotificationSettingsRepository{}
		svc := NewNotificationService(notifRepo, settingsRepo, nil, nil, nil, nil, zap.NewNop())

		result, err := svc.CreateNotification(context.Background(), &models.CreateNotificationRequest{
			UserID: "u-1",
			Type:   models.NotificationTypeLike,
			Data:   map[string]interface{}{"actor_id": "u-1"},
		})

		require.NoError(t, err)
		assert.Nil(t, result) // self-notification: returns nil, nil
		notifRepo.AssertNotCalled(t, "Create")
	})

	t.Run("persist error", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		settingsRepo := &mocks.MockNotificationSettingsRepository{}
		notifRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Notification")).
			Return(errors.New("db error"))

		svc := NewNotificationService(notifRepo, settingsRepo, nil, nil, nil, nil, zap.NewNop())
		title := "New Like"
		_, err := svc.CreateNotification(context.Background(), &models.CreateNotificationRequest{
			UserID: "u-2",
			Type:   models.NotificationTypeLike,
			Title:  &title,
			Data:   map[string]interface{}{"actor_id": "u-1"},
		})

		require.Error(t, err)
		notifRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		settingsRepo := &mocks.MockNotificationSettingsRepository{}
		notifRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Notification")).Return(nil)
		settingsRepo.On("GetByProfileID", mock.Anything, "u-2").Return([]*models.NotificationSetting{}, nil)

		svc := NewNotificationService(notifRepo, settingsRepo, nil, nil, nil, nil, zap.NewNop())
		title := "New Like"
		result, err := svc.CreateNotification(context.Background(), &models.CreateNotificationRequest{
			UserID: "u-2",
			Type:   models.NotificationTypeLike,
			Title:  &title,
			Data:   map[string]interface{}{"actor_id": "u-1"},
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
		notifRepo.AssertExpectations(t)
	})
}
