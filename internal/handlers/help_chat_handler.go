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

// HelpChatHandler handles user-facing help chat endpoints.
type HelpChatHandler struct {
	svc       *services.HelpChatService
	validator *utils.Validator
	logger    *zap.Logger
}

// NewHelpChatHandler creates a new HelpChatHandler.
func NewHelpChatHandler(svc *services.HelpChatService, validator *utils.Validator, logger *zap.Logger) *HelpChatHandler {
	return &HelpChatHandler{svc: svc, validator: validator, logger: logger}
}

// SendMessage godoc
// @Summary Send help message
// @Description Send a message to support
// @Tags help-chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.SendHelpMessageRequest true "Message"
// @Success 201 {object} utils.Response{data=models.HelpChatMessage}
// @Failure 400 {object} utils.Response
// @Router /help-chat/messages [post]
func (h *HelpChatHandler) SendMessage(c *gin.Context) {
	userID := c.GetString("user_id")
	var req models.SendHelpMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	msg, err := h.svc.SendUserMessage(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusCreated, "Message sent", msg)
}

// GetMessages godoc
// @Summary Get help messages
// @Description Get the calling user's support thread
// @Tags help-chat
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Success 200 {object} utils.Response{data=[]models.HelpChatMessage}
// @Router /help-chat/messages [get]
func (h *HelpChatHandler) GetMessages(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	msgs, total, err := h.svc.GetUserMessages(c.Request.Context(), userID, page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Messages retrieved", gin.H{
		"messages": msgs,
		"total":    total,
	})
}

// AdminGetThreads godoc
// @Summary List all help chat threads (admin)
// @Description Returns one row per user showing their latest message
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page" default(1)
// @Param limit query int false "Per page" default(50)
// @Success 200 {object} utils.Response
// @Router /admin/help-chat [get]
func (h *HelpChatHandler) AdminGetThreads(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	threads, total, err := h.svc.GetAllThreads(c.Request.Context(), page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Threads retrieved", gin.H{
		"threads": threads,
		"total":   total,
	})
}

// AdminGetUserThread godoc
// @Summary Get a user's full help chat thread (admin)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param page query int false "Page" default(1)
// @Param limit query int false "Per page" default(50)
// @Success 200 {object} utils.Response{data=[]models.HelpChatMessage}
// @Router /admin/help-chat/{user_id} [get]
func (h *HelpChatHandler) AdminGetUserThread(c *gin.Context) {
	userID := c.Param("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	msgs, total, err := h.svc.GetUserThread(c.Request.Context(), userID, page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Messages retrieved", gin.H{
		"messages": msgs,
		"total":    total,
	})
}

// AdminReply godoc
// @Summary Reply to a user's help chat (admin)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param request body models.AdminReplyRequest true "Reply"
// @Success 201 {object} utils.Response{data=models.HelpChatMessage}
// @Router /admin/help-chat/{user_id}/reply [post]
func (h *HelpChatHandler) AdminReply(c *gin.Context) {
	adminID := c.GetString("user_id")
	targetUserID := c.Param("user_id")
	var req models.AdminReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	msg, err := h.svc.AdminReply(c.Request.Context(), adminID, targetUserID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusCreated, "Reply sent", msg)
}

func (h *HelpChatHandler) handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}
	h.logger.Error("Unhandled error in help chat handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
