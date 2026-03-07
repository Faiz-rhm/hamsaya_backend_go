package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// AdminHandler handles admin-related HTTP requests
type AdminHandler struct {
	adminService *services.AdminService
	validator    *utils.Validator
	logger       *zap.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(
	adminService *services.AdminService,
	validator *utils.Validator,
	logger *zap.Logger,
) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
		validator:    validator,
		logger:       logger,
	}
}

// GetDashboardStats godoc
// @Summary Get dashboard statistics
// @Description Get aggregate statistics for the admin dashboard
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.DashboardStats}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/stats [get]
func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	stats, err := h.adminService.GetDashboardStats(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Dashboard stats retrieved successfully", stats)
}

// GetUserAnalytics godoc
// @Summary Get user analytics
// @Description Get user growth and activity analytics
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param period query string false "Period (week, month, year)" default(month)
// @Success 200 {object} utils.Response{data=models.UserAnalytics}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/analytics/users [get]
func (h *AdminHandler) GetUserAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "month")
	
	analytics, err := h.adminService.GetUserAnalytics(c.Request.Context(), period)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User analytics retrieved successfully", analytics)
}

// GetPostAnalytics godoc
// @Summary Get post analytics
// @Description Get post activity analytics
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param period query string false "Period (week, month, year)" default(month)
// @Success 200 {object} utils.Response{data=models.PostAnalytics}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/analytics/posts [get]
func (h *AdminHandler) GetPostAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "month")
	
	analytics, err := h.adminService.GetPostAnalytics(c.Request.Context(), period)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Post analytics retrieved successfully", analytics)
}

// GetEngagementAnalytics godoc
// @Summary Get engagement analytics
// @Description Get engagement metrics (likes, comments, shares)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param period query string false "Period (week, month, year)" default(month)
// @Success 200 {object} utils.Response{data=models.EngagementAnalytics}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/analytics/engagement [get]
func (h *AdminHandler) GetEngagementAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "month")
	
	analytics, err := h.adminService.GetEngagementAnalytics(c.Request.Context(), period)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Engagement analytics retrieved successfully", analytics)
}

// ListUsers godoc
// @Summary List users
// @Description List users with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search by email or name"
// @Param role query string false "Filter by role (user, admin, moderator)"
// @Param status query string false "Filter by status (active, suspended)"
// @Param province query string false "Filter by province"
// @Param sort_by query string false "Sort by (created_at, email, name)"
// @Param sort_dir query string false "Sort direction (asc, desc)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	var filter models.AdminUserFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListUsers(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Users retrieved successfully", result)
}

// GetUser godoc
// @Summary Get user details
// @Description Get a user's full details including posts and businesses
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} utils.Response{data=models.AdminUserDetailResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/users/{user_id} [get]
func (h *AdminHandler) GetUser(c *gin.Context) {
	userID := c.Param("user_id")
	
	user, err := h.adminService.GetUserDetail(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User retrieved successfully", user)
}

// SuspendUser godoc
// @Summary Suspend a user
// @Description Suspend a user for a specified number of days
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param request body models.SuspendUserRequest true "Suspension details"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/users/{user_id}/suspend [post]
func (h *AdminHandler) SuspendUser(c *gin.Context) {
	userID := c.Param("user_id")
	adminID, _ := middleware.GetUserID(c)
	
	var req models.SuspendUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	if err := h.validator.Validate(&req); err != nil {
		utils.SendBadRequest(c, err.Error(), err)
		return
	}
	
	err := h.adminService.SuspendUser(c.Request.Context(), userID, req.Days, req.Reason, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User suspended successfully", nil)
}

// UnsuspendUser godoc
// @Summary Unsuspend a user
// @Description Remove suspension from a user
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/users/{user_id}/unsuspend [post]
func (h *AdminHandler) UnsuspendUser(c *gin.Context) {
	userID := c.Param("user_id")
	adminID, _ := middleware.GetUserID(c)
	
	err := h.adminService.UnsuspendUser(c.Request.Context(), userID, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User unsuspended successfully", nil)
}

// UpdateUserRole godoc
// @Summary Update user role
// @Description Update a user's role
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param request body models.UpdateUserRoleRequest true "Role update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/users/{user_id}/role [put]
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	userID := c.Param("user_id")
	adminID, _ := middleware.GetUserID(c)
	
	var req models.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	err := h.adminService.UpdateUserRole(c.Request.Context(), userID, req.Role, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User role updated successfully", nil)
}

// DeleteUser godoc
// @Summary Delete a user
// @Description Soft delete a user
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/users/{user_id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("user_id")
	adminID, _ := middleware.GetUserID(c)
	
	err := h.adminService.DeleteUser(c.Request.Context(), userID, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User deleted successfully", nil)
}

// ListAllPosts godoc
// @Summary List all posts
// @Description List posts with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search by title or description"
// @Param type query string false "Filter by type (FEED, EVENT, SELL, PULL)"
// @Param status query string false "Filter by status"
// @Param user_id query string false "Filter by user ID"
// @Param reported query bool false "Filter reported posts"
// @Param sort_by query string false "Sort by"
// @Param sort_dir query string false "Sort direction (asc, desc)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/posts [get]
func (h *AdminHandler) ListAllPosts(c *gin.Context) {
	var filter models.AdminPostFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListPosts(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Posts retrieved successfully", result)
}

// GetPostDetail godoc
// @Summary Get post details
// @Description Get full post details including attachments and comments
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response{data=models.AdminPostDetailResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/posts/{post_id} [get]
func (h *AdminHandler) GetPostDetail(c *gin.Context) {
	postID := c.Param("post_id")

	post, err := h.adminService.GetPostDetail(c.Request.Context(), postID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Post detail retrieved successfully", post)
}

// UpdatePostStatus godoc
// @Summary Update post status
// @Description Update a post's status
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Param request body models.UpdatePostStatusRequest true "Status update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/posts/{post_id}/status [put]
func (h *AdminHandler) UpdatePostStatus(c *gin.Context) {
	postID := c.Param("post_id")
	adminID, _ := middleware.GetUserID(c)
	
	var req models.UpdatePostStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	err := h.adminService.UpdatePostStatus(c.Request.Context(), postID, req.Status, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Post status updated successfully", nil)
}

// DeletePost godoc
// @Summary Delete a post
// @Description Soft delete a post
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/posts/{post_id} [delete]
func (h *AdminHandler) DeletePost(c *gin.Context) {
	postID := c.Param("post_id")
	adminID, _ := middleware.GetUserID(c)
	
	err := h.adminService.DeletePost(c.Request.Context(), postID, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Post deleted successfully", nil)
}

// ListAllComments godoc
// @Summary List all comments
// @Description List comments with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search by content"
// @Param post_id query string false "Filter by post ID"
// @Param user_id query string false "Filter by user ID"
// @Param reported query bool false "Filter reported comments"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/comments [get]
func (h *AdminHandler) ListAllComments(c *gin.Context) {
	var filter models.AdminCommentFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListComments(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Comments retrieved successfully", result)
}

// DeleteComment godoc
// @Summary Delete a comment
// @Description Soft delete a comment
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/comments/{comment_id} [delete]
func (h *AdminHandler) DeleteComment(c *gin.Context) {
	commentID := c.Param("comment_id")
	adminID, _ := middleware.GetUserID(c)
	
	err := h.adminService.DeleteComment(c.Request.Context(), commentID, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Comment deleted successfully", nil)
}

// RestoreComment godoc
// @Summary Restore (unhide) a comment
// @Description Clears soft-delete on a comment so it is visible again
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/comments/{comment_id}/restore [put]
func (h *AdminHandler) RestoreComment(c *gin.Context) {
	commentID := c.Param("comment_id")
	adminID, _ := middleware.GetUserID(c)

	err := h.adminService.RestoreComment(c.Request.Context(), commentID, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Comment restored successfully", nil)
}

// ListPostReports godoc
// @Summary List post reports
// @Description List post reports with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (PENDING, REVIEWING, RESOLVED, REJECTED)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/posts [get]
func (h *AdminHandler) ListPostReports(c *gin.Context) {
	var filter models.AdminReportFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListPostReports(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Post reports retrieved successfully", result)
}

// GetPostReport godoc
// @Summary Get post report by ID
// @Description Get a single post report by ID
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param report_id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.AdminPostReportResponse}
// @Failure 404 {object} utils.Response
// @Router /admin/reports/posts/{report_id} [get]
func (h *AdminHandler) GetPostReport(c *gin.Context) {
	reportID := c.Param("report_id")
	if reportID == "" {
		utils.SendBadRequest(c, "Report ID is required", nil)
		return
	}
	if _, err := uuid.Parse(reportID); err != nil {
		utils.SendBadRequest(c, "Invalid report ID format", err)
		return
	}
	report, err := h.adminService.GetPostReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Post report retrieved successfully", report)
}

// ListCommentReports godoc
// @Summary List comment reports
// @Description List comment reports with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (PENDING, REVIEWING, RESOLVED, REJECTED)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/comments [get]
func (h *AdminHandler) ListCommentReports(c *gin.Context) {
	var filter models.AdminReportFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListCommentReports(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Comment reports retrieved successfully", result)
}

// GetCommentReport godoc
// @Summary Get comment report by ID
// @Description Get a single comment report by ID
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param report_id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.AdminCommentReportResponse}
// @Failure 404 {object} utils.Response
// @Router /admin/reports/comments/{report_id} [get]
func (h *AdminHandler) GetCommentReport(c *gin.Context) {
	reportID := c.Param("report_id")
	if reportID == "" {
		utils.SendBadRequest(c, "Report ID is required", nil)
		return
	}
	report, err := h.adminService.GetCommentReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Comment report retrieved successfully", report)
}

// ListUserReports godoc
// @Summary List user reports
// @Description List user reports with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (PENDING, RESOLVED)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/users [get]
func (h *AdminHandler) ListUserReports(c *gin.Context) {
	var filter models.AdminReportFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListUserReports(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User reports retrieved successfully", result)
}

// GetUserReport godoc
// @Summary Get user report by ID
// @Description Get a single user report by ID
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param report_id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.AdminUserReportResponse}
// @Failure 404 {object} utils.Response
// @Router /admin/reports/users/{report_id} [get]
func (h *AdminHandler) GetUserReport(c *gin.Context) {
	reportID := c.Param("report_id")
	if reportID == "" {
		utils.SendBadRequest(c, "Report ID is required", nil)
		return
	}
	report, err := h.adminService.GetUserReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User report retrieved successfully", report)
}

// ListBusinessReports godoc
// @Summary List business reports
// @Description List business reports with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (PENDING, REVIEWING, RESOLVED, REJECTED)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/businesses [get]
func (h *AdminHandler) ListBusinessReports(c *gin.Context) {
	var filter models.AdminReportFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListBusinessReports(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Business reports retrieved successfully", result)
}

// GetBusinessReport godoc
// @Summary Get business report by ID
// @Description Get a single business report by ID
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param report_id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.AdminBusinessReportResponse}
// @Failure 404 {object} utils.Response
// @Router /admin/reports/businesses/{report_id} [get]
func (h *AdminHandler) GetBusinessReport(c *gin.Context) {
	reportID := c.Param("report_id")
	if reportID == "" {
		utils.SendBadRequest(c, "Report ID is required", nil)
		return
	}
	report, err := h.adminService.GetBusinessReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Business report retrieved successfully", report)
}

// UpdateReportStatus godoc
// @Summary Update report status
// @Description Update a report's status
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param report_type path string true "Report type (posts, comments, users, businesses)"
// @Param report_id path string true "Report ID"
// @Param request body models.UpdateReportStatusRequest true "Status update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/{report_type}/{report_id}/status [put]
func (h *AdminHandler) UpdateReportStatus(c *gin.Context) {
	reportType := c.Param("report_type")
	reportID := c.Param("report_id")
	adminID, _ := middleware.GetUserID(c)
	
	var req models.AdminReportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	err := h.adminService.UpdateReportStatus(c.Request.Context(), reportType, reportID, req.Status, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Report status updated successfully", nil)
}

// ListFeedback godoc
// @Summary List user feedback
// @Description List all user feedback with pagination and optional type filter
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param type query string false "Filter by type (GENERAL, BUG, FEATURE, IMPROVEMENT)"
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/feedback [get]
func (h *AdminHandler) ListFeedback(c *gin.Context) {
	var filter models.AdminFeedbackFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	result, err := h.adminService.ListFeedback(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Feedback retrieved successfully", result)
}

// ListAllBusinesses godoc
// @Summary List all businesses
// @Description List businesses with filtering and pagination
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search by name"
// @Param status query string false "Filter by status"
// @Param province query string false "Filter by province"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.PaginatedResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/businesses [get]
func (h *AdminHandler) ListAllBusinesses(c *gin.Context) {
	var filter models.AdminBusinessFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		utils.SendBadRequest(c, "Invalid query parameters", err)
		return
	}
	
	result, err := h.adminService.ListBusinesses(c.Request.Context(), &filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Businesses retrieved successfully", result)
}

// GetBusinessDetail godoc
// @Summary Get business details
// @Description Get full business details including posts, hours, and gallery
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response{data=models.AdminBusinessDetailResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/businesses/{business_id} [get]
func (h *AdminHandler) GetBusinessDetail(c *gin.Context) {
	businessID := c.Param("business_id")

	business, err := h.adminService.GetBusinessDetail(c.Request.Context(), businessID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Business detail retrieved successfully", business)
}

// UpdateBusinessStatus godoc
// @Summary Update business status
// @Description Update a business's status
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param request body models.UpdateBusinessStatusRequest true "Status update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/businesses/{business_id}/status [put]
func (h *AdminHandler) UpdateBusinessStatus(c *gin.Context) {
	businessID := c.Param("business_id")
	adminID, _ := middleware.GetUserID(c)
	
	var req models.UpdateBusinessStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	err := h.adminService.UpdateBusinessStatus(c.Request.Context(), businessID, req.Status, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Business status updated successfully", nil)
}

// DeleteBusiness godoc
// @Summary Delete a business
// @Description Soft delete a business
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/businesses/{business_id} [delete]
func (h *AdminHandler) DeleteBusiness(c *gin.Context) {
	businessID := c.Param("business_id")
	adminID, _ := middleware.GetUserID(c)
	
	err := h.adminService.DeleteBusiness(c.Request.Context(), businessID, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Business deleted successfully", nil)
}

// BroadcastNotification godoc
// @Summary Broadcast notification
// @Description Send a notification to multiple users
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.BroadcastNotificationRequest true "Notification details"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/notifications/broadcast [post]
func (h *AdminHandler) BroadcastNotification(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	
	var req models.BroadcastNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	if err := h.validator.Validate(&req); err != nil {
		utils.SendBadRequest(c, err.Error(), err)
		return
	}
	
	err := h.adminService.BroadcastNotification(c.Request.Context(), &req, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Notification sent successfully", nil)
}

// SendTargetedNotification godoc
// @Summary Send targeted notification
// @Description Send a notification to specific users
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.BroadcastNotificationRequest true "Notification details"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/notifications/send [post]
func (h *AdminHandler) SendTargetedNotification(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	
	var req models.BroadcastNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}
	
	if len(req.UserIDs) == 0 {
		utils.SendBadRequest(c, "user_ids is required for targeted notifications", nil)
		return
	}
	
	err := h.adminService.BroadcastNotification(c.Request.Context(), &req, adminID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Notification sent successfully", nil)
}

func (h *AdminHandler) handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}
	h.logger.Error("Unhandled error in admin handler", zap.Error(err))
	utils.SendInternalServerError(c, "An error occurred", err)
}
