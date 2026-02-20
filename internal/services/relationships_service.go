package services

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// RelationshipsService handles user relationship operations
type RelationshipsService struct {
	relationshipsRepo   repositories.RelationshipsRepository
	userRepo            repositories.UserRepository
	notificationService *NotificationService
	logger              *zap.Logger
}

// NewRelationshipsService creates a new relationships service
func NewRelationshipsService(
	relationshipsRepo repositories.RelationshipsRepository,
	userRepo repositories.UserRepository,
	notificationService *NotificationService,
	logger *zap.Logger,
) *RelationshipsService {
	return &RelationshipsService{
		relationshipsRepo:   relationshipsRepo,
		userRepo:            userRepo,
		notificationService: notificationService,
		logger:              logger,
	}
}

// FollowUser follows a user
func (s *RelationshipsService) FollowUser(ctx context.Context, followerID, followingID string) error {
	// Validate that users are not the same
	if followerID == followingID {
		return utils.NewBadRequestError("Cannot follow yourself", nil)
	}

	// Check if target user exists
	_, err := s.userRepo.GetByID(ctx, followingID)
	if err != nil {
		s.logger.Warn("Target user not found", zap.String("user_id", followingID), zap.Error(err))
		return utils.NewNotFoundError("User not found", err)
	}

	// Check if already following
	isFollowing, err := s.relationshipsRepo.IsFollowing(ctx, followerID, followingID)
	if err != nil {
		s.logger.Error("Failed to check follow status", zap.Error(err))
		return utils.NewInternalError("Failed to check follow status", err)
	}

	if isFollowing {
		// Already following - this is idempotent, so just return success
		return nil
	}

	// Create follow relationship
	if err := s.relationshipsRepo.FollowUser(ctx, followerID, followingID); err != nil {
		s.logger.Error("Failed to follow user",
			zap.String("follower_id", followerID),
			zap.String("following_id", followingID),
			zap.Error(err),
		)
		return utils.NewInternalError("Failed to follow user", err)
	}

	s.logger.Info("User followed",
		zap.String("follower_id", followerID),
		zap.String("following_id", followingID),
	)

	if s.notificationService != nil {
		go func() {
			ctxDetach := context.WithoutCancel(ctx)
			actor, err := s.userRepo.GetProfileByUserID(ctxDetach, followerID)
			if err != nil {
				s.logger.Warn("Failed to get actor for follow notification", zap.Error(err))
				return
			}
			actorName := actor.FullName()
			title := actorName + " started following you"
			msg := title
			data := map[string]interface{}{
				"actor_id":     followerID,
				"actor_name":   actorName,
				"actor_avatar": actor.Avatar,
			}
			s.notificationService.CreateNotification(ctxDetach, &models.CreateNotificationRequest{
				UserID:  followingID,
				Type:    models.NotificationTypeFollow,
				Title:   &title,
				Message: &msg,
				Data:    data,
			})
		}()
	}

	return nil
}

// UnfollowUser unfollows a user
func (s *RelationshipsService) UnfollowUser(ctx context.Context, followerID, followingID string) error {
	// Validate that users are not the same
	if followerID == followingID {
		return utils.NewBadRequestError("Cannot unfollow yourself", nil)
	}

	// Unfollow (idempotent - no error if not following)
	if err := s.relationshipsRepo.UnfollowUser(ctx, followerID, followingID); err != nil {
		s.logger.Error("Failed to unfollow user",
			zap.String("follower_id", followerID),
			zap.String("following_id", followingID),
			zap.Error(err),
		)
		return utils.NewInternalError("Failed to unfollow user", err)
	}

	s.logger.Info("User unfollowed",
		zap.String("follower_id", followerID),
		zap.String("following_id", followingID),
	)

	return nil
}

// GetFollowers gets a user's followers with profile information
func (s *RelationshipsService) GetFollowers(ctx context.Context, userID string, viewerID *string, limit, offset int) ([]*models.FollowerResponse, error) {
	// Get followers
	follows, err := s.relationshipsRepo.GetFollowers(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get followers", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get followers", err)
	}

	// Get profile information for each follower
	var followers []*models.FollowerResponse
	for _, follow := range follows {
		profile, err := s.userRepo.GetProfileByUserID(ctx, follow.FollowerID)
		if err != nil {
			s.logger.Warn("Failed to get follower profile",
				zap.String("follower_id", follow.FollowerID),
				zap.Error(err),
			)
			continue
		}

		follower := &models.FollowerResponse{
			UserID:    follow.FollowerID,
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			FullName:  profile.FullName(),
			Avatar:    profile.Avatar,
			Province:  profile.Province,
			CreatedAt: follow.CreatedAt,
		}

		// Get relationship status if viewer is provided
		if viewerID != nil && *viewerID != "" {
			status, err := s.relationshipsRepo.GetRelationshipStatus(ctx, *viewerID, follow.FollowerID)
			if err == nil {
				follower.IsFollowing = status.IsFollowing
				follower.IsFollowedBy = status.IsFollowedBy
			}
		}

		followers = append(followers, follower)
	}

	return followers, nil
}

// GetFollowing gets users that a user is following with profile information
func (s *RelationshipsService) GetFollowing(ctx context.Context, userID string, viewerID *string, limit, offset int) ([]*models.FollowingResponse, error) {
	// Get following
	follows, err := s.relationshipsRepo.GetFollowing(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get following", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get following", err)
	}

	// Get profile information for each user being followed
	var following []*models.FollowingResponse
	for _, follow := range follows {
		profile, err := s.userRepo.GetProfileByUserID(ctx, follow.FollowingID)
		if err != nil {
			s.logger.Warn("Failed to get following profile",
				zap.String("following_id", follow.FollowingID),
				zap.Error(err),
			)
			continue
		}

		followingUser := &models.FollowingResponse{
			UserID:    follow.FollowingID,
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			FullName:  profile.FullName(),
			Avatar:    profile.Avatar,
			Province:  profile.Province,
			CreatedAt: follow.CreatedAt,
		}

		// Get relationship status if viewer is provided
		if viewerID != nil && *viewerID != "" {
			status, err := s.relationshipsRepo.GetRelationshipStatus(ctx, *viewerID, follow.FollowingID)
			if err == nil {
				followingUser.IsFollowing = status.IsFollowing
				followingUser.IsFollowedBy = status.IsFollowedBy
			}
		}

		following = append(following, followingUser)
	}

	return following, nil
}

// BlockUser blocks a user
func (s *RelationshipsService) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	// Validate that users are not the same
	if blockerID == blockedID {
		return utils.NewBadRequestError("Cannot block yourself", nil)
	}

	// Check if target user exists
	_, err := s.userRepo.GetByID(ctx, blockedID)
	if err != nil {
		s.logger.Warn("Target user not found", zap.String("user_id", blockedID), zap.Error(err))
		return utils.NewNotFoundError("User not found", err)
	}

	// Block user (this will also remove any existing follow relationships via database triggers or manually)
	if err := s.relationshipsRepo.BlockUser(ctx, blockerID, blockedID); err != nil {
		s.logger.Error("Failed to block user",
			zap.String("blocker_id", blockerID),
			zap.String("blocked_id", blockedID),
			zap.Error(err),
		)
		return utils.NewInternalError("Failed to block user", err)
	}

	// Remove any existing follow relationships
	_ = s.relationshipsRepo.UnfollowUser(ctx, blockerID, blockedID)
	_ = s.relationshipsRepo.UnfollowUser(ctx, blockedID, blockerID)

	s.logger.Info("User blocked",
		zap.String("blocker_id", blockerID),
		zap.String("blocked_id", blockedID),
	)

	return nil
}

// UnblockUser unblocks a user
func (s *RelationshipsService) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	// Validate that users are not the same
	if blockerID == blockedID {
		return utils.NewBadRequestError("Cannot unblock yourself", nil)
	}

	// Unblock (idempotent - no error if not blocked)
	if err := s.relationshipsRepo.UnblockUser(ctx, blockerID, blockedID); err != nil {
		s.logger.Error("Failed to unblock user",
			zap.String("blocker_id", blockerID),
			zap.String("blocked_id", blockedID),
			zap.Error(err),
		)
		return utils.NewInternalError("Failed to unblock user", err)
	}

	s.logger.Info("User unblocked",
		zap.String("blocker_id", blockerID),
		zap.String("blocked_id", blockedID),
	)

	return nil
}

// GetBlockedUsers gets users that a user has blocked with profile information
func (s *RelationshipsService) GetBlockedUsers(ctx context.Context, blockerID string, limit, offset int) ([]*models.BlockedUserResponse, error) {
	// Get blocked users
	blocks, err := s.relationshipsRepo.GetBlockedUsers(ctx, blockerID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get blocked users", zap.String("blocker_id", blockerID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get blocked users", err)
	}

	// Get profile information for each blocked user
	var blockedUsers []*models.BlockedUserResponse
	for _, block := range blocks {
		profile, err := s.userRepo.GetProfileByUserID(ctx, block.BlockedID)
		if err != nil {
			s.logger.Warn("Failed to get blocked user profile",
				zap.String("blocked_id", block.BlockedID),
				zap.Error(err),
			)
			continue
		}

		blockedUser := &models.BlockedUserResponse{
			UserID:    block.BlockedID,
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			FullName:  profile.FullName(),
			Avatar:    profile.Avatar,
			Province:  profile.Province,
			CreatedAt: block.CreatedAt,
		}

		blockedUsers = append(blockedUsers, blockedUser)
	}

	return blockedUsers, nil
}

// GetRelationshipStatus gets the relationship status between viewer and target user
func (s *RelationshipsService) GetRelationshipStatus(ctx context.Context, viewerID, targetUserID string) (*models.RelationshipStatus, error) {
	status, err := s.relationshipsRepo.GetRelationshipStatus(ctx, viewerID, targetUserID)
	if err != nil {
		s.logger.Error("Failed to get relationship status",
			zap.String("viewer_id", viewerID),
			zap.String("target_user_id", targetUserID),
			zap.Error(err),
		)
		return nil, utils.NewInternalError("Failed to get relationship status", err)
	}

	return status, nil
}
