package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	ws "github.com/hamsaya/backend/pkg/websocket"
	"go.uber.org/zap"
)

// ChatService handles chat operations
type ChatService struct {
	conversationRepo repositories.ConversationRepository
	messageRepo      repositories.MessageRepository
	userRepo         repositories.UserRepository
	wsHub            *ws.Hub
	logger           *zap.Logger
}

// NewChatService creates a new chat service
func NewChatService(
	conversationRepo repositories.ConversationRepository,
	messageRepo repositories.MessageRepository,
	userRepo repositories.UserRepository,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		userRepo:         userRepo,
		wsHub:            wsHub,
		logger:           logger,
	}
}

// SendMessage sends a message to another user
func (s *ChatService) SendMessage(ctx context.Context, senderID string, req *models.SendMessageRequest) (*models.MessageResponse, error) {
	// Validate message content
	if req.MessageType == models.MessageTypeText && (req.Content == nil || *req.Content == "") {
		return nil, utils.NewBadRequestError("Content is required for text messages", nil)
	}

	// Get or create conversation
	conversation, err := s.conversationRepo.GetOrCreate(ctx, senderID, req.RecipientID)
	if err != nil {
		s.logger.Error("Failed to get or create conversation",
			zap.Error(err),
			zap.String("sender_id", senderID),
			zap.String("recipient_id", req.RecipientID),
		)
		return nil, utils.NewInternalError("Failed to create conversation", err)
	}

	// Create message
	messageID := uuid.New().String()
	message := &models.Message{
		ID:             messageID,
		ConversationID: conversation.ID,
		SenderID:       senderID,
		Content:        req.Content,
		MessageType:    req.MessageType,
		CreatedAt:      time.Now(),
	}

	if err := s.messageRepo.Create(ctx, message); err != nil {
		s.logger.Error("Failed to create message",
			zap.Error(err),
			zap.String("conversation_id", conversation.ID),
		)
		return nil, utils.NewInternalError("Failed to send message", err)
	}

	// Update conversation's last_message_at
	if err := s.conversationRepo.UpdateLastMessageAt(ctx, conversation.ID); err != nil {
		s.logger.Warn("Failed to update last_message_at",
			zap.Error(err),
			zap.String("conversation_id", conversation.ID),
		)
	}

	s.logger.Info("Message sent",
		zap.String("message_id", messageID),
		zap.String("sender_id", senderID),
		zap.String("recipient_id", req.RecipientID),
	)

	// Send real-time notification to recipient via WebSocket
	go s.notifyMessageSent(message, req.RecipientID)

	// Get enriched message response
	return s.enrichMessage(ctx, message)
}

// GetConversations retrieves all conversations for a user
func (s *ChatService) GetConversations(ctx context.Context, userID string, limit, offset int) ([]*models.ConversationResponse, error) {
	filter := &models.GetConversationsFilter{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	}

	conversations, err := s.conversationRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list conversations",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return nil, utils.NewInternalError("Failed to get conversations", err)
	}

	// Enrich conversations
	var enrichedConversations []*models.ConversationResponse
	for _, conversation := range conversations {
		enriched, err := s.enrichConversation(ctx, conversation, userID)
		if err != nil {
			s.logger.Warn("Failed to enrich conversation",
				zap.Error(err),
				zap.String("conversation_id", conversation.ID),
			)
			continue
		}
		enrichedConversations = append(enrichedConversations, enriched)
	}

	return enrichedConversations, nil
}

// GetMessages retrieves messages in a conversation
func (s *ChatService) GetMessages(ctx context.Context, userID, conversationID string, limit, offset int) ([]*models.MessageResponse, error) {
	// Check if user is participant
	isParticipant, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		s.logger.Error("Failed to check participant",
			zap.Error(err),
			zap.String("conversation_id", conversationID),
		)
		return nil, utils.NewInternalError("Failed to verify access", err)
	}

	if !isParticipant {
		return nil, utils.NewForbiddenError("You don't have access to this conversation", nil)
	}

	// Get messages
	filter := &models.GetMessagesFilter{
		ConversationID: conversationID,
		Limit:          limit,
		Offset:         offset,
	}

	messages, err := s.messageRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list messages",
			zap.Error(err),
			zap.String("conversation_id", conversationID),
		)
		return nil, utils.NewInternalError("Failed to get messages", err)
	}

	// Enrich messages
	var enrichedMessages []*models.MessageResponse
	for _, message := range messages {
		enriched, err := s.enrichMessage(ctx, message)
		if err != nil {
			s.logger.Warn("Failed to enrich message",
				zap.Error(err),
				zap.String("message_id", message.ID),
			)
			continue
		}
		enrichedMessages = append(enrichedMessages, enriched)
	}

	return enrichedMessages, nil
}

// MarkConversationAsRead marks all unread messages in a conversation as read
func (s *ChatService) MarkConversationAsRead(ctx context.Context, userID, conversationID string) error {
	// Check if user is participant
	isParticipant, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return utils.NewInternalError("Failed to verify access", err)
	}

	if !isParticipant {
		return utils.NewForbiddenError("You don't have access to this conversation", nil)
	}

	// Mark as read
	if err := s.messageRepo.MarkConversationAsRead(ctx, conversationID, userID); err != nil {
		s.logger.Error("Failed to mark conversation as read",
			zap.Error(err),
			zap.String("conversation_id", conversationID),
		)
		return utils.NewInternalError("Failed to mark as read", err)
	}

	s.logger.Info("Conversation marked as read",
		zap.String("conversation_id", conversationID),
		zap.String("user_id", userID),
	)

	return nil
}

// DeleteMessage soft deletes a message
func (s *ChatService) DeleteMessage(ctx context.Context, userID, messageID string) error {
	// Get message
	message, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return utils.NewNotFoundError("Message not found", err)
	}

	// Check if user is the sender
	if message.SenderID != userID {
		return utils.NewForbiddenError("You can only delete your own messages", nil)
	}

	// Delete message
	if err := s.messageRepo.Delete(ctx, messageID); err != nil {
		s.logger.Error("Failed to delete message",
			zap.Error(err),
			zap.String("message_id", messageID),
		)
		return utils.NewInternalError("Failed to delete message", err)
	}

	s.logger.Info("Message deleted",
		zap.String("message_id", messageID),
		zap.String("user_id", userID),
	)

	return nil
}

// enrichConversation enriches a conversation with participant and last message info
func (s *ChatService) enrichConversation(ctx context.Context, conversation *models.Conversation, viewerID string) (*models.ConversationResponse, error) {
	response := &models.ConversationResponse{
		ID:            conversation.ID,
		LastMessageAt: conversation.LastMessageAt,
		CreatedAt:     conversation.CreatedAt,
	}

	// Get other participant ID
	otherParticipantID, err := s.conversationRepo.GetOtherParticipantID(ctx, conversation.ID, viewerID)
	if err != nil {
		return nil, err
	}

	// Get other participant's profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, otherParticipantID)
	if err == nil {
		firstName := ""
		if profile.FirstName != nil {
			firstName = *profile.FirstName
		}
		lastName := ""
		if profile.LastName != nil {
			lastName = *profile.LastName
		}
		response.OtherParticipant = &models.UserInfo{
			UserID:    otherParticipantID,
			FirstName: firstName,
			LastName:  lastName,
			FullName:  profile.FullName(),
			Avatar:    profile.Avatar,
		}
	}

	// Get last message
	lastMessage, err := s.messageRepo.GetLastMessage(ctx, conversation.ID)
	if err == nil && lastMessage != nil {
		response.LastMessage = &models.MessageInfo{
			ID:          lastMessage.ID,
			Content:     lastMessage.Content,
			MessageType: lastMessage.MessageType,
			SenderID:    lastMessage.SenderID,
			CreatedAt:   lastMessage.CreatedAt,
		}
	}

	// Get unread count
	unreadCount, err := s.messageRepo.GetUnreadCount(ctx, conversation.ID, viewerID)
	if err == nil {
		response.UnreadCount = unreadCount
	}

	return response, nil
}

// enrichMessage enriches a message with sender info
func (s *ChatService) enrichMessage(ctx context.Context, message *models.Message) (*models.MessageResponse, error) {
	response := &models.MessageResponse{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Content:        message.Content,
		MessageType:    message.MessageType,
		IsRead:         message.ReadAt != nil,
		CreatedAt:      message.CreatedAt,
	}

	// Get sender's profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, message.SenderID)
	if err == nil {
		firstName := ""
		if profile.FirstName != nil {
			firstName = *profile.FirstName
		}
		lastName := ""
		if profile.LastName != nil {
			lastName = *profile.LastName
		}
		response.Sender = &models.UserInfo{
			UserID:    message.SenderID,
			FirstName: firstName,
			LastName:  lastName,
			FullName:  profile.FullName(),
			Avatar:    profile.Avatar,
		}
	}

	return response, nil
}

// notifyMessageSent sends a WebSocket notification to the recipient
func (s *ChatService) notifyMessageSent(message *models.Message, recipientID string) {
	// Send WebSocket message to recipient if they're connected
	wsMessage := models.WSMessage{
		Type: "message",
		Payload: models.WSMessagePayload{
			ConversationID: message.ConversationID,
			MessageID:      message.ID,
			SenderID:       message.SenderID,
			Content:        message.Content,
			MessageType:    message.MessageType,
			CreatedAt:      message.CreatedAt,
		},
	}

	if err := s.wsHub.SendToUser(recipientID, wsMessage); err != nil {
		s.logger.Warn("Failed to send WebSocket notification",
			zap.Error(err),
			zap.String("recipient_id", recipientID),
		)
	}
}
