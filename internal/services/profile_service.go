package services

import (
	"context"
	"strings"
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
	commentRepo       repositories.CommentRepository
	relationshipsRepo repositories.RelationshipsRepository
	emailService      *EmailService
	tokenStorage      *TokenStorageService
	jwtService        *JWTService
	logger            *zap.Logger
}

// NewProfileService creates a new profile service
func NewProfileService(
	userRepo repositories.UserRepository,
	postRepo repositories.PostRepository,
	commentRepo repositories.CommentRepository,
	relationshipsRepo repositories.RelationshipsRepository,
	emailService *EmailService,
	tokenStorage *TokenStorageService,
	jwtService *JWTService,
	logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		userRepo:          userRepo,
		postRepo:          postRepo,
		commentRepo:       commentRepo,
		relationshipsRepo: relationshipsRepo,
		emailService:      emailService,
		tokenStorage:      tokenStorage,
		jwtService:        jwtService,
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

	// Compute profile-completion percentage so the mobile client can render
	// a progress bar and prompt the user to fill in missing fields. Cheap,
	// no DB call.
	pct, missing := profileCompletion(profile)
	response.CompletionPercent = pct
	response.MissingFields = missing

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

// UpdateProfile updates a user's profile.
// When IsComplete transitions from false → true and the user's email is not yet
// verified, an OTP verification email is sent so users confirm their email only
// after they have a real profile (name + location).
func (s *ProfileService) UpdateProfile(ctx context.Context, userID string, req *models.UpdateProfileRequest) (*models.FullProfileResponse, error) {
	// Get current profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get profile", err)
	}
	wasComplete := profile.IsComplete

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
	} else if req.Latitude != nil && req.Longitude != nil { //nolint:staticcheck
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude}, //nolint:staticcheck
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

	// Send OTP verification email when profile becomes complete for the first time.
	if !wasComplete && profile.IsComplete && s.emailService != nil && s.tokenStorage != nil && s.jwtService != nil {
		user, userErr := s.userRepo.GetByID(ctx, userID)
		if userErr == nil && !user.EmailVerified {
			go func() {
				bgCtx := context.Background()
				code, codeErr := s.jwtService.GenerateVerificationCode()
				if codeErr != nil {
					s.logger.Warn("Failed to generate verification code after profile complete", zap.Error(codeErr))
					return
				}
				const ttl = 24 * time.Hour
				if storeErr := s.tokenStorage.StoreVerificationToken(bgCtx, userID, code, ttl); storeErr != nil {
					s.logger.Warn("Failed to store verification token after profile complete", zap.Error(storeErr))
					return
				}
				name := strings.TrimSpace(func() string {
					if profile.FirstName != nil && profile.LastName != nil {
						return *profile.FirstName + " " + *profile.LastName
					}
					if profile.FirstName != nil {
						return *profile.FirstName
					}
					return user.Email
				}())
				if sendErr := s.emailService.SendVerificationEmail(user.Email, name, code); sendErr != nil {
					s.logger.Warn("Failed to send verification email after profile complete",
						zap.String("user_id", userID), zap.Error(sendErr))
				}
			}()
		}
	}

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
	// A profile is complete when the user has set their location.
	// First and last name are populated automatically for social (OAuth) users
	// and are not required as a completion gate.
	return profile.Location != nil && profile.Location.Valid
}

// profileCompletion returns (percent, missing_field_keys) for a profile.
// Score is the proportion of "high-value" fields populated. The list of
// missing keys is used by the mobile client to deep-link the user to the
// relevant edit section.
//
// Field weights are equal — the rule of thumb is "what would a human-eyed
// profile actually have?". Email/phone live on the user record, not the
// profile, so they're excluded.
func profileCompletion(p *models.Profile) (int, []string) {
	if p == nil {
		return 0, nil
	}
	checks := []struct {
		key     string
		present bool
	}{
		{"first_name", p.FirstName != nil && *p.FirstName != ""},
		{"last_name", p.LastName != nil && *p.LastName != ""},
		{"avatar", p.Avatar != nil && p.Avatar.URL != ""},
		{"about", p.About != nil && *p.About != ""},
		{"gender", p.Gender != nil && *p.Gender != ""},
		{"province", p.Province != nil && *p.Province != ""},
		{"district", p.District != nil && *p.District != ""},
		{"location", p.Location != nil && p.Location.Valid},
	}

	filled := 0
	missing := make([]string, 0, len(checks))
	for _, c := range checks {
		if c.present {
			filled++
		} else {
			missing = append(missing, c.key)
		}
	}
	percent := (filled * 100) / len(checks)
	return percent, missing
}

// Helper function
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ExportUserData collects the user's owned data for the GDPR data-export
// endpoint. Lists are capped (see exportListLimit) to keep the response
// inside a single HTTP cycle; total counts are reported separately so the
// client can warn when truncation happened.
//
// Heavy work — gated upstream by a 1-per-day rate limit middleware.
func (s *ProfileService) ExportUserData(ctx context.Context, userID string) (*models.UserDataExport, error) {
	const exportListLimit = 5000

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, utils.NewNotFoundError("User not found", err)
	}
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, utils.NewInternalError("Failed to get profile", err)
	}
	profileResp := models.ToFullProfileResponse(user, profile)
	pct, missing := profileCompletion(profile)
	profileResp.CompletionPercent = pct
	profileResp.MissingFields = missing

	posts, err := s.postRepo.GetUserPosts(ctx, userID, exportListLimit, 0)
	if err != nil {
		s.logger.Error("export: posts", zap.String("user_id", userID), zap.Error(err))
		posts = nil
	}
	postsCount, _ := s.postRepo.CountPostsByUser(ctx, userID)

	comments, err := s.commentRepo.GetByUserID(ctx, userID, exportListLimit, 0)
	if err != nil {
		s.logger.Error("export: comments", zap.String("user_id", userID), zap.Error(err))
		comments = nil
	}

	followers, _ := s.relationshipsRepo.GetFollowers(ctx, userID, exportListLimit, 0)
	following, _ := s.relationshipsRepo.GetFollowing(ctx, userID, exportListLimit, 0)
	followersTotal, _ := s.relationshipsRepo.GetFollowersCount(ctx, userID)
	followingTotal, _ := s.relationshipsRepo.GetFollowingCount(ctx, userID)

	followerIDs := make([]string, 0, len(followers))
	for _, f := range followers {
		followerIDs = append(followerIDs, f.FollowerID)
	}
	followingIDs := make([]string, 0, len(following))
	for _, f := range following {
		followingIDs = append(followingIDs, f.FollowingID)
	}

	blocked, _ := s.relationshipsRepo.GetBlockedUsers(ctx, userID, exportListLimit, 0)
	blockedIDs := make([]string, 0, len(blocked))
	for _, b := range blocked {
		blockedIDs = append(blockedIDs, b.BlockedID)
	}

	bookmarks, _ := s.postRepo.GetUserBookmarks(ctx, userID, exportListLimit, 0)
	bookmarkIDs := make([]string, 0, len(bookmarks))
	for _, p := range bookmarks {
		bookmarkIDs = append(bookmarkIDs, p.ID)
	}

	return &models.UserDataExport{
		GeneratedAt:     time.Now().UTC(),
		Format:          "json",
		Version:         "1",
		Profile:         profileResp,
		Posts:           posts,
		Comments:        comments,
		FollowerIDs:     followerIDs,
		FollowingIDs:    followingIDs,
		BlockedIDs:      blockedIDs,
		BookmarkPostIDs: bookmarkIDs,
		Counts: models.ExportCounts{
			Posts:     postsCount,
			Comments:  len(comments),
			Followers: followersTotal,
			Following: followingTotal,
			Blocked:   len(blockedIDs),
			Bookmarks: len(bookmarkIDs),
		},
	}, nil
}
