package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// NotificationHandler handles notification-related endpoints
type NotificationHandler struct {
	notificationService *services.NotificationService
	validator           *utils.Validator
	logger              *zap.Logger
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(
	notificationService *services.NotificationService,
	validator *utils.Validator,
	logger *zap.Logger,
) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		validator:           validator,
		logger:              logger,
	}
}

// GetNotifications handles GET /api/v1/notifications
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse query parameters
	unreadOnly := c.Query("unread_only") == "true"
	var businessID *string
	if b := c.Query("business_id"); b != "" {
		businessID = &b
	}

	limit := 20
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	if offset == 0 && c.Query("offset") == "" {
		if pageStr := c.Query("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
				offset = p * limit
			}
		}
	}

	// Get notifications
	notifications, err := h.notificationService.GetNotifications(c.Request.Context(), userID.(string), unreadOnly, limit, offset, businessID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Notifications retrieved successfully", notifications)
}

// GetUnreadCount handles GET /api/v1/notifications/unread-count
// Optional query: business_id to get unread count for that business only.
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	var businessID *string
	if bid := c.Query("business_id"); bid != "" {
		businessID = &bid
	}

	// Get unread count
	count, err := h.notificationService.GetUnreadCount(c.Request.Context(), userID.(string), businessID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Unread count retrieved successfully", gin.H{
		"count": count,
	})
}

// MarkAsRead handles POST /api/v1/notifications/:notification_id/read
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	notificationID := c.Param("notification_id")
	if notificationID == "" {
		utils.SendError(c, http.StatusBadRequest, "Notification ID is required", utils.ErrBadRequest)
		return
	}

	// Mark as read
	if err := h.notificationService.MarkAsRead(c.Request.Context(), userID.(string), notificationID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Notification marked as read", nil)
}

// MarkAllAsRead handles POST /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Mark all as read
	if err := h.notificationService.MarkAllAsRead(c.Request.Context(), userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "All notifications marked as read", nil)
}

// DeleteNotification handles DELETE /api/v1/notifications/:notification_id
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	notificationID := c.Param("notification_id")
	if notificationID == "" {
		utils.SendError(c, http.StatusBadRequest, "Notification ID is required", utils.ErrBadRequest)
		return
	}

	// Delete notification
	if err := h.notificationService.DeleteNotification(c.Request.Context(), userID.(string), notificationID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Notification deleted successfully", nil)
}

// GetNotificationSettings handles GET /api/v1/notifications/settings
func (h *NotificationHandler) GetNotificationSettings(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// For simplicity, using userID as profileID (they should be the same or linked)
	// In production, you'd want to get the actual profile ID
	settings, err := h.notificationService.GetNotificationSettings(c.Request.Context(), userID.(string))
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Notification settings retrieved successfully", settings)
}

// UpdateNotificationSetting handles PUT /api/v1/notifications/settings
func (h *NotificationHandler) UpdateNotificationSetting(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse request
	var req models.UpdateNotificationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Update setting (using userID as profileID)
	if err := h.notificationService.UpdateNotificationSetting(c.Request.Context(), userID.(string), &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Notification setting updated successfully", nil)
}

// RegisterFCMToken handles POST /api/v1/notifications/fcm-token
func (h *NotificationHandler) RegisterFCMToken(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse request
	var req models.FCMTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Register FCM token
	if err := h.notificationService.RegisterFCMToken(c.Request.Context(), userID.(string), req.Token); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "FCM token registered successfully", nil)
}

// UnregisterFCMToken handles DELETE /api/v1/notifications/fcm-token
func (h *NotificationHandler) UnregisterFCMToken(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Unregister FCM token
	if err := h.notificationService.UnregisterFCMToken(c.Request.Context(), userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "FCM token unregistered successfully", nil)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *NotificationHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in notification handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
