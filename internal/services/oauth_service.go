package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// OAuthService handles OAuth authentication with third-party providers
type OAuthService struct {
	cfg      *config.Config
	userRepo repositories.UserRepository
	logger   *zap.Logger
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(
	cfg *config.Config,
	userRepo repositories.UserRepository,
	logger *zap.Logger,
) *OAuthService {
	return &OAuthService{
		cfg:      cfg,
		userRepo: userRepo,
		logger:   logger,
	}
}

// GoogleUserInfo represents user info from Google
type GoogleUserInfo struct {
	ID            string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// AppleUserInfo represents user info from Apple
type AppleUserInfo struct {
	Sub            string `json:"sub"`
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	IsPrivateEmail bool   `json:"is_private_email"`
}

// FacebookUserInfo represents user info from Facebook
type FacebookUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

// OAuthUserInfo is a normalized struct for OAuth user information
type OAuthUserInfo struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
	FirstName      string
	LastName       string
	Picture        string
	Provider       string
}

// VerifyGoogleToken verifies a Google ID token and returns user info
func (s *OAuthService) VerifyGoogleToken(ctx context.Context, idToken string) (*OAuthUserInfo, error) {
	// Verify token with Google's tokeninfo endpoint
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Failed to verify Google token", zap.Error(err))
		return nil, utils.NewUnauthorizedError("Failed to verify Google token", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("Google token verification failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, utils.NewUnauthorizedError("Invalid Google token", nil)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		s.logger.Error("Failed to decode Google user info", zap.Error(err))
		return nil, utils.NewInternalError("Failed to decode user info", err)
	}

	// Verify the token is for our app
	// Note: In production, you should verify the 'aud' claim matches your client ID
	// This requires parsing the JWT token properly
	// For now, we rely on Google's tokeninfo endpoint validation

	s.logger.Info("Google token verified",
		zap.String("user_id", userInfo.ID),
		zap.String("email", userInfo.Email),
	)

	return &OAuthUserInfo{
		ProviderUserID: userInfo.ID,
		Email:          userInfo.Email,
		EmailVerified:  userInfo.EmailVerified,
		FirstName:      userInfo.GivenName,
		LastName:       userInfo.FamilyName,
		Picture:        userInfo.Picture,
		Provider:       "google",
	}, nil
}

// VerifyAppleToken verifies an Apple ID token and returns user info
func (s *OAuthService) VerifyAppleToken(ctx context.Context, idToken string) (*OAuthUserInfo, error) {
	// Apple Sign In requires validating the JWT token with Apple's public keys
	// This is more complex and typically requires a JWT library
	// For now, we'll return a not implemented error
	// In production, you would:
	// 1. Fetch Apple's public keys from https://appleid.apple.com/auth/keys
	// 2. Validate the JWT signature
	// 3. Verify claims (iss, aud, exp, etc.)
	// 4. Extract user info from the token

	s.logger.Warn("Apple Sign In not yet fully implemented")
	return nil, utils.NewNotImplementedError("Apple Sign In not yet fully implemented", nil)
}

// VerifyFacebookToken verifies a Facebook access token and returns user info
func (s *OAuthService) VerifyFacebookToken(ctx context.Context, accessToken string) (*OAuthUserInfo, error) {
	// Get user info from Facebook Graph API
	url := fmt.Sprintf("https://graph.facebook.com/me?fields=id,email,name,picture&access_token=%s", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Failed to verify Facebook token", zap.Error(err))
		return nil, utils.NewUnauthorizedError("Failed to verify Facebook token", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("Facebook token verification failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, utils.NewUnauthorizedError("Invalid Facebook token", nil)
	}

	var userInfo FacebookUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		s.logger.Error("Failed to decode Facebook user info", zap.Error(err))
		return nil, utils.NewInternalError("Failed to decode user info", err)
	}

	// Split name into first and last name
	nameParts := strings.SplitN(userInfo.Name, " ", 2)
	firstName := ""
	lastName := ""
	if len(nameParts) > 0 {
		firstName = nameParts[0]
	}
	if len(nameParts) > 1 {
		lastName = nameParts[1]
	}

	s.logger.Info("Facebook token verified",
		zap.String("user_id", userInfo.ID),
		zap.String("email", userInfo.Email),
	)

	return &OAuthUserInfo{
		ProviderUserID: userInfo.ID,
		Email:          userInfo.Email,
		EmailVerified:  true, // Facebook emails are considered verified
		FirstName:      firstName,
		LastName:       lastName,
		Picture:        userInfo.Picture.Data.URL,
		Provider:       "facebook",
	}, nil
}

// AuthenticateWithOAuth authenticates or registers a user with OAuth
func (s *OAuthService) AuthenticateWithOAuth(ctx context.Context, oauthInfo *OAuthUserInfo) (*models.User, *models.Profile, bool, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(oauthInfo.Email))

	if email == "" {
		return nil, nil, false, utils.NewBadRequestError("Email is required from OAuth provider", nil)
	}

	// Check if user exists
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		// User exists - check if it's an OAuth account or needs linking
		if existingUser.OAuthProvider != nil && *existingUser.OAuthProvider != oauthInfo.Provider {
			// User exists with different OAuth provider
			return nil, nil, false, utils.NewConflictError(
				fmt.Sprintf("This email is already registered with %s", *existingUser.OAuthProvider),
				nil,
			)
		}

		if existingUser.OAuthProvider == nil {
			// User exists with password - could implement account linking here
			return nil, nil, false, utils.NewConflictError(
				"This email is already registered with a password. Please login with your password.",
				nil,
			)
		}

		// Update last login
		if err := s.userRepo.UpdateLastLogin(ctx, existingUser.ID); err != nil {
			s.logger.Error("Failed to update last login", zap.Error(err))
			// Continue anyway
		}

		// Get profile
		profile, err := s.userRepo.GetProfileByUserID(ctx, existingUser.ID)
		if err != nil {
			s.logger.Error("Failed to get profile", zap.Error(err))
			return nil, nil, false, utils.NewInternalError("Failed to get profile", err)
		}

		s.logger.Info("OAuth user logged in",
			zap.String("user_id", existingUser.ID),
			zap.String("provider", oauthInfo.Provider),
		)

		return existingUser, profile, false, nil // false = not a new user
	}

	// User doesn't exist - create new account
	userID := uuid.New().String()
	now := time.Now()
	providerID := oauthInfo.ProviderUserID

	user := &models.User{
		ID:                  userID,
		Email:               email,
		EmailVerified:       oauthInfo.EmailVerified,
		PhoneVerified:       false,
		MFAEnabled:          false,
		OAuthProvider:       &oauthInfo.Provider,
		OAuthProviderID:     &providerID,
		FailedLoginAttempts: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create OAuth user", zap.Error(err))
		return nil, nil, false, utils.NewInternalError("Failed to create user", err)
	}

	// Create profile with random avatar color
	avatarColor := models.RandomAvatarColor()
	profile := &models.Profile{
		ID:          userID,
		FirstName:   &oauthInfo.FirstName,
		LastName:    &oauthInfo.LastName,
		AvatarColor: &avatarColor,
		IsComplete:  false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Set avatar from OAuth provider picture
	if oauthInfo.Picture != "" {
		profile.Avatar = &models.Photo{
			URL:      oauthInfo.Picture,
			Name:     "avatar",
			MimeType: "image/jpeg",
		}
	}

	if err := s.userRepo.CreateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to create profile", zap.Error(err))
		return nil, nil, false, utils.NewInternalError("Failed to create profile", err)
	}

	s.logger.Info("New OAuth user registered",
		zap.String("user_id", userID),
		zap.String("email", email),
		zap.String("provider", oauthInfo.Provider),
	)

	return user, profile, true, nil // true = new user
}
