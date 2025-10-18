package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// PollHandler handles poll-related endpoints
type PollHandler struct {
	pollService *services.PollService
	validator   *utils.Validator
	logger      *zap.Logger
}

// NewPollHandler creates a new poll handler
func NewPollHandler(
	pollService *services.PollService,
	validator *utils.Validator,
	logger *zap.Logger,
) *PollHandler {
	return &PollHandler{
		pollService: pollService,
		validator:   validator,
		logger:      logger,
	}
}

// CreatePoll godoc
// @Summary Create a poll
// @Description Create a poll for a PULL type post
// @Tags polls
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Param request body models.CreatePollRequest true "Poll creation request"
// @Success 201 {object} utils.Response{data=models.PollResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/polls [post]
func (h *PollHandler) CreatePoll(c *gin.Context) {
	// Get authenticated user ID
	_, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Parse request
	var req models.CreatePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Create poll
	poll, err := h.pollService.CreatePoll(c.Request.Context(), postID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Poll created successfully", poll)
}

// GetPoll godoc
// @Summary Get a poll
// @Description Get a poll by ID
// @Tags polls
// @Produce json
// @Param poll_id path string true "Poll ID"
// @Success 200 {object} utils.Response{data=models.PollResponse}
// @Failure 404 {object} utils.Response
// @Router /polls/{poll_id} [get]
func (h *PollHandler) GetPoll(c *gin.Context) {
	pollID := c.Param("poll_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get poll
	poll, err := h.pollService.GetPoll(c.Request.Context(), pollID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Poll retrieved successfully", poll)
}

// GetPostPoll godoc
// @Summary Get poll for a post
// @Description Get poll by post ID
// @Tags polls
// @Produce json
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response{data=models.PollResponse}
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/polls [get]
func (h *PollHandler) GetPostPoll(c *gin.Context) {
	postID := c.Param("post_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get poll
	poll, err := h.pollService.GetPollByPostID(c.Request.Context(), postID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Poll retrieved successfully", poll)
}

// VotePoll godoc
// @Summary Vote on a poll
// @Description Vote on a poll option
// @Tags polls
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param poll_id path string true "Poll ID"
// @Param request body models.VotePollRequest true "Vote request"
// @Success 200 {object} utils.Response{data=models.PollResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /polls/{poll_id}/vote [post]
func (h *PollHandler) VotePoll(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	pollID := c.Param("poll_id")

	// Parse request
	var req models.VotePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Vote on poll
	poll, err := h.pollService.VotePoll(c.Request.Context(), pollID, userID.(string), req.PollOptionID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Vote recorded successfully", poll)
}

// DeleteVote godoc
// @Summary Delete vote
// @Description Remove user's vote from a poll
// @Tags polls
// @Produce json
// @Security BearerAuth
// @Param poll_id path string true "Poll ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /polls/{poll_id}/vote [delete]
func (h *PollHandler) DeleteVote(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	pollID := c.Param("poll_id")

	// Delete vote
	if err := h.pollService.DeleteVote(c.Request.Context(), pollID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Vote deleted successfully", nil)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *PollHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in poll handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
