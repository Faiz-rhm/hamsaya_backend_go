package services

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// HelpChatService handles the support chat between users and admins.
type HelpChatService struct {
	repo   repositories.HelpChatRepository
	logger *zap.Logger
}

// NewHelpChatService creates a new HelpChatService.
func NewHelpChatService(repo repositories.HelpChatRepository, logger *zap.Logger) *HelpChatService {
	return &HelpChatService{repo: repo, logger: logger}
}

// SendUserMessage stores a message sent by a user.
func (s *HelpChatService) SendUserMessage(ctx context.Context, userID string, req *models.SendHelpMessageRequest) (*models.HelpChatMessage, error) {
	msg := &models.HelpChatMessage{
		UserID:     userID,
		Content:    req.Content,
		IsFromUser: true,
		AppVersion: req.AppVersion,
		DeviceInfo: req.DeviceInfo,
	}
	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		s.logger.Error("HelpChatService: create user message", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to send message", err)
	}
	return msg, nil
}

// GetUserMessages returns the full thread for the calling user.
func (s *HelpChatService) GetUserMessages(ctx context.Context, userID string, page, limit int) ([]*models.HelpChatMessage, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	msgs, total, err := s.repo.GetMessages(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, utils.NewInternalError("Failed to get messages", err)
	}
	return msgs, total, nil
}

// AdminReply stores a support reply from an admin.
func (s *HelpChatService) AdminReply(ctx context.Context, adminID, targetUserID string, req *models.AdminReplyRequest) (*models.HelpChatMessage, error) {
	msg := &models.HelpChatMessage{
		UserID:     targetUserID,
		Content:    req.Content,
		IsFromUser: false,
	}
	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		s.logger.Error("HelpChatService: admin reply", zap.String("admin_id", adminID), zap.String("target_user", targetUserID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to send reply", err)
	}
	return msg, nil
}

// GetAllThreads returns the admin inbox (one summary row per user).
func (s *HelpChatService) GetAllThreads(ctx context.Context, page, limit int) ([]*models.HelpChatThread, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	threads, total, err := s.repo.GetAllUserThreads(ctx, limit, offset)
	if err != nil {
		return nil, 0, utils.NewInternalError("Failed to get threads", err)
	}
	return threads, total, nil
}

// GetUserThread returns the full message history for a specific user (admin view).
func (s *HelpChatService) GetUserThread(ctx context.Context, userID string, page, limit int) ([]*models.HelpChatMessage, int64, error) {
	return s.GetUserMessages(ctx, userID, page, limit)
}
