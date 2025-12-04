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

// PollService handles poll operations
type PollService struct {
	pollRepo repositories.PollRepository
	postRepo repositories.PostRepository
	logger   *zap.Logger
}

// NewPollService creates a new poll service
func NewPollService(
	pollRepo repositories.PollRepository,
	postRepo repositories.PostRepository,
	logger *zap.Logger,
) *PollService {
	return &PollService{
		pollRepo: pollRepo,
		postRepo: postRepo,
		logger:   logger,
	}
}

// CreatePoll creates a new poll for a PULL post
func (s *PollService) CreatePoll(ctx context.Context, postID string, req *models.CreatePollRequest) (*models.PollResponse, error) {
	// Validate post exists and is of type PULL
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	if post.Type != models.PostTypePull {
		return nil, utils.NewBadRequestError("Polls can only be created for PULL type posts", nil)
	}

	// Check if poll already exists for this post
	existingPoll, _ := s.pollRepo.GetByPostID(ctx, postID)
	if existingPoll != nil {
		return nil, utils.NewBadRequestError("Poll already exists for this post", nil)
	}

	// Create poll
	pollID := uuid.New().String()
	now := time.Now()

	poll := &models.Poll{
		ID:        pollID,
		PostID:    postID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.pollRepo.Create(ctx, poll); err != nil {
		s.logger.Error("Failed to create poll", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to create poll", err)
	}

	// Create poll options
	for _, optionText := range req.Options {
		option := &models.PollOption{
			ID:        uuid.New().String(),
			PollID:    pollID,
			Option:    optionText,
			VoteCount: 0,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.pollRepo.CreateOption(ctx, option); err != nil {
			s.logger.Error("Failed to create poll option",
				zap.String("poll_id", pollID),
				zap.Error(err),
			)
			// Continue with other options
		}
	}

	s.logger.Info("Poll created",
		zap.String("poll_id", pollID),
		zap.String("post_id", postID),
	)

	// Return enriched poll
	return s.GetPoll(ctx, pollID, nil)
}

// GetPoll gets a poll by ID with full details
func (s *PollService) GetPoll(ctx context.Context, pollID string, viewerID *string) (*models.PollResponse, error) {
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		s.logger.Warn("Poll not found", zap.String("poll_id", pollID), zap.Error(err))
		return nil, utils.NewNotFoundError("Poll not found", err)
	}

	return s.enrichPoll(ctx, poll, viewerID)
}

// GetPollByPostID gets a poll by post ID
func (s *PollService) GetPollByPostID(ctx context.Context, postID string, viewerID *string) (*models.PollResponse, error) {
	// Get poll
	poll, err := s.pollRepo.GetByPostID(ctx, postID)
	if err != nil {
		s.logger.Warn("Poll not found for post", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewNotFoundError("Poll not found", err)
	}

	return s.enrichPoll(ctx, poll, viewerID)
}

// VotePoll votes on a poll option
func (s *PollService) VotePoll(ctx context.Context, pollID, userID, optionID string) (*models.PollResponse, error) {
	// Validate poll exists
	_, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		return nil, utils.NewNotFoundError("Poll not found", err)
	}

	// Validate option exists and belongs to this poll
	option, err := s.pollRepo.GetOptionByID(ctx, optionID)
	if err != nil {
		return nil, utils.NewNotFoundError("Poll option not found", err)
	}

	if option.PollID != pollID {
		return nil, utils.NewBadRequestError("Poll option does not belong to this poll", nil)
	}

	// Check if user has already voted
	existingVote, err := s.pollRepo.GetUserVote(ctx, userID, pollID)
	if err != nil {
		s.logger.Error("Failed to get user vote", zap.Error(err))
		return nil, utils.NewInternalError("Failed to check existing vote", err)
	}

	// If user already voted and is voting for the same option, do nothing
	if existingVote != nil && existingVote.PollOptionID == optionID {
		return s.GetPoll(ctx, pollID, &userID)
	}

	// If user already voted for a different option, change vote
	if existingVote != nil && existingVote.PollOptionID != optionID {
		// Note: The database trigger will handle incrementing/decrementing vote counts
		if err := s.pollRepo.ChangeVote(ctx, userID, pollID, optionID); err != nil {
			s.logger.Error("Failed to change vote", zap.Error(err))
			return nil, utils.NewInternalError("Failed to change vote", err)
		}
	} else {
		// Create new vote
		vote := &models.UserPoll{
			ID:           uuid.New().String(),
			UserID:       userID,
			PollID:       pollID,
			PollOptionID: optionID,
			CreatedAt:    time.Now(),
		}

		if err := s.pollRepo.VotePoll(ctx, vote); err != nil {
			s.logger.Error("Failed to vote on poll", zap.Error(err))
			return nil, utils.NewInternalError("Failed to vote on poll", err)
		}
	}

	s.logger.Info("User voted on poll",
		zap.String("poll_id", pollID),
		zap.String("user_id", userID),
		zap.String("option_id", optionID),
	)

	// Return enriched poll
	return s.GetPoll(ctx, pollID, &userID)
}

// DeleteVote removes a user's vote from a poll
func (s *PollService) DeleteVote(ctx context.Context, pollID, userID string) error {
	// Validate poll exists
	_, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		return utils.NewNotFoundError("Poll not found", err)
	}

	// Check if user has voted
	existingVote, err := s.pollRepo.GetUserVote(ctx, userID, pollID)
	if err != nil {
		return utils.NewInternalError("Failed to check existing vote", err)
	}

	if existingVote == nil {
		return utils.NewBadRequestError("User has not voted on this poll", nil)
	}

	// Delete vote (trigger will handle decrementing vote count)
	if err := s.pollRepo.DeleteVote(ctx, userID, pollID); err != nil {
		s.logger.Error("Failed to delete vote", zap.Error(err))
		return utils.NewInternalError("Failed to delete vote", err)
	}

	s.logger.Info("User vote deleted",
		zap.String("poll_id", pollID),
		zap.String("user_id", userID),
	)

	return nil
}

// enrichPoll enriches a poll with options and user vote status
func (s *PollService) enrichPoll(ctx context.Context, poll *models.Poll, viewerID *string) (*models.PollResponse, error) {
	response := &models.PollResponse{
		ID:        poll.ID,
		PostID:    poll.PostID,
		HasVoted:  false,
		CreatedAt: poll.CreatedAt,
		UpdatedAt: poll.UpdatedAt,
	}

	// Get poll options
	options, err := s.pollRepo.GetOptionsByPollID(ctx, poll.ID)
	if err != nil {
		s.logger.Error("Failed to get poll options", zap.String("poll_id", poll.ID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get poll options", err)
	}

	// Calculate total votes
	totalVotes := 0
	for _, option := range options {
		totalVotes += option.VoteCount
	}
	response.TotalVotes = totalVotes

	// Build option responses with percentages
	var optionResponses []*models.PollOptionResponse
	for _, option := range options {
		percentage := 0.0
		if totalVotes > 0 {
			percentage = (float64(option.VoteCount) / float64(totalVotes)) * 100
		}

		optionResponses = append(optionResponses, &models.PollOptionResponse{
			ID:         option.ID,
			Option:     option.Option,
			VoteCount:  option.VoteCount,
			Percentage: percentage,
		})
	}
	response.Options = optionResponses

	// Get user's vote if viewer is authenticated
	if viewerID != nil && *viewerID != "" {
		s.logger.Info("Checking user vote",
			zap.String("viewer_id", *viewerID),
			zap.String("poll_id", poll.ID),
		)
		userVote, err := s.pollRepo.GetUserVote(ctx, *viewerID, poll.ID)
		if err != nil {
			s.logger.Warn("Error getting user vote",
				zap.String("viewer_id", *viewerID),
				zap.String("poll_id", poll.ID),
				zap.Error(err),
			)
		}
		if err == nil && userVote != nil {
			response.HasVoted = true
			response.UserVote = &userVote.PollOptionID
			s.logger.Info("User has voted",
				zap.String("viewer_id", *viewerID),
				zap.String("poll_id", poll.ID),
				zap.String("voted_option_id", userVote.PollOptionID),
			)
		}
	}

	return response, nil
}
