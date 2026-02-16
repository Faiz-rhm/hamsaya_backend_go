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

// BusinessHandler handles business-related endpoints
type BusinessHandler struct {
	businessService *services.BusinessService
	storageService  *services.StorageService
	validator       *utils.Validator
	logger          *zap.Logger
}

// NewBusinessHandler creates a new business handler
func NewBusinessHandler(
	businessService *services.BusinessService,
	storageService *services.StorageService,
	validator *utils.Validator,
	logger *zap.Logger,
) *BusinessHandler {
	return &BusinessHandler{
		businessService: businessService,
		storageService:  storageService,
		validator:       validator,
		logger:          logger,
	}
}

// CreateBusiness godoc
// @Summary Create a business profile
// @Description Create a new business profile
// @Tags businesses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.CreateBusinessRequest true "Business creation request"
// @Success 201 {object} utils.Response{data=models.BusinessResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /businesses [post]
func (h *BusinessHandler) CreateBusiness(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse request
	var req models.CreateBusinessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Create business
	business, err := h.businessService.CreateBusiness(c.Request.Context(), userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Business created successfully", business)
}

// GetBusiness godoc
// @Summary Get a business profile
// @Description Get a business profile by ID
// @Tags businesses
// @Produce json
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response{data=models.BusinessResponse}
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id} [get]
func (h *BusinessHandler) GetBusiness(c *gin.Context) {
	businessID := c.Param("business_id")

	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Get business
	business, err := h.businessService.GetBusiness(c.Request.Context(), businessID, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business retrieved successfully", business)
}

// GetMyBusinesses godoc
// @Summary Get authenticated user's businesses
// @Description Get all businesses owned by the authenticated user
// @Tags businesses
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.BusinessResponse}
// @Failure 401 {object} utils.Response
// @Router /businesses [get]
func (h *BusinessHandler) GetMyBusinesses(c *gin.Context) {
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

	// Get businesses
	businesses, err := h.businessService.GetUserBusinesses(c.Request.Context(), userID.(string), limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Businesses retrieved successfully", businesses)
}

// UpdateBusiness godoc
// @Summary Update a business profile
// @Description Update a business profile
// @Tags businesses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param request body models.UpdateBusinessRequest true "Business update request"
// @Success 200 {object} utils.Response{data=models.BusinessResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id} [put]
func (h *BusinessHandler) UpdateBusiness(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Parse request
	var req models.UpdateBusinessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Update business
	business, err := h.businessService.UpdateBusiness(c.Request.Context(), businessID, userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business updated successfully", business)
}

// DeleteBusiness godoc
// @Summary Delete a business profile
// @Description Delete a business profile (soft delete)
// @Tags businesses
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id} [delete]
func (h *BusinessHandler) DeleteBusiness(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Delete business
	if err := h.businessService.DeleteBusiness(c.Request.Context(), businessID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business deleted successfully", nil)
}

// SetBusinessHours godoc
// @Summary Set business hours
// @Description Set operating hours for a business
// @Tags businesses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param request body models.SetBusinessHoursRequest true "Business hours request"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/hours [post]
func (h *BusinessHandler) SetBusinessHours(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Parse request
	var req models.SetBusinessHoursRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Set business hours
	if err := h.businessService.SetBusinessHours(c.Request.Context(), businessID, userID.(string), &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business hours set successfully", nil)
}

// UploadAvatar godoc
// @Summary Upload business avatar
// @Description Upload an avatar image for a business (multipart file upload)
// @Tags businesses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param file formData file true "Avatar image file (JPEG/PNG/WebP, max 10MB)"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/avatar [post]
func (h *BusinessHandler) UploadAvatar(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "No file uploaded", err)
		return
	}
	defer file.Close()

	// Upload and process the image via storage service
	photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypeAvatar)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Save the photo URL to the business profile
	if err := h.businessService.UploadAvatar(c.Request.Context(), businessID, userID.(string), photo.URL); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Avatar uploaded successfully", &models.UploadImageResponse{
		Photo: photo,
	})
}

// UploadCover godoc
// @Summary Upload business cover photo
// @Description Upload a cover photo for a business (multipart file upload)
// @Tags businesses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param file formData file true "Cover image file (JPEG/PNG/WebP, max 10MB)"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/cover [post]
func (h *BusinessHandler) UploadCover(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "No file uploaded", err)
		return
	}
	defer file.Close()

	// Upload and process the image via storage service
	photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypeCover)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Save the photo URL to the business profile
	if err := h.businessService.UploadCover(c.Request.Context(), businessID, userID.(string), photo.URL); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Cover uploaded successfully", &models.UploadImageResponse{
		Photo: photo,
	})
}

// GetGallery godoc
// @Summary Get business gallery
// @Description Get all gallery images for a business (id + photo per item)
// @Tags businesses
// @Produce json
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response{data=[]models.GalleryItem}
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /businesses/{business_id}/attachments [get]
func (h *BusinessHandler) GetGallery(c *gin.Context) {
	businessID := c.Param("business_id")
	gallery, err := h.businessService.GetBusinessGallery(c.Request.Context(), businessID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if gallery == nil {
		gallery = []*models.GalleryItem{}
	}
	utils.SendSuccess(c, http.StatusOK, "Gallery retrieved successfully", gallery)
}

// AddGalleryImage godoc
// @Summary Add gallery image
// @Description Add an image to business gallery (multipart file upload)
// @Tags businesses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param file formData file true "Gallery image file (JPEG/PNG/WebP, max 10MB)"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/attachments [post]
func (h *BusinessHandler) AddGalleryImage(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "No file uploaded", err)
		return
	}
	defer file.Close()

	// Upload and process the image via storage service
	photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypePost)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Save the photo URL to the business gallery
	if err := h.businessService.AddGalleryImage(c.Request.Context(), businessID, userID.(string), photo.URL); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Gallery image added successfully", &models.UploadImageResponse{
		Photo: photo,
	})
}

// DeleteGalleryImage godoc
// @Summary Delete gallery image
// @Description Remove an image from business gallery
// @Tags businesses
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param attachment_id path string true "Attachment ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/attachments/{attachment_id} [delete]
func (h *BusinessHandler) DeleteGalleryImage(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")
	attachmentID := c.Param("attachment_id")

	// Delete gallery image
	if err := h.businessService.DeleteGalleryImage(c.Request.Context(), businessID, userID.(string), attachmentID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Gallery image deleted successfully", nil)
}

// FollowBusiness godoc
// @Summary Follow a business
// @Description Follow a business profile
// @Tags businesses
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/follow [post]
func (h *BusinessHandler) FollowBusiness(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Follow business
	if err := h.businessService.FollowBusiness(c.Request.Context(), businessID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business followed successfully", nil)
}

// UnfollowBusiness godoc
// @Summary Unfollow a business
// @Description Unfollow a business profile
// @Tags businesses
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /businesses/{business_id}/follow [delete]
func (h *BusinessHandler) UnfollowBusiness(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	businessID := c.Param("business_id")

	// Unfollow business
	if err := h.businessService.UnfollowBusiness(c.Request.Context(), businessID, userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business unfollowed successfully", nil)
}

// ListBusinesses godoc
// @Summary List businesses
// @Description List business profiles with filters
// @Tags businesses
// @Produce json
// @Param user_id query string false "Filter by user ID"
// @Param category_id query string false "Filter by category ID"
// @Param province query string false "Filter by province"
// @Param search query string false "Search by name or description"
// @Param latitude query number false "Latitude for nearby search"
// @Param longitude query number false "Longitude for nearby search"
// @Param radius_km query number false "Radius in kilometers for nearby search"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.BusinessResponse}
// @Failure 500 {object} utils.Response
// @Router /businesses/search [get]
func (h *BusinessHandler) ListBusinesses(c *gin.Context) {
	// Get viewer ID (may be nil for unauthenticated requests)
	var viewerID *string
	if id, exists := c.Get("user_id"); exists {
		idStr := id.(string)
		viewerID = &idStr
	}

	// Parse query parameters
	filter := &models.BusinessListFilter{
		Limit:  20,
		Offset: 0,
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

	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}

	if latStr := c.Query("latitude"); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			filter.Latitude = &lat
		}
	}

	if lngStr := c.Query("longitude"); lngStr != "" {
		if lng, err := strconv.ParseFloat(lngStr, 64); err == nil {
			filter.Longitude = &lng
		}
	}

	if radiusStr := c.Query("radius_km"); radiusStr != "" {
		if radius, err := strconv.ParseFloat(radiusStr, 64); err == nil {
			filter.RadiusKm = &radius
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// List businesses
	businesses, err := h.businessService.ListBusinesses(c.Request.Context(), filter, viewerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Businesses retrieved successfully", businesses)
}

// GetCategories godoc
// @Summary Get business categories
// @Description Get all active business categories, optionally filtered by search query
// @Tags businesses
// @Produce json
// @Param search query string false "Search by category name"
// @Success 200 {object} utils.Response{data=[]models.BusinessCategory}
// @Failure 500 {object} utils.Response
// @Router /businesses/categories [get]
func (h *BusinessHandler) GetCategories(c *gin.Context) {
	var search *string
	if q := c.Query("search"); q != "" {
		search = &q
	}
	categories, err := h.businessService.GetAllCategories(c.Request.Context(), search)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if categories == nil {
		categories = []*models.BusinessCategory{}
	}
	utils.SendSuccess(c, http.StatusOK, "Categories retrieved successfully", categories)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *BusinessHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in business handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
