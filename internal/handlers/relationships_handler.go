package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// RelationshipsHandler handles user relationship endpoints
type RelationshipsHandler struct {
	relationshipsService *services.RelationshipsService
	logger               *zap.Logger
}

// NewRelationshipsHandler creates a new relationships handler
func NewRelationshipsHandler(
	relationshipsService *services.RelationshipsService,
	logger *zap.Logger,
) *RelationshipsHandler {
	return &RelationshipsHandler{
		relationshipsService: relationshipsService,
		logger:               logger,
	}
}

// FollowUser godoc
// @Summary Follow a user
// @Description Follow another user
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID to follow"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /users/{user_id}/follow [post]
func (h *RelationshipsHandler) FollowUser(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Follow user
	if err := h.relationshipsService.FollowUser(c.Request.Context(), userID.(string), targetUserID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User followed successfully", nil)
}

// UnfollowUser godoc
// @Summary Unfollow a user
// @Description Unfollow a user that you're currently following
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID to unfollow"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /users/{user_id}/follow [delete]
func (h *RelationshipsHandler) UnfollowUser(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Unfollow user
	if err := h.relationshipsService.UnfollowUser(c.Request.Context(), userID.(string), targetUserID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User unfollowed successfully", nil)
}

// GetFollowers godoc
// @Summary Get followers
// @Description Get a list of users following the specified user
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.FollowerResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/{user_id}/followers [get]
func (h *RelationshipsHandler) GetFollowers(c *gin.Context) {
	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Parse pagination params
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

	// Get followers
	followers, err := h.relationshipsService.GetFollowers(c.Request.Context(), targetUserID, viewerID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Followers retrieved successfully", followers)
}

// GetFollowing godoc
// @Summary Get following
// @Description Get a list of users that the specified user is following
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.FollowingResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/{user_id}/following [get]
func (h *RelationshipsHandler) GetFollowing(c *gin.Context) {
	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Parse pagination params
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

	// Get following
	following, err := h.relationshipsService.GetFollowing(c.Request.Context(), targetUserID, viewerID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Following retrieved successfully", following)
}

// BlockUser godoc
// @Summary Block a user
// @Description Block another user
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID to block"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /users/{user_id}/block [post]
func (h *RelationshipsHandler) BlockUser(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Block user
	if err := h.relationshipsService.BlockUser(c.Request.Context(), userID.(string), targetUserID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User blocked successfully", nil)
}

// UnblockUser godoc
// @Summary Unblock a user
// @Description Unblock a previously blocked user
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID to unblock"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /users/{user_id}/block [delete]
func (h *RelationshipsHandler) UnblockUser(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Unblock user
	if err := h.relationshipsService.UnblockUser(c.Request.Context(), userID.(string), targetUserID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User unblocked successfully", nil)
}

// GetRelationshipStatus godoc
// @Summary Get relationship status
// @Description Get the relationship status between authenticated user and another user
// @Tags relationships
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} utils.Response{data=models.RelationshipStatus}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/{user_id}/relationship [get]
func (h *RelationshipsHandler) GetRelationshipStatus(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get target user ID from path
	targetUserID := c.Param("user_id")

	// Get relationship status
	status, err := h.relationshipsService.GetRelationshipStatus(c.Request.Context(), userID.(string), targetUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Relationship status retrieved successfully", status)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *RelationshipsHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in relationships handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
