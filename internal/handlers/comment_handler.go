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

// CommentHandler handles comment-related endpoints
type CommentHandler struct {
	commentService *services.CommentService
	validator      *utils.Validator
	logger         *zap.Logger
}

// NewCommentHandler creates a new comment handler
func NewCommentHandler(
	commentService *services.CommentService,
	validator *utils.Validator,
	logger *zap.Logger,
) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
		validator:      validator,
		logger:         logger,
	}
}

// CreateComment godoc
// @Summary Create a comment
// @Description Create a new comment on a post or reply to a comment
// @Tags comments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Param request body models.CreateCommentRequest true "Comment creation request"
// @Success 201 {object} utils.Response{data=models.CommentResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/comments [post]
func (h *CommentHandler) CreateComment(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Parse request
	var req models.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Create comment
	comment, err := h.commentService.CreateComment(c.Request.Context(), postID, userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Comment created successfully", comment)
}

// GetPostComments godoc
// @Summary Get post comments
// @Description Get comments for a post
// @Tags comments
// @Produce json
// @Param post_id path string true "Post ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.CommentResponse}
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/comments [get]
func (h *CommentHandler) GetPostComments(c *gin.Context) {
	postID := c.Param("post_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

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

	// Get comments
	comments, err := h.commentService.GetPostComments(c.Request.Context(), postID, limit, offset, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comments retrieved successfully", comments)
}

// GetComment godoc
// @Summary Get a comment
// @Description Get a comment by ID
// @Tags comments
// @Produce json
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} utils.Response{data=models.CommentResponse}
// @Failure 404 {object} utils.Response
// @Router /comments/{comment_id} [get]
func (h *CommentHandler) GetComment(c *gin.Context) {
	commentID := c.Param("comment_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get comment
	comment, err := h.commentService.GetComment(c.Request.Context(), commentID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment retrieved successfully", comment)
}

// GetCommentReplies godoc
// @Summary Get comment replies
// @Description Get replies to a comment
// @Tags comments
// @Produce json
// @Param comment_id path string true "Comment ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.CommentResponse}
// @Failure 404 {object} utils.Response
// @Router /comments/{comment_id}/replies [get]
func (h *CommentHandler) GetCommentReplies(c *gin.Context) {
	commentID := c.Param("comment_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

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

	// Get replies
	replies, err := h.commentService.GetCommentReplies(c.Request.Context(), commentID, limit, offset, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Replies retrieved successfully", replies)
}

// UpdateComment godoc
// @Summary Update a comment
// @Description Update a comment
// @Tags comments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Param request body models.UpdateCommentRequest true "Comment update request"
// @Success 200 {object} utils.Response{data=models.CommentResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /comments/{comment_id} [put]
func (h *CommentHandler) UpdateComment(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	commentID := c.Param("comment_id")

	// Parse request
	var req models.UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Update comment
	comment, err := h.commentService.UpdateComment(c.Request.Context(), commentID, userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment updated successfully", comment)
}

// DeleteComment godoc
// @Summary Delete a comment
// @Description Delete a comment (soft delete)
// @Tags comments
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /comments/{comment_id} [delete]
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	commentID := c.Param("comment_id")

	// Delete comment
	if err := h.commentService.DeleteComment(c.Request.Context(), commentID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment deleted successfully", nil)
}

// LikeComment godoc
// @Summary Like a comment
// @Description Like a comment
// @Tags comments
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /comments/{comment_id}/like [post]
func (h *CommentHandler) LikeComment(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	commentID := c.Param("comment_id")

	// Like comment
	if err := h.commentService.LikeComment(c.Request.Context(), userID.(string), commentID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment liked successfully", nil)
}

// UnlikeComment godoc
// @Summary Unlike a comment
// @Description Remove like from a comment
// @Tags comments
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /comments/{comment_id}/like [delete]
func (h *CommentHandler) UnlikeComment(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	commentID := c.Param("comment_id")

	// Unlike comment
	if err := h.commentService.UnlikeComment(c.Request.Context(), userID.(string), commentID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment unliked successfully", nil)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *CommentHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in comment handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
