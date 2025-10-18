package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// ProfileHandler handles profile-related endpoints
type ProfileHandler struct {
	profileService *services.ProfileService
	storageService *services.StorageService
	validator      *utils.Validator
	logger         *zap.Logger
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(
	profileService *services.ProfileService,
	storageService *services.StorageService,
	validator *utils.Validator,
	logger *zap.Logger,
) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
		storageService: storageService,
		validator:      validator,
		logger:         logger,
	}
}

// GetMyProfile godoc
// @Summary Get authenticated user's profile
// @Description Get the profile of the currently authenticated user
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.FullProfileResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me [get]
func (h *ProfileHandler) GetMyProfile(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get profile
	profile, err := h.profileService.GetProfile(c.Request.Context(), userID.(string), nil)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Profile retrieved successfully", profile)
}

// GetUserProfile godoc
// @Summary Get user profile by ID
// @Description Get a user's profile by their user ID
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} utils.Response{data=models.FullProfileResponse}
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/{user_id} [get]
func (h *ProfileHandler) GetUserProfile(c *gin.Context) {
	targetUserID := c.Param("user_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get profile
	profile, err := h.profileService.GetProfile(c.Request.Context(), targetUserID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Profile retrieved successfully", profile)
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update the authenticated user's profile information
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.UpdateProfileRequest true "Profile update request"
// @Success 200 {object} utils.Response{data=models.FullProfileResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me [put]
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse request
	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Validate location: both latitude and longitude must be provided together
	if (req.Latitude != nil && req.Longitude == nil) || (req.Latitude == nil && req.Longitude != nil) {
		utils.SendError(c, http.StatusBadRequest, "Both latitude and longitude must be provided together", utils.ErrValidation)
		return
	}

	// Update profile
	profile, err := h.profileService.UpdateProfile(c.Request.Context(), userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Profile updated successfully", profile)
}

// UploadAvatar godoc
// @Summary Upload avatar
// @Description Upload a new avatar image for the authenticated user
// @Tags profile
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Avatar image file (JPEG/PNG, max 10MB)"
// @Success 200 {object} utils.Response{data=models.UploadImageResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me/avatar [post]
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "No file uploaded", err)
		return
	}
	defer file.Close()

	// Upload image
	photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypeAvatar)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Update profile avatar
	if err := h.profileService.UpdateAvatar(c.Request.Context(), userID.(string), photo); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Avatar uploaded successfully", &models.UploadImageResponse{
		Photo: photo,
	})
}

// DeleteAvatar godoc
// @Summary Delete avatar
// @Description Delete the authenticated user's avatar
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me/avatar [delete]
func (h *ProfileHandler) DeleteAvatar(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Delete avatar
	if err := h.profileService.DeleteAvatar(c.Request.Context(), userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Avatar deleted successfully", nil)
}

// UploadCover godoc
// @Summary Upload cover photo
// @Description Upload a new cover photo for the authenticated user
// @Tags profile
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Cover image file (JPEG/PNG, max 10MB)"
// @Success 200 {object} utils.Response{data=models.UploadImageResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me/cover [post]
func (h *ProfileHandler) UploadCover(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "No file uploaded", err)
		return
	}
	defer file.Close()

	// Upload image
	photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypeCover)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Update profile cover
	if err := h.profileService.UpdateCover(c.Request.Context(), userID.(string), photo); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Cover photo uploaded successfully", &models.UploadImageResponse{
		Photo: photo,
	})
}

// DeleteCover godoc
// @Summary Delete cover photo
// @Description Delete the authenticated user's cover photo
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/me/cover [delete]
func (h *ProfileHandler) DeleteCover(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Delete cover
	if err := h.profileService.DeleteCover(c.Request.Context(), userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Cover photo deleted successfully", nil)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *ProfileHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in profile handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
