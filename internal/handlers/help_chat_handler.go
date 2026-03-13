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

// HelpChatHandler handles help center chat HTTP requests (not user-to-user chat).
type HelpChatHandler struct {
	helpChatService *services.HelpChatService
	logger           *zap.SugaredLogger
}

// NewHelpChatHandler creates a new help chat handler.
func NewHelpChatHandler(helpChatService *services.HelpChatService) *HelpChatHandler {
	return &HelpChatHandler{
		helpChatService: helpChatService,
		logger:          utils.GetLogger(),
	}
}

// SendMessage godoc
// @Summary Send a help center message
// @Description Send a message to support via the help center. Content may include [Image: url] placeholders. Not for user-to-user chat.
// @Tags help-chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.CreateHelpChatMessageRequest true "Message content"
// @Success 201 {object} utils.Response{data=models.HelpChatSendResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /help-chat/messages [post]
func (h *HelpChatHandler) SendMessage(c *gin.Context) {
	userID := c.GetString("user_id")

	h.logger.Infow("Help chat send message request", "user_id", userID, "ip", c.ClientIP())

	var req models.CreateHelpChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnw("Invalid help chat request body", "user_id", userID, "error", err)
		utils.SendBadRequest(c, "Invalid request body", err)
		return
	}

	response, err := h.helpChatService.SendMessage(c.Request.Context(), userID, &req)
	if err != nil {
		if appErr, ok := err.(*utils.AppError); ok {
			utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
			return
		}
		utils.SendInternalServerError(c, "An error occurred", err)
		return
	}

	h.logger.Infow("Help chat message sent", "user_id", userID, "message_id", response.ID)
	utils.SendSuccess(c, http.StatusCreated, "Message sent", response)
}

// ListMessages godoc
// @Summary List help center messages
// @Description List the current user's help center thread (oldest first). Not user-to-user chat.
// @Tags help-chat
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page (default 1)"
// @Param page_size query int false "Page size (default 50, max 100)"
// @Success 200 {object} utils.Response{data=[]models.HelpChatMessageResponse}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /help-chat/messages [get]
func (h *HelpChatHandler) ListMessages(c *gin.Context) {
	userID := c.GetString("user_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	messages, err := h.helpChatService.ListMessages(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		if appErr, ok := err.(*utils.AppError); ok {
			utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
			return
		}
		utils.SendInternalServerError(c, "An error occurred", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Messages retrieved", messages)
}
