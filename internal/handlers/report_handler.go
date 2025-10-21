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

// ReportHandler handles report-related HTTP requests
type ReportHandler struct {
	reportService *services.ReportService
	logger        *zap.SugaredLogger
}

// NewReportHandler creates a new report handler
func NewReportHandler(reportService *services.ReportService) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
		logger:        utils.GetLogger(),
	}
}

// handleError handles errors in a consistent way
func (h *ReportHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	utils.SendInternalServerError(c, "An error occurred", err)
}

// ReportPost godoc
// @Summary Report a post
// @Description Create a report for a post
// @Tags reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param post_id path string true "Post ID"
// @Param request body models.CreatePostReportRequest true "Report details"
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /posts/{post_id}/report [post]
func (h *ReportHandler) ReportPost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("post_id")

	h.logger.Infow("Received post report request",
		"user_id", userID,
		"post_id", postID,
		"ip", c.ClientIP(),
	)

	var req models.CreatePostReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnw("Invalid post report request body", "user_id", userID, "error", err)
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.ReportPost(c.Request.Context(), userID, postID, &req); err != nil {
		h.handleError(c, err)
		return
	}

	h.logger.Infow("Post report created", "user_id", userID, "post_id", postID)
	utils.SendCreated(c, "Post reported successfully", nil)
}

// ReportComment godoc
// @Summary Report a comment
// @Description Create a report for a comment
// @Tags reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param comment_id path string true "Comment ID"
// @Param request body models.CreateCommentReportRequest true "Report details"
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /comments/{comment_id}/report [post]
func (h *ReportHandler) ReportComment(c *gin.Context) {
	userID := c.GetString("user_id")
	commentID := c.Param("comment_id")

	var req models.CreateCommentReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.ReportComment(c.Request.Context(), userID, commentID, &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendCreated(c, "Comment reported successfully", nil)
}

// ReportUser godoc
// @Summary Report a user
// @Description Create a report for a user
// @Tags reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param request body models.CreateUserReportRequest true "Report details"
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /users/{user_id}/report [post]
func (h *ReportHandler) ReportUser(c *gin.Context) {
	reporterID := c.GetString("user_id")
	reportedUserID := c.Param("user_id")

	var req models.CreateUserReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.ReportUser(c.Request.Context(), reporterID, reportedUserID, &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendCreated(c, "User reported successfully", nil)
}

// ReportBusiness godoc
// @Summary Report a business
// @Description Create a report for a business
// @Tags reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param request body models.CreateBusinessReportRequest true "Report details"
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /businesses/{business_id}/report [post]
func (h *ReportHandler) ReportBusiness(c *gin.Context) {
	userID := c.GetString("user_id")
	businessID := c.Param("business_id")

	var req models.CreateBusinessReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.ReportBusiness(c.Request.Context(), userID, businessID, &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendCreated(c, "Business reported successfully", nil)
}

// ListPostReports godoc
// @Summary List all post reports (Admin only)
// @Description Get a paginated list of all post reports
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.ReportListResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/posts [get]
func (h *ReportHandler) ListPostReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	response, err := h.reportService.ListPostReports(c.Request.Context(), page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post reports fetched successfully", response)
}

// ListCommentReports godoc
// @Summary List all comment reports (Admin only)
// @Description Get a paginated list of all comment reports
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.ReportListResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/comments [get]
func (h *ReportHandler) ListCommentReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	response, err := h.reportService.ListCommentReports(c.Request.Context(), page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment reports fetched successfully", response)
}

// ListUserReports godoc
// @Summary List all user reports (Admin only)
// @Description Get a paginated list of all user reports
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.ReportListResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/users [get]
func (h *ReportHandler) ListUserReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	response, err := h.reportService.ListUserReports(c.Request.Context(), page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User reports fetched successfully", response)
}

// ListBusinessReports godoc
// @Summary List all business reports (Admin only)
// @Description Get a paginated list of all business reports
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} utils.Response{data=models.ReportListResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/businesses [get]
func (h *ReportHandler) ListBusinessReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	response, err := h.reportService.ListBusinessReports(c.Request.Context(), page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business reports fetched successfully", response)
}

// GetPostReport godoc
// @Summary Get a specific post report (Admin only)
// @Description Get details of a specific post report by ID
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.PostReport}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/posts/{id} [get]
func (h *ReportHandler) GetPostReport(c *gin.Context) {
	reportID := c.Param("id")

	report, err := h.reportService.GetPostReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Post report fetched successfully", report)
}

// GetCommentReport godoc
// @Summary Get a specific comment report (Admin only)
// @Description Get details of a specific comment report by ID
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.CommentReport}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/comments/{id} [get]
func (h *ReportHandler) GetCommentReport(c *gin.Context) {
	reportID := c.Param("id")

	report, err := h.reportService.GetCommentReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Comment report fetched successfully", report)
}

// GetUserReport godoc
// @Summary Get a specific user report (Admin only)
// @Description Get details of a specific user report by ID
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.UserReport}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/users/{id} [get]
func (h *ReportHandler) GetUserReport(c *gin.Context) {
	reportID := c.Param("id")

	report, err := h.reportService.GetUserReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User report fetched successfully", report)
}

// GetBusinessReport godoc
// @Summary Get a specific business report (Admin only)
// @Description Get details of a specific business report by ID
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Success 200 {object} utils.Response{data=models.BusinessReport}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/businesses/{id} [get]
func (h *ReportHandler) GetBusinessReport(c *gin.Context) {
	reportID := c.Param("id")

	report, err := h.reportService.GetBusinessReport(c.Request.Context(), reportID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Business report fetched successfully", report)
}

// UpdatePostReportStatus godoc
// @Summary Update post report status (Admin only)
// @Description Update the status of a post report
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Param request body models.UpdateReportStatusRequest true "Status update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/posts/{id}/status [put]
func (h *ReportHandler) UpdatePostReportStatus(c *gin.Context) {
	reportID := c.Param("id")

	var req models.UpdateReportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.UpdatePostReportStatus(c.Request.Context(), reportID, req.Status); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Report status updated successfully", nil)
}

// UpdateCommentReportStatus godoc
// @Summary Update comment report status (Admin only)
// @Description Update the status of a comment report
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Param request body models.UpdateReportStatusRequest true "Status update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/comments/{id}/status [put]
func (h *ReportHandler) UpdateCommentReportStatus(c *gin.Context) {
	reportID := c.Param("id")

	var req models.UpdateReportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.UpdateCommentReportStatus(c.Request.Context(), reportID, req.Status); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Report status updated successfully", nil)
}

// UpdateUserReportStatus godoc
// @Summary Update user report status (Admin only)
// @Description Update the resolved status of a user report
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Param request body map[string]bool true "Resolved status" example({"resolved": true})
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/users/{id}/status [put]
func (h *ReportHandler) UpdateUserReportStatus(c *gin.Context) {
	reportID := c.Param("id")

	var req struct {
		Resolved bool `json:"resolved" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.UpdateUserReportStatus(c.Request.Context(), reportID, req.Resolved); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Report status updated successfully", nil)
}

// UpdateBusinessReportStatus godoc
// @Summary Update business report status (Admin only)
// @Description Update the status of a business report
// @Tags admin,reports
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Param request body models.UpdateReportStatusRequest true "Status update"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /admin/reports/businesses/{id}/status [put]
func (h *ReportHandler) UpdateBusinessReportStatus(c *gin.Context) {
	reportID := c.Param("id")

	var req models.UpdateReportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	if err := h.reportService.UpdateBusinessReportStatus(c.Request.Context(), reportID, req.Status); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Report status updated successfully", nil)
}
