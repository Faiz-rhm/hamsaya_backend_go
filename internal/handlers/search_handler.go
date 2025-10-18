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

// SearchHandler handles search and discovery endpoints
type SearchHandler struct {
	searchService *services.SearchService
	validator     *utils.Validator
	logger        *zap.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(
	searchService *services.SearchService,
	validator *utils.Validator,
	logger *zap.Logger,
) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		validator:     validator,
		logger:        logger,
	}
}

// Search handles GET /api/v1/search
// @Summary Global search
// @Description Search across posts, users, and businesses
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query (minimum 2 characters)"
// @Param type query string false "Search type: all, posts, users, businesses" Enums(all, posts, users, businesses)
// @Param limit query int false "Limit per page (default 20, max 100)"
// @Param offset query int false "Offset for pagination"
// @Param latitude query number false "Latitude for location-based search"
// @Param longitude query number false "Longitude for location-based search"
// @Param radius_km query number false "Radius in kilometers for location-based search"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.SearchResponse}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	// Parse query parameters
	query := c.Query("query")
	if query == "" {
		utils.SendError(c, http.StatusBadRequest, "Search query is required", utils.ErrBadRequest)
		return
	}

	searchType := c.DefaultQuery("type", "all")
	if searchType == "" {
		searchType = "all"
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse optional location parameters
	var latitude, longitude, radiusKm *float64
	if latStr := c.Query("latitude"); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			latitude = &lat
		}
	}
	if lngStr := c.Query("longitude"); lngStr != "" {
		if lng, err := strconv.ParseFloat(lngStr, 64); err == nil {
			longitude = &lng
		}
	}
	if radiusStr := c.Query("radius_km"); radiusStr != "" {
		if radius, err := strconv.ParseFloat(radiusStr, 64); err == nil {
			radiusKm = &radius
		}
	}

	// Create request
	req := &models.SearchRequest{
		Query:     query,
		Type:      models.SearchType(searchType),
		Limit:     limit,
		Offset:    offset,
		Latitude:  latitude,
		Longitude: longitude,
		RadiusKm:  radiusKm,
	}

	// Validate request
	if err := h.validator.Validate(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Get authenticated user ID (optional)
	var userID *string
	if id, exists := c.Get("user_id"); exists {
		userIDStr := id.(string)
		userID = &userIDStr
	}

	// Perform search
	results, err := h.searchService.Search(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Search completed successfully", results)
}

// SearchPosts handles GET /api/v1/search/posts
// @Summary Search posts
// @Description Search for posts with full-text search and location filtering
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param limit query int false "Limit per page"
// @Param offset query int false "Offset for pagination"
// @Param latitude query number false "Latitude for location-based search"
// @Param longitude query number false "Longitude for location-based search"
// @Param radius_km query number false "Radius in kilometers"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]models.PostResponse}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /search/posts [get]
func (h *SearchHandler) SearchPosts(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		utils.SendError(c, http.StatusBadRequest, "Search query is required", utils.ErrBadRequest)
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse optional location parameters
	var latitude, longitude, radiusKm *float64
	if latStr := c.Query("latitude"); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			latitude = &lat
		}
	}
	if lngStr := c.Query("longitude"); lngStr != "" {
		if lng, err := strconv.ParseFloat(lngStr, 64); err == nil {
			longitude = &lng
		}
	}
	if radiusStr := c.Query("radius_km"); radiusStr != "" {
		if radius, err := strconv.ParseFloat(radiusStr, 64); err == nil {
			radiusKm = &radius
		}
	}

	req := &models.SearchRequest{
		Query:     query,
		Type:      models.SearchTypePosts,
		Limit:     limit,
		Offset:    offset,
		Latitude:  latitude,
		Longitude: longitude,
		RadiusKm:  radiusKm,
	}

	var userID *string
	if id, exists := c.Get("user_id"); exists {
		userIDStr := id.(string)
		userID = &userIDStr
	}

	results, err := h.searchService.Search(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Posts found", results.Posts)
}

// SearchUsers handles GET /api/v1/search/users
// @Summary Search users
// @Description Search for users by name
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param limit query int false "Limit per page"
// @Param offset query int false "Offset for pagination"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]models.UserSearchResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /search/users [get]
func (h *SearchHandler) SearchUsers(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		utils.SendError(c, http.StatusBadRequest, "Search query is required", utils.ErrBadRequest)
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	req := &models.SearchRequest{
		Query:  query,
		Type:   models.SearchTypeUsers,
		Limit:  limit,
		Offset: offset,
	}

	var userID *string
	if id, exists := c.Get("user_id"); exists {
		userIDStr := id.(string)
		userID = &userIDStr
	}

	results, err := h.searchService.Search(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Users found", results.Users)
}

// SearchBusinesses handles GET /api/v1/search/businesses
// @Summary Search businesses
// @Description Search for businesses by name, description, or location
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param limit query int false "Limit per page"
// @Param offset query int false "Offset for pagination"
// @Param latitude query number false "Latitude for location-based search"
// @Param longitude query number false "Longitude for location-based search"
// @Param radius_km query number false "Radius in kilometers"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]models.BusinessSearchResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /search/businesses [get]
func (h *SearchHandler) SearchBusinesses(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		utils.SendError(c, http.StatusBadRequest, "Search query is required", utils.ErrBadRequest)
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var latitude, longitude, radiusKm *float64
	if latStr := c.Query("latitude"); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			latitude = &lat
		}
	}
	if lngStr := c.Query("longitude"); lngStr != "" {
		if lng, err := strconv.ParseFloat(lngStr, 64); err == nil {
			longitude = &lng
		}
	}
	if radiusStr := c.Query("radius_km"); radiusStr != "" {
		if radius, err := strconv.ParseFloat(radiusStr, 64); err == nil {
			radiusKm = &radius
		}
	}

	req := &models.SearchRequest{
		Query:     query,
		Type:      models.SearchTypeBusinesses,
		Limit:     limit,
		Offset:    offset,
		Latitude:  latitude,
		Longitude: longitude,
		RadiusKm:  radiusKm,
	}

	var userID *string
	if id, exists := c.Get("user_id"); exists {
		userIDStr := id.(string)
		userID = &userIDStr
	}

	results, err := h.searchService.Search(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Businesses found", results.Businesses)
}

// Discover handles GET /api/v1/discover
// @Summary Map-based discovery
// @Description Get posts and businesses within a radius for map view
// @Tags Discovery
// @Accept json
// @Produce json
// @Param latitude query number true "Latitude"
// @Param longitude query number true "Longitude"
// @Param radius_km query number true "Radius in kilometers (max 100)"
// @Param type query string false "Post type filter: FEED, EVENT, SELL, PULL"
// @Param limit query int false "Limit results (default 100, max 500)"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.DiscoverResponse}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /discover [get]
func (h *SearchHandler) Discover(c *gin.Context) {
	// Parse required parameters
	latStr := c.Query("latitude")
	lngStr := c.Query("longitude")
	radiusStr := c.Query("radius_km")

	if latStr == "" || lngStr == "" || radiusStr == "" {
		utils.SendError(c, http.StatusBadRequest, "Latitude, longitude, and radius_km are required", utils.ErrBadRequest)
		return
	}

	latitude, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid latitude", utils.ErrBadRequest)
		return
	}

	longitude, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid longitude", utils.ErrBadRequest)
		return
	}

	radiusKm, err := strconv.ParseFloat(radiusStr, 64)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid radius_km", utils.ErrBadRequest)
		return
	}

	// Parse optional parameters
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	var postType *models.PostType
	if typeStr := c.Query("type"); typeStr != "" {
		pt := models.PostType(typeStr)
		postType = &pt
	}

	// Create request
	req := &models.DiscoverRequest{
		Latitude:  latitude,
		Longitude: longitude,
		RadiusKm:  radiusKm,
		Type:      postType,
		Limit:     limit,
	}

	// Validate request
	if err := h.validator.Validate(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Get authenticated user ID (optional)
	var userID *string
	if id, exists := c.Get("user_id"); exists {
		userIDStr := id.(string)
		userID = &userIDStr
	}

	// Perform discovery
	results, err := h.searchService.Discover(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Discovery completed successfully", results)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *SearchHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in search handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
