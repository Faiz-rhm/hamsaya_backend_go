package handlers

import (
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
