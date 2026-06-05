package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// OAuthService handles OAuth authentication with third-party providers
type OAuthService struct {
	cfg       *config.Config
	userRepo  repositories.UserRepository
	logger    *zap.Logger
	appleKeys *appleKeyCache
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(
	cfg *config.Config,
	userRepo repositories.UserRepository,
	logger *zap.Logger,
) *OAuthService {
	return &OAuthService{
		cfg:       cfg,
		userRepo:  userRepo,
		logger:    logger,
		appleKeys: newAppleKeyCache(),
	}
}

// GoogleUserInfo represents user info from Google.
// email_verified is returned as a string ("true"/"false") by Google's tokeninfo endpoint.
type GoogleUserInfo struct {
	ID            string `json:"sub"`
	Aud           string `json:"aud"` // OAuth2 client ID this token was minted for — must match our config.
	Iss           string `json:"iss"` // Issuer — must be accounts.google.com or https://accounts.google.com.
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
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
	IsPrivateEmail bool
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
	defer func() { _ = resp.Body.Close() }()

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

	// Verify the token is for our app. tokeninfo proves Google signed the
	// token, NOT that it was minted for THIS client. Without checking `aud`,
	// any third-party Google OAuth client's id_token would authenticate on
	// our backend. Fail-closed: if APPLE_CLIENT_ID-equivalent (GOOGLE_CLIENT_ID)
	// is unset we refuse to verify rather than accept everything.
	expectedAud := s.cfg.OAuth.Google.ClientID
	if expectedAud == "" {
		s.logger.Warn("GOOGLE_CLIENT_ID not configured; refusing Google token verification")
		return nil, utils.NewInternalError("Google OAuth not configured (GOOGLE_CLIENT_ID missing)", nil)
	}
	if userInfo.Aud != expectedAud {
		s.logger.Warn("Google token aud mismatch",
			zap.String("got_aud", userInfo.Aud),
			zap.String("expected_aud", expectedAud),
		)
		return nil, utils.NewUnauthorizedError("Google token not minted for this client", nil)
	}
	if userInfo.Iss != "accounts.google.com" && userInfo.Iss != "https://accounts.google.com" {
		s.logger.Warn("Google token iss mismatch", zap.String("iss", userInfo.Iss))
		return nil, utils.NewUnauthorizedError("Invalid Google token issuer", nil)
	}

	s.logger.Info("Google token verified",
		zap.String("user_id", userInfo.ID),
	)

	return &OAuthUserInfo{
		ProviderUserID: userInfo.ID,
		Email:          userInfo.Email,
		EmailVerified:  userInfo.EmailVerified == "true",
		FirstName:      userInfo.GivenName,
		LastName:       userInfo.FamilyName,
		Picture:        userInfo.Picture,
		Provider:       "google",
	}, nil
}

// VerifyAppleToken verifies an Apple ID token and returns user info.
//
// Steps:
//  1. Parse the JWT header to extract the `kid`.
//  2. Fetch Apple's public keys (cached) and look up the matching key.
//  3. Verify the RS256 signature using that key.
//  4. Validate `iss`, `aud`, and `exp` claims.
//  5. Extract `sub`, `email`, `email_verified`.
func (s *OAuthService) VerifyAppleToken(ctx context.Context, idToken string) (*OAuthUserInfo, error) {
	clientID := s.cfg.OAuth.Apple.ClientID
	if clientID == "" {
		return nil, utils.NewInternalError("Apple OAuth not configured (APPLE_CLIENT_ID missing)", nil)
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://appleid.apple.com"),
		jwt.WithAudience(clientID),
		jwt.WithExpirationRequired(),
	)

	token, err := parser.Parse(idToken, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("apple token missing kid header")
		}
		return s.appleKeys.publicKey(ctx, kid)
	})
	if err != nil {
		// Decode-only parse so we can log the actual `aud`/`iss` claims when
		// strict verification fails — easier than hex-dumping the JWT.
		debugParser := jwt.NewParser(jwt.WithoutClaimsValidation())
		if dbg, _, dbgErr := debugParser.ParseUnverified(idToken, jwt.MapClaims{}); dbgErr == nil {
			if dbgClaims, ok := dbg.Claims.(jwt.MapClaims); ok {
				s.logger.Warn("Apple token verification failed",
					zap.Error(err),
					zap.Any("aud", dbgClaims["aud"]),
					zap.Any("iss", dbgClaims["iss"]),
					zap.Any("sub", dbgClaims["sub"]),
					zap.String("expected_aud", clientID),
				)
			} else {
				s.logger.Warn("Apple token verification failed", zap.Error(err))
			}
		} else {
			s.logger.Warn("Apple token verification failed", zap.Error(err))
		}
		return nil, utils.NewUnauthorizedError("Invalid Apple identity token", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, utils.NewUnauthorizedError("Invalid Apple identity token claims", nil)
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, utils.NewUnauthorizedError("Apple token missing sub claim", nil)
	}
	email, _ := claims["email"].(string)
	// `email_verified` and `is_private_email` are sometimes a bool, sometimes the
	// string "true"/"false" depending on Apple's response.
	emailVerified := false
	switch v := claims["email_verified"].(type) {
	case bool:
		emailVerified = v
	case string:
		emailVerified = v == "true"
	}
	isPrivateEmail := false
	switch v := claims["is_private_email"].(type) {
	case bool:
		isPrivateEmail = v
	case string:
		isPrivateEmail = v == "true"
	}

	s.logger.Info("Apple token verified",
		zap.String("sub", sub),
		zap.String("email", email),
		zap.Bool("is_private_email", isPrivateEmail),
	)

	return &OAuthUserInfo{
		ProviderUserID: sub,
		Email:          email,
		EmailVerified:  emailVerified,
		IsPrivateEmail: isPrivateEmail,
		Provider:       "apple",
	}, nil
}

// VerifyFacebookToken verifies a Facebook access token and returns user info.
// First calls /debug_token (app-token signed) to confirm the token was minted
// for THIS app — without this check any Facebook OAuth client could mint a
// token that authenticates on our backend. Then calls /me for user details.
func (s *OAuthService) VerifyFacebookToken(ctx context.Context, accessToken string) (*OAuthUserInfo, error) {
	expectedAppID := s.cfg.OAuth.Facebook.AppID
	expectedAppSecret := s.cfg.OAuth.Facebook.AppSecret
	if expectedAppID == "" || expectedAppSecret == "" {
		s.logger.Warn("Facebook OAuth not configured; refusing token verification")
		return nil, utils.NewInternalError("Facebook OAuth not configured (FACEBOOK_APP_ID/SECRET missing)", nil)
	}

	// Step 1 — verify token belongs to our app via debug_token.
	debugURL := fmt.Sprintf(
		"https://graph.facebook.com/debug_token?input_token=%s&access_token=%s|%s",
		accessToken, expectedAppID, expectedAppSecret,
	)
	client := &http.Client{Timeout: 10 * time.Second}
	dbgReq, err := http.NewRequestWithContext(ctx, "GET", debugURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug_token request: %w", err)
	}
	dbgResp, err := client.Do(dbgReq)
	if err != nil {
		s.logger.Error("Failed to call Facebook debug_token", zap.Error(err))
		return nil, utils.NewUnauthorizedError("Failed to verify Facebook token", err)
	}
	defer func() { _ = dbgResp.Body.Close() }()
	if dbgResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dbgResp.Body)
		s.logger.Warn("Facebook debug_token failed",
			zap.Int("status", dbgResp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, utils.NewUnauthorizedError("Invalid Facebook token", nil)
	}
	var dbg struct {
		Data struct {
			AppID   string `json:"app_id"`
			IsValid bool   `json:"is_valid"`
			UserID  string `json:"user_id"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		} `json:"data"`
	}
	if err := json.NewDecoder(dbgResp.Body).Decode(&dbg); err != nil {
		return nil, utils.NewInternalError("Failed to decode Facebook debug_token", err)
	}
	if !dbg.Data.IsValid {
		s.logger.Warn("Facebook token is not valid")
		return nil, utils.NewUnauthorizedError("Invalid Facebook token", nil)
	}
	if dbg.Data.AppID != expectedAppID {
		s.logger.Warn("Facebook token app_id mismatch",
			zap.String("got_app_id", dbg.Data.AppID),
			zap.String("expected_app_id", expectedAppID),
		)
		return nil, utils.NewUnauthorizedError("Facebook token not minted for this app", nil)
	}

	// Step 2 — fetch user details (now that we trust the token belongs to us).
	url := fmt.Sprintf("https://graph.facebook.com/me?fields=id,email,name,picture&access_token=%s", accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Failed to verify Facebook token", zap.Error(err))
		return nil, utils.NewUnauthorizedError("Failed to verify Facebook token", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("Facebook /me failed",
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
	if userInfo.ID != dbg.Data.UserID {
		// /me must return the same user as debug_token reported.
		s.logger.Warn("Facebook user_id mismatch between debug_token and /me",
			zap.String("debug_user", dbg.Data.UserID),
			zap.String("me_user", userInfo.ID),
		)
		return nil, utils.NewUnauthorizedError("Inconsistent Facebook token", nil)
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

// syntheticOAuthEmail builds a stable, non-deliverable placeholder email for an
// OAuth account whose provider returned no email (notably Apple after the first
// sign-in, or when the user chose "Hide My Email"). It is deterministic for a
// given provider + provider-user-id, so the same identity always maps to the
// same address and can be found again by GetByEmail as well as by provider id.
func syntheticOAuthEmail(provider, providerUserID string) string {
	sanitize := func(s string) string {
		return strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
				return r
			case r >= 'A' && r <= 'Z':
				return r + ('a' - 'A')
			default:
				return '-'
			}
		}, s)
	}
	return fmt.Sprintf("%s_%s@no-email.hamsaya.af", sanitize(provider), sanitize(providerUserID))
}

// AuthenticateWithOAuth authenticates or registers a user with OAuth
func (s *OAuthService) AuthenticateWithOAuth(ctx context.Context, oauthInfo *OAuthUserInfo) (*models.User, *models.Profile, bool, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(oauthInfo.Email))

	// Apple omits the email claim on logins after the first one (and the user
	// may have chosen "Hide My Email"). When email is absent, recover the
	// returning user by provider + provider-user-id instead of failing. We never
	// re-ask the user for email — Apple already authenticated them.
	if email == "" {
		if oauthInfo.ProviderUserID != "" {
			existingByProvider, lookupErr := s.userRepo.GetByOAuthProviderID(ctx, oauthInfo.Provider, oauthInfo.ProviderUserID)
			if lookupErr == nil && existingByProvider != nil {
				if err := s.userRepo.UpdateLastLogin(ctx, existingByProvider.ID); err != nil {
					s.logger.Error("Failed to update last login", zap.Error(err))
				}
				profile, profErr := s.userRepo.GetProfileByUserID(ctx, existingByProvider.ID)
				if profErr != nil {
					s.logger.Error("Failed to get profile", zap.Error(profErr))
					return nil, nil, false, utils.NewInternalError("Failed to get profile", profErr)
				}
				s.logger.Info("OAuth returning user recovered by provider id (no email claim)",
					zap.String("user_id", existingByProvider.ID),
					zap.String("provider", oauthInfo.Provider),
				)
				return existingByProvider, profile, false, nil
			}
		}

		// Brand-new user and the provider gave us no email (Apple omits it after
		// the first authorization and when the user hides it). Apple still
		// authenticated the identity via the verified `sub`, so synthesize a
		// stable, non-deliverable placeholder email from it and create the
		// account. The user is never asked for an email (App Store 4.0); they
		// may add a real one later from profile settings.
		if oauthInfo.ProviderUserID != "" {
			email = syntheticOAuthEmail(oauthInfo.Provider, oauthInfo.ProviderUserID)
			// Identity is provider-verified; the placeholder can't receive mail,
			// so mark it verified to avoid bouncing the user to email-verification.
			oauthInfo.EmailVerified = true
			s.logger.Info("OAuth provider gave no email; using synthetic placeholder",
				zap.String("provider", oauthInfo.Provider),
				zap.String("synthetic_email", email),
			)
		} else {
			return nil, nil, false, utils.NewBadRequestError("Email is required from OAuth provider", nil)
		}
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

		// If the profile has no avatar (e.g. registered before the fix) and the OAuth
		// provider returned a picture URL, persist it now so subsequent fetches have it.
		if profile.Avatar == nil && oauthInfo.Picture != "" {
			profile.Avatar = &models.Photo{
				URL:      oauthInfo.Picture,
				Name:     "avatar",
				MimeType: "image/jpeg",
			}
			if updateErr := s.userRepo.UpdateProfile(ctx, profile); updateErr != nil {
				s.logger.Error("Failed to back-fill OAuth avatar", zap.Error(updateErr))
				// Non-fatal — continue with profile as-is
			}
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
		Role:                models.RoleUser,
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
