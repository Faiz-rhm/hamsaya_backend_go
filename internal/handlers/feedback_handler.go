package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// FeedbackHandler handles feedback-related HTTP requests
type FeedbackHandler struct {
	feedbackService *services.FeedbackService
	logger          *zap.SugaredLogger
}

// NewFeedbackHandler creates a new feedback handler
func NewFeedbackHandler(feedbackService *services.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{
		feedbackService: feedbackService,
		logger:          utils.GetLogger(),
	}
}

// SubmitFeedback godoc
// @Summary Submit user feedback
// @Description Submit feedback about the app
// @Tags feedback
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.CreateFeedbackRequest true "Feedback details"
// @Success 201 {object} utils.Response{data=models.FeedbackResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /feedback [post]
func (h *FeedbackHandler) SubmitFeedback(c *gin.Context) {
	userID := c.GetString("user_id")

	h.logger.Infow("Received feedback submission request",
		"user_id", userID,
		"ip", c.ClientIP(),
	)

	var req models.CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnw("Invalid feedback request body", "user_id", userID, "error", err)
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	response, err := h.feedbackService.SubmitFeedback(c.Request.Context(), userID, &req)
	if err != nil {
		if appErr, ok := err.(*utils.AppError); ok {
			utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
			return
		}
		utils.SendInternalServerError(c, "An error occurred", err)
		return
	}

	h.logger.Infow("Feedback submitted", "user_id", userID, "feedback_id", response.ID)
	utils.SendResponse(c, http.StatusCreated, "Feedback submitted successfully", response)
}

// GetFeedbackStatus godoc
// @Summary Get user feedback status
// @Description Check if user has submitted recent feedback
// @Tags feedback
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.FeedbackStatusResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /feedback/status [get]
func (h *FeedbackHandler) GetFeedbackStatus(c *gin.Context) {
	userID := c.GetString("user_id")

	response, err := h.feedbackService.GetFeedbackStatus(c.Request.Context(), userID)
	if err != nil {
		if appErr, ok := err.(*utils.AppError); ok {
			utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
			return
		}
		utils.SendInternalServerError(c, "An error occurred", err)
		return
	}

	utils.SendSuccess(c, "Feedback status retrieved", response)
}
