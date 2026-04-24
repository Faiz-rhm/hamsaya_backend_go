package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	notifTestUserID   = "notif-user-001"
	notifTestNotifID  = "notif-notif-001"
)

func newNotificationRouter(
	t *testing.T,
	notifRepo *mocks.MockNotificationRepository,
	settingsRepo *mocks.MockNotificationSettingsRepository,
) *gin.Engine {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := services.NewNotificationService(
		notifRepo,
		settingsRepo,
		&mocks.MockUserRepository{},
		nil, // fcmClient nil-guarded
		rdb,
		nil, // wsHub nil-guarded
		zap.NewNop(),
	)
	h := NewNotificationHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(notifTestUserID, "notif-sess-001")
	r := gin.New()
	r.GET("/api/v1/notifications", authed, h.GetNotifications)
	r.GET("/api/v1/notifications/unread-count", authed, h.GetUnreadCount)
	r.POST("/api/v1/notifications/:notification_id/read", authed, h.MarkAsRead)
	r.POST("/api/v1/notifications/read-all", authed, h.MarkAllAsRead)
	r.DELETE("/api/v1/notifications/:notification_id", authed, h.DeleteNotification)
	r.GET("/api/v1/notifications/settings", authed, h.GetNotificationSettings)
	r.PUT("/api/v1/notifications/settings", authed, h.UpdateNotificationSetting)
	r.POST("/api/v1/notifications/fcm-token", authed, h.RegisterFCMToken)
	r.DELETE("/api/v1/notifications/fcm-token", authed, h.UnregisterFCMToken)

	r.GET("/api/v1/noauth/notifications", h.GetNotifications)
	return r
}

// --- GetNotifications ---

func TestNotificationHandler_GetNotifications(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, &mocks.MockNotificationSettingsRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/noauth/notifications", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("success empty", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notifRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetNotificationsFilter")).
			Return([]*models.Notification{}, nil)
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
		notifRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notifRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetNotificationsFilter")).
			Return(nil, fmt.Errorf("db error"))
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- GetUnreadCount ---

func TestNotificationHandler_GetUnreadCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notifRepo.On("GetUnreadCount", mock.Anything, notifTestUserID, (*string)(nil)).Return(5, nil)
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		notifRepo.AssertExpectations(t)
	})
}

// --- MarkAsRead ---

func TestNotificationHandler_MarkAsRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notif := &models.Notification{ID: notifTestNotifID, UserID: notifTestUserID}
		notifRepo.On("GetByID", mock.Anything, notifTestNotifID).Return(notif, nil)
		notifRepo.On("MarkAsRead", mock.Anything, notifTestNotifID).Return(nil)
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/notifications/"+notifTestNotifID+"/read", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notifRepo.On("GetByID", mock.Anything, notifTestNotifID).Return(nil, fmt.Errorf("not found"))
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/notifications/"+notifTestNotifID+"/read", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- MarkAllAsRead ---

func TestNotificationHandler_MarkAllAsRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notifRepo.On("MarkAllAsRead", mock.Anything, notifTestUserID).Return(nil)
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/notifications/read-all", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		notifRepo.AssertExpectations(t)
	})
}

// --- DeleteNotification ---

func TestNotificationHandler_DeleteNotification(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notifRepo.On("GetByID", mock.Anything, notifTestNotifID).Return(nil, fmt.Errorf("not found"))
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/notifications/"+notifTestNotifID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		notifRepo := &mocks.MockNotificationRepository{}
		notif := &models.Notification{ID: notifTestNotifID, UserID: notifTestUserID}
		notifRepo.On("GetByID", mock.Anything, notifTestNotifID).Return(notif, nil)
		notifRepo.On("Delete", mock.Anything, notifTestNotifID).Return(nil)
		r := newNotificationRouter(t, notifRepo, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/notifications/"+notifTestNotifID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- GetNotificationSettings ---

func TestNotificationHandler_GetNotificationSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		settingsRepo := &mocks.MockNotificationSettingsRepository{}
		settingsRepo.On("GetByProfileID", mock.Anything, notifTestUserID).
			Return([]*models.NotificationSetting{}, nil)
		settingsRepo.On("InitializeDefaults", mock.Anything, notifTestUserID).Return(nil).Maybe()
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, settingsRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications/settings", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
		settingsRepo.AssertExpectations(t)
	})
}

// --- UpdateNotificationSetting ---

func TestNotificationHandler_UpdateNotificationSetting(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, &mocks.MockNotificationSettingsRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/notifications/settings",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid category", func(t *testing.T) {
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, &mocks.MockNotificationSettingsRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/notifications/settings",
			strings.NewReader(`{"category":"INVALID","push_pref":true}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		settingsRepo := &mocks.MockNotificationSettingsRepository{}
		settingsRepo.On("UpsertCategory", mock.Anything, notifTestUserID, models.NotificationCategoryPosts, true).Return(nil)
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, settingsRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/notifications/settings",
			strings.NewReader(`{"category":"POSTS","push_pref":true}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		settingsRepo.AssertExpectations(t)
	})
}

// --- RegisterFCMToken ---

func TestNotificationHandler_RegisterFCMToken(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, &mocks.MockNotificationSettingsRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/notifications/fcm-token",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("token too short", func(t *testing.T) {
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, &mocks.MockNotificationSettingsRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/notifications/fcm-token",
			strings.NewReader(`{"token":"short"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- UnregisterFCMToken ---

func TestNotificationHandler_UnregisterFCMToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := newNotificationRouter(t, &mocks.MockNotificationRepository{}, &mocks.MockNotificationSettingsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/notifications/fcm-token", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
