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

// EventHandler handles event-related endpoints
type EventHandler struct {
	eventService *services.EventService
	validator    *utils.Validator
	logger       *zap.Logger
}

// NewEventHandler creates a new event handler
func NewEventHandler(
	eventService *services.EventService,
	validator *utils.Validator,
	logger *zap.Logger,
) *EventHandler {
	return &EventHandler{
		eventService: eventService,
		validator:    validator,
		logger:       logger,
	}
}

// SetEventInterest godoc
// @Summary Set event interest
// @Description Set user's interest level for an event (interested, going, not_interested)
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID (must be an EVENT type post)"
// @Param request body models.EventInterestRequest true "Event interest request"
// @Success 200 {object} utils.Response{data=models.EventInterestResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /events/{post_id}/interest [post]
func (h *EventHandler) SetEventInterest(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Parse request
	var req models.EventInterestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Set interest
	response, err := h.eventService.SetEventInterest(c.Request.Context(), postID, userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Event interest set successfully", response)
}

// RemoveEventInterest godoc
// @Summary Remove event interest
// @Description Remove user's interest from an event
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID (must be an EVENT type post)"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /events/{post_id}/interest [delete]
func (h *EventHandler) RemoveEventInterest(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Remove interest
	if err := h.eventService.RemoveEventInterest(c.Request.Context(), postID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Event interest removed successfully", nil)
}

// GetEventInterestStatus godoc
// @Summary Get event interest status
// @Description Get the interest status for an event (counts and user's current state)
// @Tags events
// @Produce json
// @Param post_id path string true "Post ID (must be an EVENT type post)"
// @Success 200 {object} utils.Response{data=models.EventInterestResponse}
// @Failure 404 {object} utils.Response
// @Router /events/{post_id}/interest [get]
func (h *EventHandler) GetEventInterestStatus(c *gin.Context) {
	postID := c.Param("post_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get interest status
	response, err := h.eventService.GetEventInterestStatus(c.Request.Context(), postID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Event interest status retrieved successfully", response)
}

// GetInterestedUsers godoc
// @Summary Get interested users
// @Description Get list of users who are interested in an event
// @Tags events
// @Produce json
// @Param post_id path string true "Post ID (must be an EVENT type post)"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.EventInterestedUser}
// @Failure 404 {object} utils.Response
// @Router /events/{post_id}/interested [get]
func (h *EventHandler) GetInterestedUsers(c *gin.Context) {
	postID := c.Param("post_id")

	// Parse pagination
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

	// Get interested users
	users, err := h.eventService.GetInterestedUsers(c.Request.Context(), postID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Interested users retrieved successfully", users)
}

// GetGoingUsers godoc
// @Summary Get going users
// @Description Get list of users who are going to an event
// @Tags events
// @Produce json
// @Param post_id path string true "Post ID (must be an EVENT type post)"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.EventInterestedUser}
// @Failure 404 {object} utils.Response
// @Router /events/{post_id}/going [get]
func (h *EventHandler) GetGoingUsers(c *gin.Context) {
	postID := c.Param("post_id")

	// Parse pagination
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

	// Get going users
	users, err := h.eventService.GetGoingUsers(c.Request.Context(), postID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Going users retrieved successfully", users)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *EventHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in event handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
