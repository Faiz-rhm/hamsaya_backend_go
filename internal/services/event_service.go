package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// EventService handles event interest operations
type EventService struct {
	eventRepo repositories.EventRepository
	postRepo  repositories.PostRepository
	userRepo  repositories.UserRepository
	logger    *zap.Logger
}

// NewEventService creates a new event service
func NewEventService(
	eventRepo repositories.EventRepository,
	postRepo repositories.PostRepository,
	userRepo repositories.UserRepository,
	logger *zap.Logger,
) *EventService {
	return &EventService{
		eventRepo: eventRepo,
		postRepo:  postRepo,
		userRepo:  userRepo,
		logger:    logger,
	}
}

// SetEventInterest sets a user's interest level for an event
func (s *EventService) SetEventInterest(ctx context.Context, postID, userID string, req *models.EventInterestRequest) (*models.EventInterestResponse, error) {
	// Validate post exists and is of type EVENT
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	if post.Type != models.PostTypeEvent {
		return nil, utils.NewBadRequestError("Interest can only be set for EVENT type posts", nil)
	}

	// Check if user already expressed interest
	existingInterest, err := s.eventRepo.GetUserInterest(ctx, userID, postID)
	if err != nil {
		s.logger.Error("Failed to get existing interest", zap.Error(err))
		return nil, utils.NewInternalError("Failed to check existing interest", err)
	}

	// If user already has the same interest, just return current status
	if existingInterest != nil && existingInterest.EventState == req.EventState {
		return s.GetEventInterestStatus(ctx, postID, &userID)
	}

	// Create or update interest
	now := time.Now()
	interest := &models.EventInterest{
		ID:         uuid.New().String(),
		PostID:     postID,
		UserID:     userID,
		EventState: req.EventState,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Use existing ID if updating
	if existingInterest != nil {
		interest.ID = existingInterest.ID
		interest.CreatedAt = existingInterest.CreatedAt
	}

	if err := s.eventRepo.SetInterest(ctx, interest); err != nil {
		s.logger.Error("Failed to set event interest", zap.Error(err))
		return nil, utils.NewInternalError("Failed to set event interest", err)
	}

	s.logger.Info("Event interest set",
		zap.String("post_id", postID),
		zap.String("user_id", userID),
		zap.String("state", string(req.EventState)),
	)

	// Return updated status
	return s.GetEventInterestStatus(ctx, postID, &userID)
}

// RemoveEventInterest removes a user's interest from an event
func (s *EventService) RemoveEventInterest(ctx context.Context, postID, userID string) error {
	// Validate post exists
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return utils.NewNotFoundError("Post not found", err)
	}

	if post.Type != models.PostTypeEvent {
		return utils.NewBadRequestError("Interest can only be removed from EVENT type posts", nil)
	}

	// Check if user has expressed interest
	existingInterest, err := s.eventRepo.GetUserInterest(ctx, userID, postID)
	if err != nil {
		return utils.NewInternalError("Failed to check existing interest", err)
	}

	if existingInterest == nil {
		return utils.NewBadRequestError("User has not expressed interest in this event", nil)
	}

	// Delete interest (trigger will handle decrementing counts)
	if err := s.eventRepo.DeleteInterest(ctx, userID, postID); err != nil {
		s.logger.Error("Failed to delete event interest", zap.Error(err))
		return utils.NewInternalError("Failed to delete event interest", err)
	}

	s.logger.Info("Event interest removed",
		zap.String("post_id", postID),
		zap.String("user_id", userID),
	)

	return nil
}

// GetEventInterestStatus gets the interest status for an event
func (s *EventService) GetEventInterestStatus(ctx context.Context, postID string, viewerID *string) (*models.EventInterestResponse, error) {
	// Validate post exists
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	if post.Type != models.PostTypeEvent {
		return nil, utils.NewBadRequestError("Interest status can only be retrieved for EVENT type posts", nil)
	}

	response := &models.EventInterestResponse{
		PostID:          postID,
		InterestedCount: post.InterestedCount,
		GoingCount:      post.GoingCount,
	}

	// Get user's interest if viewer is authenticated
	if viewerID != nil && *viewerID != "" {
		userInterest, err := s.eventRepo.GetUserInterest(ctx, *viewerID, postID)
		if err == nil && userInterest != nil {
			response.UserEventState = userInterest.EventState
		}
	}

	return response, nil
}

// GetInterestedUsers gets users who are interested in an event
func (s *EventService) GetInterestedUsers(ctx context.Context, postID string, limit, offset int) ([]*models.EventInterestedUser, error) {
	// Validate post exists
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	if post.Type != models.PostTypeEvent {
		return nil, utils.NewBadRequestError("Can only get interested users for EVENT type posts", nil)
	}

	// Get interested users
	interests, err := s.eventRepo.GetInterestedUsers(ctx, postID, models.EventInterestInterested, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get interested users", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get interested users", err)
	}

	return s.enrichEventInterests(ctx, interests)
}

// GetGoingUsers gets users who are going to an event
func (s *EventService) GetGoingUsers(ctx context.Context, postID string, limit, offset int) ([]*models.EventInterestedUser, error) {
	// Validate post exists
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	if post.Type != models.PostTypeEvent {
		return nil, utils.NewBadRequestError("Can only get going users for EVENT type posts", nil)
	}

	// Get going users
	interests, err := s.eventRepo.GetInterestedUsers(ctx, postID, models.EventInterestGoing, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get going users", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get going users", err)
	}

	return s.enrichEventInterests(ctx, interests)
}

// enrichEventInterests enriches event interests with user information
func (s *EventService) enrichEventInterests(ctx context.Context, interests []*models.EventInterest) ([]*models.EventInterestedUser, error) {
	var enrichedUsers []*models.EventInterestedUser

	for _, interest := range interests {
		// Get user profile
		profile, err := s.userRepo.GetProfileByUserID(ctx, interest.UserID)
		if err != nil {
			s.logger.Warn("Failed to get user profile",
				zap.String("user_id", interest.UserID),
				zap.Error(err),
			)
			continue
		}

		enrichedUser := &models.EventInterestedUser{
			User: &models.AuthorInfo{
				UserID:       interest.UserID,
				FirstName:    profile.FirstName,
				LastName:     profile.LastName,
				FullName:     profile.FullName(),
				Avatar:       profile.Avatar,
				Province:     profile.Province,
				District:     profile.District,
				Neighborhood: profile.Neighborhood,
			},
			EventState: interest.EventState,
			CreatedAt:  interest.CreatedAt,
		}

		enrichedUsers = append(enrichedUsers, enrichedUser)
	}

	return enrichedUsers, nil
}
