package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// PostHandler handles post-related endpoints
type PostHandler struct {
	postService    *services.PostService
	storageService *services.StorageService
	validator      *utils.Validator
	logger         *zap.Logger
}

// NewPostHandler creates a new post handler
func NewPostHandler(
	postService *services.PostService,
	storageService *services.StorageService,
	validator *utils.Validator,
	logger *zap.Logger,
) *PostHandler {
	return &PostHandler{
		postService:    postService,
		storageService: storageService,
		validator:      validator,
		logger:         logger,
	}
}

// CreatePost godoc
// @Summary Create a post
// @Description Create a new post (FEED, EVENT, SELL, or PULL)
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.CreatePostRequest true "Post creation request"
// @Success 201 {object} utils.Response{data=models.PostResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /posts [post]
func (h *PostHandler) CreatePost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse request
	var req models.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Create post
	post, err := h.postService.CreatePost(c.Request.Context(), userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Post created successfully", post)
}

// GetPost godoc
// @Summary Get a post
// @Description Get a post by ID
// @Tags posts
// @Produce json
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response{data=models.PostResponse}
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /posts/{post_id} [get]
func (h *PostHandler) GetPost(c *gin.Context) {
	postID := c.Param("post_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get post
	post, err := h.postService.GetPost(c.Request.Context(), postID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post retrieved successfully", post)
}

// UpdatePost godoc
// @Summary Update a post
// @Description Update a post
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Param request body models.UpdatePostRequest true "Post update request"
// @Success 200 {object} utils.Response{data=models.PostResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id} [put]
func (h *PostHandler) UpdatePost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Read body once so we can restore for binding and fallback parse poll_options
	bodyBytes, errRead := io.ReadAll(c.Request.Body)
	if errRead != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	// Fallback: if binding left poll_options empty but body has them, parse manually
	if len(req.PollOptions) == 0 {
		var aux struct {
			PollOptions []string `json:"poll_options"`
		}
		if _ = json.Unmarshal(bodyBytes, &aux); len(aux.PollOptions) >= 2 {
			req.PollOptions = aux.PollOptions
			h.logger.Info("UpdatePost: poll_options restored from body",
				zap.String("post_id", postID),
				zap.Int("count", len(req.PollOptions)),
				zap.Strings("options", req.PollOptions),
			)
		}
	}
	h.logger.Info("UpdatePost request",
		zap.String("post_id", postID),
		zap.Int("poll_options_count", len(req.PollOptions)),
		zap.Strings("poll_options", req.PollOptions),
	)

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Update post
	post, err := h.postService.UpdatePost(c.Request.Context(), postID, userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post updated successfully", post)
}

// DeletePost godoc
// @Summary Delete a post
// @Description Delete a post (soft delete)
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id} [delete]
func (h *PostHandler) DeletePost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Delete post
	if err := h.postService.DeletePost(c.Request.Context(), postID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post deleted successfully", nil)
}

// LikePost godoc
// @Summary Like a post
// @Description Like a post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/like [post]
func (h *PostHandler) LikePost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Like post
	if err := h.postService.LikePost(c.Request.Context(), userID.(string), postID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post liked successfully", nil)
}

// UnlikePost godoc
// @Summary Unlike a post
// @Description Remove like from a post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /posts/{post_id}/like [delete]
func (h *PostHandler) UnlikePost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Unlike post
	if err := h.postService.UnlikePost(c.Request.Context(), userID.(string), postID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post unliked successfully", nil)
}

// BookmarkPost godoc
// @Summary Bookmark a post
// @Description Bookmark a post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/bookmark [post]
func (h *PostHandler) BookmarkPost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Bookmark post
	if err := h.postService.BookmarkPost(c.Request.Context(), userID.(string), postID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post bookmarked successfully", nil)
}

// UnbookmarkPost godoc
// @Summary Remove bookmark from a post
// @Description Remove bookmark from a post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /posts/{post_id}/bookmark [delete]
func (h *PostHandler) UnbookmarkPost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Unbookmark post
	if err := h.postService.UnbookmarkPost(c.Request.Context(), userID.(string), postID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post unbookmarked successfully", nil)
}

// SharePost godoc
// @Summary Share a post
// @Description Share a post with optional text
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Param share_text body string false "Share text"
// @Success 200 {object} utils.Response{data=models.PostResponse}
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /posts/{post_id}/share [post]
func (h *PostHandler) SharePost(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	postID := c.Param("post_id")

	// Parse optional share text
	var req struct {
		ShareText *string `json:"share_text,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)

	// Share post
	post, err := h.postService.SharePost(c.Request.Context(), userID.(string), postID, req.ShareText)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post shared successfully", post)
}

// GetFeed godoc
// @Summary Get feed
// @Description Get posts feed with filters
// @Tags posts
// @Produce json
// @Param type query string false "Post type (FEED, EVENT, SELL, PULL)"
// @Param user_id query string false "Filter by user ID"
// @Param category_id query string false "Filter by category ID (for SELL posts)"
// @Param province query string false "Filter by province"
// @Param sort_by query string false "Sort by (recent, trending, nearby)" default(recent)
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.PostResponse}
// @Failure 500 {object} utils.Response
// @Router /posts [get]
func (h *PostHandler) GetFeed(c *gin.Context) {
	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Parse query parameters
	filter := &models.FeedFilter{
		SortBy: "recent",
		Limit:  20,
		Offset: 0,
	}

	if postType := c.Query("type"); postType != "" {
		pt := models.PostType(postType)
		filter.Type = &pt
	}

	if userID := c.Query("user_id"); userID != "" {
		filter.UserID = &userID
	}

	if categoryID := c.Query("category_id"); categoryID != "" {
		filter.CategoryID = &categoryID
	}

	if province := c.Query("province"); province != "" {
		filter.Province = &province
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		filter.SortBy = sortBy
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	// Support both 'page' and 'offset' parameters
	// 'page' takes precedence if both are provided
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			filter.Offset = (p - 1) * filter.Limit
		}
	} else if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
			// Calculate page from offset
			page = (offset / filter.Limit) + 1
		}
	}

	// Get feed
	posts, totalCount, err := h.postService.GetFeed(c.Request.Context(), filter, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Build filters map for response
	filters := make(map[string]interface{})
	if filter.Type != nil {
		filters["type"] = string(*filter.Type)
	}
	if filter.UserID != nil {
		filters["user_id"] = *filter.UserID
	}
	if filter.CategoryID != nil {
		filters["category_id"] = *filter.CategoryID
	}
	if filter.Province != nil {
		filters["province"] = *filter.Province
	}

	// Build sorts map for response
	sorts := map[string]interface{}{
		"sort_by": filter.SortBy,
	}

	utils.SendPaginatedWithFilters(c, posts, page, filter.Limit, totalCount, filters, sorts)
}

// GetMyPosts godoc
// @Summary Get authenticated user's posts
// @Description Get all posts created by the authenticated user
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.PostResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me/posts [get]
func (h *PostHandler) GetMyPosts(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Convert to string
	userIDStr := userID.(string)

	// Parse pagination
	limit := 20
	offset := 0
	page := 1

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Support both 'page' and 'offset' parameters
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (p - 1) * limit
		}
	} else if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
			page = (offset / limit) + 1
		}
	}

	// Get user's posts using feed filter
	filter := &models.FeedFilter{
		UserID: &userIDStr,
		SortBy: "recent",
		Limit:  limit,
		Offset: offset,
	}

	posts, totalCount, err := h.postService.GetFeed(c.Request.Context(), filter, &userIDStr)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendPaginated(c, posts, page, limit, totalCount)
}

// GetMyBookmarks godoc
// @Summary Get bookmarked posts
// @Description Get all bookmarked posts for the authenticated user
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.PostResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me/bookmarks [get]
func (h *PostHandler) GetMyBookmarks(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
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

	// Get bookmarks
	posts, err := h.postService.GetUserBookmarks(c.Request.Context(), userID.(string), limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Bookmarks retrieved successfully", posts)
}

// UploadPostImage godoc
// @Summary Upload a post image
// @Description Upload an image for a post before creating the post
// @Tags posts
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Image file to upload"
// @Success 200 {object} utils.Response{data=models.UploadImageResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /posts/upload-image [post]
func (h *PostHandler) UploadPostImage(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get file from request
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "No file uploaded", err)
		return
	}
	defer file.Close()

	// Upload image to storage
	photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypePost)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.logger.Info("Post image uploaded successfully",
		zap.String("user_id", userID.(string)),
		zap.String("url", photo.URL),
	)

	utils.SendSuccess(c, http.StatusOK, "Image uploaded successfully", &models.UploadImageResponse{
		Photo: photo,
	})
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *PostHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in post handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
