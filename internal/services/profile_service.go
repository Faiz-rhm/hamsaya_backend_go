package services

import (
	"context"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// ProfileService handles user profile operations
type ProfileService struct {
	userRepo          repositories.UserRepository
	postRepo          repositories.PostRepository
	relationshipsRepo repositories.RelationshipsRepository
	logger            *zap.Logger
}

// NewProfileService creates a new profile service
func NewProfileService(
	userRepo repositories.UserRepository,
	postRepo repositories.PostRepository,
	relationshipsRepo repositories.RelationshipsRepository,
	logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		userRepo:          userRepo,
		postRepo:          postRepo,
		relationshipsRepo: relationshipsRepo,
		logger:            logger,
	}
}

// GetProfile gets a user's profile by user ID
func (s *ProfileService) GetProfile(ctx context.Context, userID string, viewerID *string) (*models.FullProfileResponse, error) {
	// Get user (active only)
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		// User not found as active - check if soft-deleted (deactivated)
		deletedUser, delErr := s.userRepo.GetByIDIncludingDeleted(ctx, userID)
		if delErr != nil || deletedUser == nil || deletedUser.DeletedAt == nil {
			s.logger.Warn("User not found", zap.String("user_id", userID), zap.Error(err))
			return nil, utils.NewNotFoundError("User not found", err)
		}
		// Return minimal deactivated profile
		postsCount, _ := s.postRepo.CountPostsByUser(ctx, userID)
		response := models.ToDeactivatedProfileResponse(userID, postsCount)
		s.logger.Info("Deactivated profile retrieved", zap.String("user_id", userID))
		return response, nil
	}

	// Get profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get profile", err)
	}

	// Convert to response
	response := models.ToFullProfileResponse(user, profile)

	// Populate stats (followers, following, posts count)
	followersCount, err := s.relationshipsRepo.GetFollowersCount(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to get followers count", zap.String("user_id", userID), zap.Error(err))
		followersCount = 0
	}
	response.FollowersCount = followersCount

	followingCount, err := s.relationshipsRepo.GetFollowingCount(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to get following count", zap.String("user_id", userID), zap.Error(err))
		followingCount = 0
	}
	response.FollowingCount = followingCount

	postsCount, err := s.postRepo.CountPostsByUser(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to get posts count", zap.String("user_id", userID), zap.Error(err))
		postsCount = 0
	}
	response.PostsCount = postsCount

	// Populate relationship status (is_blocked, has_blocked_me) if viewer is authenticated
	if viewerID != nil && *viewerID != "" && *viewerID != userID {
		status, err := s.relationshipsRepo.GetRelationshipStatus(ctx, *viewerID, userID)
		if err == nil {
			response.IsBlocked = status.IsBlocked    // viewer blocks target (I am blocking them)
			response.HasBlockedMe = status.HasBlockedMe // target blocks viewer (they blocked me)
		}
	}

	s.logger.Info("Profile retrieved",
		zap.String("user_id", userID),
		zap.String("viewer_id", stringOrEmpty(viewerID)),
		zap.Int("followers", followersCount),
		zap.Int("following", followingCount),
		zap.Int("posts", postsCount),
	)

	return response, nil
}

// UpdateProfile updates a user's profile
func (s *ProfileService) UpdateProfile(ctx context.Context, userID string, req *models.UpdateProfileRequest) (*models.FullProfileResponse, error) {
	// Get current profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get profile", err)
	}

	// Update fields if provided
	if req.FirstName != nil {
		profile.FirstName = req.FirstName
	}
	if req.LastName != nil {
		profile.LastName = req.LastName
	}
	if req.About != nil {
		profile.About = req.About
	}
	if req.Gender != nil {
		profile.Gender = req.Gender
	}
	if req.DOB != nil {
		profile.DOB = req.DOB
	}
	if req.Website != nil {
		profile.Website = req.Website
	}
	if req.Country != nil {
		profile.Country = req.Country
	}
	if req.Province != nil {
		profile.Province = req.Province
	}
	if req.District != nil {
		profile.District = req.District
	}
	if req.Neighborhood != nil {
		profile.Neighborhood = req.Neighborhood
	}
	if req.AvatarColor != nil {
		profile.AvatarColor = req.AvatarColor
	}

	// Handle location update (Latitude/Longitude -> pgtype.Point)
	// Support both nested location object and flat latitude/longitude fields
	if req.Location != nil {
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: req.Location.Longitude, Y: req.Location.Latitude},
			Valid: true,
		}
	} else if req.Latitude != nil && req.Longitude != nil {
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude},
			Valid: true,
		}
	}

	// Update IsComplete field
	// If explicitly provided in request, use that value
	// Otherwise, automatically calculate based on profile fields
	if req.IsComplete != nil {
		profile.IsComplete = *req.IsComplete
	} else {
		profile.IsComplete = s.isProfileComplete(profile)
	}
	profile.UpdatedAt = time.Now()

	// Update profile
	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to update profile", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to update profile", err)
	}

	s.logger.Info("Profile updated",
		zap.String("user_id", userID),
		zap.Bool("is_complete", profile.IsComplete),
	)

	// Return updated profile
	return s.GetProfile(ctx, userID, nil)
}

// UpdateAvatar updates a user's avatar
func (s *ProfileService) UpdateAvatar(ctx context.Context, userID string, photo *models.Photo) error {
	// Get current profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to get profile", err)
	}

	// Update avatar
	profile.Avatar = photo
	profile.UpdatedAt = time.Now()

	// Update profile
	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to update avatar", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to update avatar", err)
	}

	s.logger.Info("Avatar updated", zap.String("user_id", userID))
	return nil
}

// DeleteAvatar deletes a user's avatar
func (s *ProfileService) DeleteAvatar(ctx context.Context, userID string) error {
	// Get current profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to get profile", err)
	}

	// Delete avatar
	profile.Avatar = nil
	profile.UpdatedAt = time.Now()

	// Update profile
	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to delete avatar", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to delete avatar", err)
	}

	s.logger.Info("Avatar deleted", zap.String("user_id", userID))
	return nil
}

// UpdateCover updates a user's cover photo
func (s *ProfileService) UpdateCover(ctx context.Context, userID string, photo *models.Photo) error {
	// Get current profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to get profile", err)
	}

	// Update cover
	profile.Cover = photo
	profile.UpdatedAt = time.Now()

	// Update profile
	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to update cover", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to update cover", err)
	}

	s.logger.Info("Cover updated", zap.String("user_id", userID))
	return nil
}

// DeleteCover deletes a user's cover photo
// DeactivateAccount soft-deletes the user and revokes all sessions
func (s *ProfileService) DeactivateAccount(ctx context.Context, userID string) error {
	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		s.logger.Error("Failed to soft delete user", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to deactivate account", err)
	}
	if err := s.userRepo.RevokeAllUserSessions(ctx, userID); err != nil {
		s.logger.Warn("Failed to revoke sessions after deactivation", zap.String("user_id", userID), zap.Error(err))
	}
	s.logger.Info("Account deactivated", zap.String("user_id", userID))
	return nil
}

func (s *ProfileService) DeleteCover(ctx context.Context, userID string) error {
	// Get current profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to get profile", err)
	}

	// Delete cover
	profile.Cover = nil
	profile.UpdatedAt = time.Now()

	// Update profile
	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to delete cover", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to delete cover", err)
	}

	s.logger.Info("Cover deleted", zap.String("user_id", userID))
	return nil
}

// isProfileComplete checks if a profile has all required fields
func (s *ProfileService) isProfileComplete(profile *models.Profile) bool {
	// A profile is complete if it has:
	// - First name and last name
	// - Location (latitude and longitude)

	if profile.FirstName == nil || *profile.FirstName == "" {
		return false
	}
	if profile.LastName == nil || *profile.LastName == "" {
		return false
	}
	if profile.Location == nil || !profile.Location.Valid {
		return false
	}

	return true
}

// Helper function
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
