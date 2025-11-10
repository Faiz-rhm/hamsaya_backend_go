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

// AdminHandler handles admin endpoints
type AdminHandler struct {
	adminService *services.AdminService
	logger       *zap.SugaredLogger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(adminService *services.AdminService, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
		logger:       logger.Sugar(),
	}
}

// GetStatistics godoc
// @Summary Get admin dashboard statistics
// @Description Retrieve comprehensive statistics for the admin dashboard including user counts, post counts by type, business counts, and pending reports
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.AdminStatistics} "Admin statistics retrieved successfully"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/statistics [get]
func (h *AdminHandler) GetStatistics(c *gin.Context) {
	stats, err := h.adminService.GetStatistics(c.Request.Context())
	if err != nil {
		h.logger.Errorw("Failed to get admin statistics", "error", err)
		utils.SendError(c, http.StatusInternalServerError, "Failed to retrieve statistics", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Statistics retrieved successfully", stats)
}

// ListUsers godoc
// @Summary List users
// @Description Retrieve a paginated list of users with optional filtering by active status and search term
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search term (email, first name, last name)"
// @Param is_active query boolean false "Filter by active status (true/false)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Success 200 {object} utils.Response{data=[]models.AdminUserListItem} "Users retrieved successfully"
// @Failure 400 {object} utils.Response "Invalid query parameters"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	// Parse query parameters
	search := c.Query("search")

	var isActive *bool
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActiveBool, err := strconv.ParseBool(isActiveStr)
		if err != nil {
			utils.SendError(c, http.StatusBadRequest, "Invalid is_active parameter", err)
			return
		}
		isActive = &isActiveBool
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			utils.SendError(c, http.StatusBadRequest, "Invalid page parameter", err)
			return
		}
		page = p
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			utils.SendError(c, http.StatusBadRequest, "Invalid limit parameter (must be 1-100)", err)
			return
		}
		limit = l
	}

	users, totalCount, err := h.adminService.ListUsers(c.Request.Context(), search, isActive, page, limit)
	if err != nil {
		h.logger.Errorw("Failed to list users", "error", err)
		utils.SendError(c, http.StatusInternalServerError, "Failed to retrieve users", err)
		return
	}

	utils.SendPaginated(c, users, page, limit, totalCount)
}

// UpdateUserStatus godoc
// @Summary Update user status
// @Description Activate or deactivate a user account
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param request body models.UpdateUserStatusRequest true "Status update request"
// @Success 200 {object} utils.Response "User status updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "User not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/users/{id}/status [put]
func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		utils.SendError(c, http.StatusBadRequest, "User ID is required", nil)
		return
	}

	var req models.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdateUserStatus(c.Request.Context(), userID, req.IsActive)
	if err != nil {
		h.logger.Errorw("Failed to update user status",
			"user_id", userID,
			"error", err,
		)

		// Check if user not found
		if err.Error() == "user not found: "+userID {
			utils.SendError(c, http.StatusNotFound, "User not found", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update user status", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User status updated successfully", nil)
}

// ListPosts godoc
// @Summary List posts
// @Description Retrieve a paginated list of posts with optional filtering by type and search term
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param type query string false "Filter by post type (FEED, EVENT, SELL, PULL, or 'all')"
// @Param search query string false "Search term (title, description, user email, business name)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Success 200 {object} utils.Response{data=[]models.AdminPostListItem} "Posts retrieved successfully"
// @Failure 400 {object} utils.Response "Invalid query parameters"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/posts [get]
func (h *AdminHandler) ListPosts(c *gin.Context) {
	// Parse query parameters
	postType := c.DefaultQuery("type", "all")
	search := c.Query("search")

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			utils.SendError(c, http.StatusBadRequest, "Invalid page parameter", err)
			return
		}
		page = p
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			utils.SendError(c, http.StatusBadRequest, "Invalid limit parameter (must be 1-100)", err)
			return
		}
		limit = l
	}

	posts, totalCount, err := h.adminService.ListPosts(c.Request.Context(), postType, search, page, limit)
	if err != nil {
		h.logger.Errorw("Failed to list posts", "error", err)
		utils.SendError(c, http.StatusInternalServerError, "Failed to retrieve posts", err)
		return
	}

	utils.SendPaginated(c, posts, page, limit, totalCount)
}

// ListReports godoc
// @Summary List reports
// @Description Retrieve a paginated list of reports with optional filtering by type, status, and search term
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param type query string false "Filter by report type (POST, COMMENT, USER, BUSINESS, or 'all')"
// @Param status query string false "Filter by status (PENDING, REVIEWING, RESOLVED, REJECTED, or 'all')"
// @Param search query string false "Search term (reporter email/name, reported item, reason)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Success 200 {object} utils.Response{data=[]models.AdminReportListItem} "Reports retrieved successfully"
// @Failure 400 {object} utils.Response "Invalid query parameters"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/reports [get]
func (h *AdminHandler) ListReports(c *gin.Context) {
	// Parse query parameters
	reportType := c.DefaultQuery("type", "all")
	status := c.DefaultQuery("status", "all")
	search := c.Query("search")

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			utils.SendError(c, http.StatusBadRequest, "Invalid page parameter", err)
			return
		}
		page = p
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			utils.SendError(c, http.StatusBadRequest, "Invalid limit parameter (must be 1-100)", err)
			return
		}
		limit = l
	}

	reports, totalCount, err := h.adminService.ListReports(c.Request.Context(), reportType, status, search, page, limit)
	if err != nil {
		h.logger.Errorw("Failed to list reports", "error", err)
		utils.SendError(c, http.StatusInternalServerError, "Failed to retrieve reports", err)
		return
	}

	utils.SendPaginated(c, reports, page, limit, totalCount)
}

// UpdateReportStatus godoc
// @Summary Update report status
// @Description Update the status of a report (PENDING, REVIEWING, RESOLVED, REJECTED)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param type path string true "Report type (POST, COMMENT, USER, BUSINESS)"
// @Param id path string true "Report ID"
// @Param request body models.UpdateReportStatusRequest true "Status update request"
// @Success 200 {object} utils.Response "Report status updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body or parameters"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "Report not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/reports/{type}/{id}/status [put]
func (h *AdminHandler) UpdateReportStatus(c *gin.Context) {
	reportType := c.Param("type")
	reportID := c.Param("id")

	if reportType == "" || reportID == "" {
		utils.SendError(c, http.StatusBadRequest, "Report type and ID are required", nil)
		return
	}

	// Validate report type
	validTypes := map[string]bool{
		"POST":     true,
		"COMMENT":  true,
		"USER":     true,
		"BUSINESS": true,
	}
	if !validTypes[reportType] {
		utils.SendError(c, http.StatusBadRequest, "Invalid report type. Must be POST, COMMENT, USER, or BUSINESS", nil)
		return
	}

	var req models.UpdateReportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdateReportStatus(c.Request.Context(), reportType, reportID, string(req.Status))
	if err != nil {
		h.logger.Errorw("Failed to update report status",
			"report_type", reportType,
			"report_id", reportID,
			"error", err,
		)

		// Check if report not found
		if err.Error() == "report not found: "+reportID {
			utils.SendError(c, http.StatusNotFound, "Report not found", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update report status", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Report status updated successfully", nil)
}

// ListBusinesses godoc
// @Summary List businesses
// @Description Retrieve a paginated list of businesses with optional filtering by status and search term
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search term (name, license_no, email, phone_number, province, district)"
// @Param status query boolean false "Filter by status (true=active, false=inactive)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Success 200 {object} utils.Response{data=[]models.AdminBusinessListItem} "Businesses retrieved successfully"
// @Failure 400 {object} utils.Response "Invalid query parameters"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/businesses [get]
func (h *AdminHandler) ListBusinesses(c *gin.Context) {
	// Parse query parameters
	search := c.Query("search")

	var status *bool
	if statusStr := c.Query("status"); statusStr != "" {
		statusBool, err := strconv.ParseBool(statusStr)
		if err != nil {
			utils.SendError(c, http.StatusBadRequest, "Invalid status parameter", err)
			return
		}
		status = &statusBool
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			utils.SendError(c, http.StatusBadRequest, "Invalid page parameter", err)
			return
		}
		page = p
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			utils.SendError(c, http.StatusBadRequest, "Invalid limit parameter (must be 1-100)", err)
			return
		}
		limit = l
	}

	businesses, totalCount, err := h.adminService.ListBusinesses(c.Request.Context(), search, status, page, limit)
	if err != nil {
		h.logger.Errorw("Failed to list businesses", "error", err)
		utils.SendError(c, http.StatusInternalServerError, "Failed to retrieve businesses", err)
		return
	}

	utils.SendPaginated(c, businesses, page, limit, totalCount)
}

// UpdateBusinessStatus godoc
// @Summary Update business status
// @Description Activate or deactivate a business
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Business ID"
// @Param request body models.UpdateBusinessStatusRequest true "Status update request"
// @Success 200 {object} utils.Response "Business status updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "Business not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/businesses/{id}/status [put]
func (h *AdminHandler) UpdateBusinessStatus(c *gin.Context) {
	businessID := c.Param("id")
	if businessID == "" {
		utils.SendError(c, http.StatusBadRequest, "Business ID is required", nil)
		return
	}

	var req models.UpdateBusinessStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdateBusinessStatus(c.Request.Context(), businessID, req.Status)
	if err != nil {
		h.logger.Errorw("Failed to update business status",
			"business_id", businessID,
			"error", err,
		)

		// Check if business not found
		if err.Error() == "business not found: "+businessID {
			utils.SendError(c, http.StatusNotFound, "Business not found", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update business status", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business status updated successfully", nil)
}

// UpdatePostStatus godoc
// @Summary Update post status
// @Description Activate or deactivate a post
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Param request body models.UpdatePostStatusRequest true "Status update request"
// @Success 200 {object} utils.Response "Post status updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "Post not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/posts/{id}/status [put]
func (h *AdminHandler) UpdatePostStatus(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		utils.SendError(c, http.StatusBadRequest, "Post ID is required", nil)
		return
	}

	var req models.UpdatePostStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdatePostStatus(c.Request.Context(), postID, req.Status)
	if err != nil {
		h.logger.Errorw("Failed to update post status",
			"post_id", postID,
			"error", err,
		)

		// Check if post not found
		if err.Error() == "post not found: "+postID {
			utils.SendError(c, http.StatusNotFound, "Post not found", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update post status", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post status updated successfully", nil)
}

// UpdateUser godoc
// @Summary Update user information
// @Description Update user information including email, role, names, and verification status (admin operation)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param request body models.AdminUpdateUserRequest true "User update request"
// @Success 200 {object} utils.Response "User updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "User not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/users/{id} [put]
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		utils.SendError(c, http.StatusBadRequest, "User ID is required", nil)
		return
	}

	var req models.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.Errorw("Failed to update user",
			"user_id", userID,
			"error", err,
		)

		// Check if user not found
		if err.Error() == "user not found: "+userID {
			utils.SendError(c, http.StatusNotFound, "User not found", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update user", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User updated successfully", nil)
}

// UpdatePost godoc
// @Summary Update post information
// @Description Update post information including title, description, visibility, and type (admin operation)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Param request body models.AdminUpdatePostRequest true "Post update request"
// @Success 200 {object} utils.Response "Post updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "Post not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/posts/{id} [put]
func (h *AdminHandler) UpdatePost(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		utils.SendError(c, http.StatusBadRequest, "Post ID is required", nil)
		return
	}

	var req models.AdminUpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdatePost(c.Request.Context(), postID, &req)
	if err != nil {
		h.logger.Errorw("Failed to update post",
			"post_id", postID,
			"error", err,
		)

		// Check if post not found
		if err.Error() == "post not found: "+postID {
			utils.SendError(c, http.StatusNotFound, "Post not found", err)
			return
		}

		// Check for no fields to update
		if err.Error() == "no fields to update" {
			utils.SendError(c, http.StatusBadRequest, "No fields to update", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update post", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post updated successfully", nil)
}

// UpdateBusiness godoc
// @Summary Update business information
// @Description Update business information including name, license, contact info, and location (admin operation)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Business ID"
// @Param request body models.AdminUpdateBusinessRequest true "Business update request"
// @Success 200 {object} utils.Response "Business updated successfully"
// @Failure 400 {object} utils.Response "Invalid request body"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 404 {object} utils.Response "Business not found"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/businesses/{id} [put]
func (h *AdminHandler) UpdateBusiness(c *gin.Context) {
	businessID := c.Param("id")
	if businessID == "" {
		utils.SendError(c, http.StatusBadRequest, "Business ID is required", nil)
		return
	}

	var req models.AdminUpdateBusinessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	err := h.adminService.UpdateBusiness(c.Request.Context(), businessID, &req)
	if err != nil {
		h.logger.Errorw("Failed to update business",
			"business_id", businessID,
			"error", err,
		)

		// Check if business not found
		if err.Error() == "business not found: "+businessID {
			utils.SendError(c, http.StatusNotFound, "Business not found", err)
			return
		}

		// Check for no fields to update
		if err.Error() == "no fields to update" {
			utils.SendError(c, http.StatusBadRequest, "No fields to update", err)
			return
		}

		utils.SendError(c, http.StatusInternalServerError, "Failed to update business", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business updated successfully", nil)
}

// GetSellPostStatistics godoc
// @Summary Get sell post statistics
// @Description Retrieve comprehensive statistics for SELL type posts including total, sold, active, expired, revenue, and average price
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.SellStatistics} "Sell statistics retrieved successfully"
// @Failure 401 {object} utils.Response "Unauthorized - admin access required"
// @Failure 500 {object} utils.Response "Internal server error"
// @Router /admin/posts/sell/statistics [get]
func (h *AdminHandler) GetSellPostStatistics(c *gin.Context) {
	stats, err := h.adminService.GetSellPostStatistics(c.Request.Context())
	if err != nil {
		h.logger.Errorw("Failed to get sell post statistics", "error", err)
		utils.SendError(c, http.StatusInternalServerError, "Failed to retrieve sell statistics", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Sell statistics retrieved successfully", stats)
}
