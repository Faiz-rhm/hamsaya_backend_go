package services

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

const systemReplyContent = "Thank you for your message. We'll get back to you soon."

// HelpChatService handles help center chat business logic.
type HelpChatService struct {
	helpChatRepo repositories.HelpChatRepository
	validator    *utils.Validator
	logger       *zap.SugaredLogger
}

// NewHelpChatService creates a new help chat service.
func NewHelpChatService(
	helpChatRepo repositories.HelpChatRepository,
	validator *utils.Validator,
) *HelpChatService {
	return &HelpChatService{
		helpChatRepo: helpChatRepo,
		validator:    validator,
		logger:       utils.GetLogger(),
	}
}

// SendMessage stores a user message and an automatic system reply, returns the user message.
func (s *HelpChatService) SendMessage(ctx context.Context, userID string, req *models.CreateHelpChatMessageRequest) (*models.HelpChatSendResponse, error) {
	s.logger.Infow("Processing help chat message", "user_id", userID)

	if err := s.validator.Validate(req); err != nil {
		s.logger.Warnw("Help chat validation failed", "user_id", userID, "error", err)
		return nil, utils.NewBadRequestError("Invalid request", err)
	}

	userMsg := &models.HelpChatMessage{
		UserID:     userID,
		Content:    req.Message,
		IsFromUser: true,
		AppVersion: req.AppVersion,
		DeviceInfo: req.DeviceInfo,
	}

	if err := s.helpChatRepo.Create(ctx, userMsg); err != nil {
		s.logger.Errorw("Failed to create help chat message", "user_id", userID, "error", err)
		return nil, utils.NewInternalServerError("Failed to send message", err)
	}

	systemMsg := &models.HelpChatMessage{
		UserID:     userID,
		Content:    systemReplyContent,
		IsFromUser: false,
	}

	if err := s.helpChatRepo.Create(ctx, systemMsg); err != nil {
		s.logger.Errorw("Failed to create help chat system reply", "user_id", userID, "error", err)
		// User message was stored; still return success
	}

	s.logger.Infow("Help chat message sent", "user_id", userID, "message_id", userMsg.ID)

	return &models.HelpChatSendResponse{
		ID:        userMsg.ID,
		Message:   systemReplyContent,
		CreatedAt: userMsg.CreatedAt,
	}, nil
}

// ListMessages returns the help chat thread for the user (oldest first).
func (s *HelpChatService) ListMessages(ctx context.Context, userID string, page, pageSize int) ([]*models.HelpChatMessageResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	messages, err := s.helpChatRepo.ListByUserID(ctx, userID, pageSize, offset)
	if err != nil {
		s.logger.Errorw("Failed to list help chat messages", "user_id", userID, "error", err)
		return nil, utils.NewInternalServerError("Failed to load messages", err)
	}

	result := make([]*models.HelpChatMessageResponse, len(messages))
	for i, m := range messages {
		result[i] = &models.HelpChatMessageResponse{
			ID:         m.ID,
			Content:    m.Content,
			IsFromUser: m.IsFromUser,
			CreatedAt:  m.CreatedAt,
		}
	}

	return result, nil
}
