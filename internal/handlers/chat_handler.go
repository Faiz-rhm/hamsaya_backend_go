package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	ws "github.com/hamsaya/backend/pkg/websocket"
	"go.uber.org/zap"
)

// ChatHandler handles chat-related endpoints
type ChatHandler struct {
	chatService *services.ChatService
	wsHub       *ws.Hub
	validator   *utils.Validator
	logger      *zap.Logger
	upgrader    websocket.Upgrader
}

// NewChatHandler creates a new chat handler
func NewChatHandler(
	chatService *services.ChatService,
	wsHub *ws.Hub,
	validator *utils.Validator,
	logger *zap.Logger,
	cfg *config.Config,
) *ChatHandler {
	// Create upgrader with proper origin checking
	allowedOrigins := cfg.CORS.AllowedOrigins

	return &ChatHandler{
		chatService: chatService,
		wsHub:       wsHub,
		validator:   validator,
		logger:      logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")

				// If no origin header, reject (common for non-browser clients)
				if origin == "" {
					logger.Warn("WebSocket connection rejected: no origin header")
					return false
				}

				// Check if origin is in allowed list
				for _, allowedOrigin := range allowedOrigins {
					// Handle wildcard
					if allowedOrigin == "*" {
						return true
					}

					// Exact match or wildcard subdomain
					if origin == allowedOrigin || strings.HasSuffix(origin, allowedOrigin) {
						return true
					}
				}

				logger.Warn("WebSocket connection rejected: origin not allowed",
					zap.String("origin", origin),
					zap.Strings("allowed_origins", allowedOrigins),
				)
				return false
			},
		},
	}
}

// HandleWebSocket handles WebSocket connections for real-time chat
func (h *ChatHandler) HandleWebSocket(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection",
			zap.Error(err),
			zap.String("user_id", userID.(string)),
		)
		return
	}

	// Create client
	client := &ws.Client{
		ID:   userID.(string),
		Conn: conn,
		Hub:  h.wsHub,
		Send: make(chan []byte, 256),
	}

	// Register client with hub
	h.wsHub.Register(client)

	// Start client goroutines
	go client.WritePump()
	go client.ReadPump()

	h.logger.Info("WebSocket connection established",
		zap.String("user_id", userID.(string)),
	)
}

// SendMessage handles POST /api/v1/chat/messages
func (h *ChatHandler) SendMessage(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	// Parse request
	var req models.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Send message
	message, err := h.chatService.SendMessage(c.Request.Context(), userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Message sent successfully", message)
}

// GetConversations handles GET /api/v1/chat/conversations
func (h *ChatHandler) GetConversations(c *gin.Context) {
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

	// Get conversations
	conversations, err := h.chatService.GetConversations(c.Request.Context(), userID.(string), limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Conversations retrieved successfully", conversations)
}

// GetMessages handles GET /api/v1/chat/conversations/:conversation_id/messages
func (h *ChatHandler) GetMessages(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		utils.SendError(c, http.StatusBadRequest, "Conversation ID is required", utils.ErrBadRequest)
		return
	}

	// Parse pagination
	limit := 50
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

	// Get messages
	messages, err := h.chatService.GetMessages(c.Request.Context(), userID.(string), conversationID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Messages retrieved successfully", messages)
}

// MarkConversationAsRead handles POST /api/v1/chat/conversations/:conversation_id/read
func (h *ChatHandler) MarkConversationAsRead(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		utils.SendError(c, http.StatusBadRequest, "Conversation ID is required", utils.ErrBadRequest)
		return
	}

	// Mark as read
	if err := h.chatService.MarkConversationAsRead(c.Request.Context(), userID.(string), conversationID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Conversation marked as read", nil)
}

// DeleteMessage handles DELETE /api/v1/chat/messages/:message_id
func (h *ChatHandler) DeleteMessage(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	messageID := c.Param("message_id")
	if messageID == "" {
		utils.SendError(c, http.StatusBadRequest, "Message ID is required", utils.ErrBadRequest)
		return
	}

	// Delete message
	if err := h.chatService.DeleteMessage(c.Request.Context(), userID.(string), messageID); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Message deleted successfully", nil)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *ChatHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in chat handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
