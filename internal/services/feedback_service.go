package services

import (
	"context"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// FeedbackService handles feedback-related business logic
type FeedbackService struct {
	feedbackRepo repositories.FeedbackRepository
	validator    *utils.Validator
	logger       *zap.SugaredLogger
}

// NewFeedbackService creates a new feedback service
func NewFeedbackService(
	feedbackRepo repositories.FeedbackRepository,
	validator *utils.Validator,
) *FeedbackService {
	return &FeedbackService{
		feedbackRepo: feedbackRepo,
		validator:    validator,
		logger:       utils.GetLogger(),
	}
}

// SubmitFeedback creates user feedback
func (s *FeedbackService) SubmitFeedback(ctx context.Context, userID string, req *models.CreateFeedbackRequest) (*models.FeedbackResponse, error) {
	s.logger.Infow("Processing feedback submission",
		"user_id", userID,
		"rating", req.Rating,
		"type", req.Type,
	)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		s.logger.Warnw("Feedback validation failed", "user_id", userID, "error", err)
		return nil, utils.NewBadRequestError("Invalid request", err)
	}

	// Create feedback
	feedback := &models.Feedback{
		UserID:     userID,
		Rating:     req.Rating,
		Type:       req.Type,
		Message:    req.Message,
		AppVersion: req.AppVersion,
		DeviceInfo: req.DeviceInfo,
	}

	if err := s.feedbackRepo.Create(ctx, feedback); err != nil {
		s.logger.Errorw("Failed to create feedback", "user_id", userID, "error", err)
		return nil, utils.NewInternalServerError("Failed to submit feedback", err)
	}

	s.logger.Infow("Feedback submitted successfully", "user_id", userID, "feedback_id", feedback.ID)

	return &models.FeedbackResponse{
		ID:        feedback.ID,
		Message:   "Thank you for your feedback!",
		CreatedAt: feedback.CreatedAt,
	}, nil
}

// GetFeedbackStatus checks if user has submitted feedback recently
func (s *FeedbackService) GetFeedbackStatus(ctx context.Context, userID string) (*models.FeedbackStatusResponse, error) {
	hasSubmitted, lastFeedback, err := s.feedbackRepo.GetUserFeedbackStatus(ctx, userID)
	if err != nil {
		s.logger.Errorw("Failed to get feedback status", "user_id", userID, "error", err)
		return nil, utils.NewInternalServerError("Failed to get feedback status", err)
	}

	// Consider feedback "recent" if submitted within last 30 days
	if hasSubmitted && lastFeedback != nil {
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
		if lastFeedback.Before(thirtyDaysAgo) {
			hasSubmitted = false
		}
	}

	return &models.FeedbackStatusResponse{
		HasSubmitted: hasSubmitted,
		LastFeedback: lastFeedback,
	}, nil
}
