package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

const (
	// Account locking configuration
	MaxLoginAttempts = 5
	LockDuration     = 30 * time.Minute
)

// AuthService handles authentication operations
type AuthService struct {
	userRepo        repositories.UserRepository
	passwordService *PasswordService
	jwtService      *JWTService
	emailService    *EmailService
	tokenStorage    *TokenStorageService
	mfaService      *MFAService
	logger          *zap.Logger
	cfg             *config.Config
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo repositories.UserRepository,
	passwordService *PasswordService,
	jwtService *JWTService,
	emailService *EmailService,
	tokenStorage *TokenStorageService,
	mfaService *MFAService,
	cfg *config.Config,
	logger *zap.Logger,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		passwordService: passwordService,
		jwtService:      jwtService,
		emailService:    emailService,
		tokenStorage:    tokenStorage,
		mfaService:      mfaService,
		logger:          logger,
		cfg:             cfg,
	}
}

// Register creates a complete user profile with firstname, lastname, and location
// This endpoint requires email, password, firstname, lastname, latitude, and longitude
func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	now := time.Now()

	// USER DOESN'T EXIST - Create new user with complete profile
	if err != nil || existingUser == nil {
		// Validate password strength
		if err := s.passwordService.ValidatePasswordStrength(req.Password); err != nil {
			return nil, utils.NewBadRequestError(err.Error(), err)
		}

		// Hash password
		passwordHash, err := s.passwordService.Hash(req.Password)
		if err != nil {
			s.logger.Error("Failed to hash password", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create user", err)
		}

		// Create user
		userID := uuid.New().String()
		user := &models.User{
			ID:                  userID,
			Email:               email,
			PasswordHash:        &passwordHash,
			EmailVerified:       false,
			PhoneVerified:       false,
			MFAEnabled:          false,
			Role:                models.RoleUser,
			FailedLoginAttempts: 0,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			s.logger.Error("Failed to create user", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create user", err)
		}

		// Create complete profile with location
		profile := &models.Profile{
			ID:         userID,
			FirstName:  &req.FirstName,
			LastName:   &req.LastName,
			IsComplete: true, // Profile is complete with all required fields
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		// Set location from request
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: req.Longitude, Y: req.Latitude},
			Valid: true,
		}

		if err := s.userRepo.CreateProfile(ctx, profile); err != nil {
			s.logger.Error("Failed to create profile", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create profile", err)
		}

		s.logger.Info("User registered with complete profile",
			zap.String("user_id", userID),
			zap.String("email", email),
		)

		// Generate AAL1 token pair
		return s.generateAuthResponse(ctx, user, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
	}

	// USER EXISTS - Check if profile needs to be completed
	profile, err := s.userRepo.GetProfileByUserID(ctx, existingUser.ID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get profile", err)
	}

	// If profile is already complete, return error
	if profile.IsComplete {
		return nil, utils.NewConflictError("User already registered with complete profile", nil)
	}

	// Profile is incomplete - update it with complete information
	profile.FirstName = &req.FirstName
	profile.LastName = &req.LastName
	profile.Location = &pgtype.Point{
		P:     pgtype.Vec2{X: req.Longitude, Y: req.Latitude},
		Valid: true,
	}
	profile.IsComplete = true
	profile.UpdatedAt = now

	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to update profile", zap.Error(err))
		return nil, utils.NewInternalError("Failed to complete profile", err)
	}

	s.logger.Info("User profile completed",
		zap.String("user_id", existingUser.ID),
		zap.String("email", email),
	)

	// Generate AAL1 token pair for existing user
	return s.generateAuthResponse(ctx, existingUser, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
}

// UnifiedAuth handles both login and registration in a single endpoint
// If user exists, logs them in. If not, registers them with location.
func (s *AuthService) UnifiedAuth(ctx context.Context, req *models.UnifiedAuthRequest) (*models.AuthResponse, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user exists
	existingUser, err := s.userRepo.GetByEmail(ctx, email)

	// USER EXISTS - Login flow
	if err == nil && existingUser != nil {
		// Check if account is locked
		if existingUser.IsLocked() {
			s.logger.Warn("Login attempt for locked account",
				zap.String("user_id", existingUser.ID),
				zap.Time("locked_until", *existingUser.LockedUntil),
			)
			return nil, utils.NewUnauthorizedError(
				fmt.Sprintf("Account is locked until %s due to too many failed login attempts",
					existingUser.LockedUntil.Format(time.RFC3339)),
				nil,
			)
		}

		// Verify password
		if existingUser.PasswordHash == nil || !s.passwordService.Verify(req.Password, *existingUser.PasswordHash) {
			// Increment failed login attempts
			attempts := existingUser.FailedLoginAttempts + 1
			var lockedUntil *time.Time
			if attempts >= MaxLoginAttempts {
				lockTime := time.Now().Add(LockDuration)
				lockedUntil = &lockTime
			}

			if err := s.userRepo.UpdateLoginAttempts(ctx, existingUser.ID, attempts, lockedUntil); err != nil {
				s.logger.Error("Failed to update login attempts", zap.Error(err))
			}

			s.logger.Warn("Failed login attempt",
				zap.String("user_id", existingUser.ID),
				zap.Int("attempts", attempts),
			)

			return nil, utils.NewUnauthorizedError("Invalid email or password", nil)
		}

		// Check if MFA is enabled
		if existingUser.MFAEnabled {
			// Generate MFA challenge
			challengeID := s.jwtService.GenerateMFAChallengeID()

			// Store MFA challenge in Redis (valid for 5 minutes)
			if err := s.tokenStorage.StoreMFAChallenge(ctx, challengeID, existingUser.ID, 5*time.Minute); err != nil {
				s.logger.Error("Failed to store MFA challenge", zap.Error(err))
				return nil, utils.NewInternalError("Failed to initiate MFA", err)
			}

			s.logger.Info("MFA challenge generated",
				zap.String("user_id", existingUser.ID),
				zap.String("challenge_id", challengeID),
			)

			return &models.AuthResponse{
				RequiresMFA:    true,
				MFAChallengeID: &challengeID,
			}, nil
		}

		// Login successful
		s.logger.Info("User logged in via unified auth",
			zap.String("user_id", existingUser.ID),
			zap.String("email", email),
		)

		return s.generateAuthResponse(ctx, existingUser, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
	}

	// USER DOESN'T EXIST - Registration flow
	// Validate that required fields are provided for registration
	if req.FirstName == nil || *req.FirstName == "" {
		return nil, utils.NewBadRequestError("first_name is required for new users", nil)
	}
	if req.LastName == nil || *req.LastName == "" {
		return nil, utils.NewBadRequestError("last_name is required for new users", nil)
	}
	if req.Latitude == nil {
		return nil, utils.NewBadRequestError("latitude is required for new users", nil)
	}
	if req.Longitude == nil {
		return nil, utils.NewBadRequestError("longitude is required for new users", nil)
	}

	// Validate password strength
	if err := s.passwordService.ValidatePasswordStrength(req.Password); err != nil {
		return nil, utils.NewBadRequestError(err.Error(), err)
	}

	// Hash password
	passwordHash, err := s.passwordService.Hash(req.Password)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create user", err)
	}

	// Create user
	userID := uuid.New().String()
	now := time.Now()
	user := &models.User{
		ID:                  userID,
		Email:               email,
		PasswordHash:        &passwordHash,
		EmailVerified:       false,
		PhoneVerified:       false,
		MFAEnabled:          false,
		Role:                models.RoleUser,
		FailedLoginAttempts: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create user", err)
	}

	// Create profile with location
	profile := &models.Profile{
		ID:         userID,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		IsComplete: false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Set location if provided
	if req.Latitude != nil && req.Longitude != nil {
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude},
			Valid: true,
		}
	}

	if err := s.userRepo.CreateProfile(ctx, profile); err != nil {
		s.logger.Error("Failed to create profile", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create profile", err)
	}

	s.logger.Info("User registered via unified auth",
		zap.String("user_id", userID),
		zap.String("email", email),
	)

	// Generate AAL1 token pair (basic authentication)
	sessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(userID, email, models.AAL1, sessionID)
	if err != nil {
		s.logger.Error("Failed to generate tokens", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	// Create session
	session := &models.UserSession{
		ID:              sessionID,
		UserID:          userID,
		RefreshToken:    tokenPair.RefreshToken,
		AccessTokenHash: s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:      req.DeviceInfo,
		IPAddress:       req.IPAddress,
		UserAgent:       req.UserAgent,
		ExpiresAt:       now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:         false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create session", err)
	}

	return &models.AuthResponse{
		User: &models.UserResponse{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			MFAEnabled:    user.MFAEnabled,
			CreatedAt:     user.CreatedAt,
		},
		Profile: &models.ProfileResponse{
			ID:         profile.ID,
			FirstName:  profile.FirstName,
			LastName:   profile.LastName,
			IsComplete: profile.IsComplete,
		},
		Tokens: tokenPair,
	}, nil
}

// Login authenticates a user and returns tokens
// If user doesn't exist, it auto-registers them with email and password only
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, email)

	// USER DOESN'T EXIST - Auto-register
	if err != nil {
		s.logger.Info("Auto-registering new user via login", zap.String("email", email))

		// Validate password strength
		if err := s.passwordService.ValidatePasswordStrength(req.Password); err != nil {
			return nil, utils.NewBadRequestError(err.Error(), err)
		}

		// Hash password
		passwordHash, err := s.passwordService.Hash(req.Password)
		if err != nil {
			s.logger.Error("Failed to hash password", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create user", err)
		}

		// Create user with basic info only
		userID := uuid.New().String()
		now := time.Now()
		user = &models.User{
			ID:                  userID,
			Email:               email,
			PasswordHash:        &passwordHash,
			EmailVerified:       false,
			PhoneVerified:       false,
			MFAEnabled:          false,
			Role:                models.RoleUser,
			FailedLoginAttempts: 0,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			s.logger.Error("Failed to create user", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create user", err)
		}

		// Create empty profile
		profile := &models.Profile{
			ID:         userID,
			IsComplete: false,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if err := s.userRepo.CreateProfile(ctx, profile); err != nil {
			s.logger.Error("Failed to create profile", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create profile", err)
		}

		s.logger.Info("User auto-registered successfully",
			zap.String("user_id", userID),
			zap.String("email", email),
		)

		// Return auth response for newly created user
		return s.generateAuthResponse(ctx, user, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
	}

	// USER EXISTS - Normal login flow
	// Check if account is locked
	if user.IsLocked() {
		s.logger.Warn("Login attempt for locked account",
			zap.String("user_id", user.ID),
			zap.Time("locked_until", *user.LockedUntil),
		)
		return nil, utils.NewUnauthorizedError(
			fmt.Sprintf("Account is locked until %s due to too many failed login attempts",
				user.LockedUntil.Format(time.RFC3339)),
			nil,
		)
	}

	// Verify password
	if user.PasswordHash == nil || !s.passwordService.Verify(req.Password, *user.PasswordHash) {
		// Increment failed login attempts
		attempts := user.FailedLoginAttempts + 1
		var lockedUntil *time.Time
		if attempts >= MaxLoginAttempts {
			lockTime := time.Now().Add(LockDuration)
			lockedUntil = &lockTime
		}

		if err := s.userRepo.UpdateLoginAttempts(ctx, user.ID, attempts, lockedUntil); err != nil {
			s.logger.Error("Failed to update login attempts", zap.Error(err))
		}

		s.logger.Warn("Failed login attempt",
			zap.String("user_id", user.ID),
			zap.Int("attempts", attempts),
		)

		return nil, utils.NewUnauthorizedError("Invalid email or password", nil)
	}

	// Check if MFA is enabled
	if user.MFAEnabled {
		// Generate MFA challenge
		challengeID := s.jwtService.GenerateMFAChallengeID()

		// Store MFA challenge in Redis (valid for 5 minutes)
		if err := s.tokenStorage.StoreMFAChallenge(ctx, challengeID, user.ID, 5*time.Minute); err != nil {
			s.logger.Error("Failed to store MFA challenge", zap.Error(err))
			return nil, utils.NewInternalError("Failed to initiate MFA", err)
		}

		s.logger.Info("MFA challenge generated",
			zap.String("user_id", user.ID),
			zap.String("challenge_id", challengeID),
		)

		return &models.AuthResponse{
			RequiresMFA:    true,
			MFAChallengeID: &challengeID,
		}, nil
	}

	// Generate AAL1 token pair (basic authentication, no MFA)
	return s.generateAuthResponse(ctx, user, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
}

// VerifyMFA verifies an MFA code and returns tokens
func (s *AuthService) VerifyMFA(ctx context.Context, req *models.MFAVerifyChallengeRequest) (*models.AuthResponse, error) {
	// Get user ID from MFA challenge
	userID, err := s.tokenStorage.GetUserIDFromMFAChallenge(ctx, req.ChallengeID)
	if err != nil {
		s.logger.Warn("Invalid or expired MFA challenge", zap.Error(err))
		return nil, utils.NewBadRequestError("Invalid or expired MFA challenge", err)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return nil, utils.NewInternalError("Failed to verify MFA", err)
	}

	// Verify TOTP code
	valid, err := s.mfaService.VerifyTOTP(ctx, userID, req.Code)
	if err != nil {
		s.logger.Error("Failed to verify TOTP", zap.Error(err))
		return nil, err
	}

	if !valid {
		s.logger.Warn("Invalid MFA code",
			zap.String("user_id", userID),
			zap.String("challenge_id", req.ChallengeID),
		)
		return nil, utils.NewUnauthorizedError("Invalid verification code", nil)
	}

	// Delete MFA challenge
	if err := s.tokenStorage.DeleteMFAChallenge(ctx, req.ChallengeID); err != nil {
		s.logger.Error("Failed to delete MFA challenge", zap.Error(err))
		// Continue anyway
	}

	// Generate AAL2 token pair (MFA verified)
	response, err := s.generateAuthResponse(ctx, user, models.AAL2, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	s.logger.Info("MFA verification successful",
		zap.String("user_id", userID),
	)

	return response, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *AuthService) RefreshToken(ctx context.Context, req *models.RefreshTokenRequest) (*models.TokenPair, error) {
	// Get session by refresh token
	session, err := s.userRepo.GetSessionByRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		s.logger.Warn("Invalid refresh token", zap.Error(err))
		return nil, utils.NewUnauthorizedError("Invalid refresh token", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		s.logger.Warn("Expired refresh token", zap.String("session_id", session.ID))
		return nil, utils.NewUnauthorizedError("Refresh token has expired", nil)
	}

	// Check if session is revoked
	if session.Revoked {
		s.logger.Warn("Revoked refresh token used", zap.String("session_id", session.ID))
		return nil, utils.NewUnauthorizedError("Refresh token has been revoked", nil)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return nil, utils.NewInternalError("Failed to refresh token", err)
	}

	// Determine AAL level based on MFA status
	aal := models.AAL1
	if user.MFAEnabled {
		// Check if this session was verified with MFA
		// For now, we'll use AAL1 for refresh tokens
		// In production, you might want to track AAL level in session
		aal = models.AAL1
	}

	// Generate new token pair
	newSessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(user.ID, user.Email, aal, newSessionID)
	if err != nil {
		s.logger.Error("Failed to generate tokens", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	// Revoke old session
	if err := s.userRepo.RevokeSession(ctx, session.ID); err != nil {
		s.logger.Error("Failed to revoke old session", zap.Error(err))
		// Continue anyway - old token will expire naturally
	}

	// Create new session
	now := time.Now()
	newSession := &models.UserSession{
		ID:              newSessionID,
		UserID:          user.ID,
		RefreshToken:    tokenPair.RefreshToken,
		AccessTokenHash: s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:      session.DeviceInfo,
		IPAddress:       session.IPAddress,
		UserAgent:       session.UserAgent,
		ExpiresAt:       now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:         false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.userRepo.CreateSession(ctx, newSession); err != nil {
		s.logger.Error("Failed to create new session", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create session", err)
	}

	s.logger.Info("Token refreshed successfully",
		zap.String("user_id", user.ID),
		zap.String("old_session_id", session.ID),
		zap.String("new_session_id", newSessionID),
	)

	return tokenPair, nil
}

// Logout revokes the current session
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if err := s.userRepo.RevokeSession(ctx, sessionID); err != nil {
		s.logger.Error("Failed to revoke session", zap.Error(err))
		return utils.NewInternalError("Failed to logout", err)
	}

	s.logger.Info("User logged out", zap.String("session_id", sessionID))
	return nil
}

// LogoutAll revokes all sessions for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	if err := s.userRepo.RevokeAllUserSessions(ctx, userID); err != nil {
		s.logger.Error("Failed to revoke all sessions", zap.Error(err))
		return utils.NewInternalError("Failed to logout from all devices", err)
	}

	s.logger.Info("User logged out from all devices", zap.String("user_id", userID))
	return nil
}

// VerifyEmail verifies a user's email address
func (s *AuthService) VerifyEmail(ctx context.Context, req *models.VerifyEmailRequest) error {
	// Get user ID from verification token
	userID, err := s.tokenStorage.GetUserIDFromVerificationToken(ctx, req.Token)
	if err != nil {
		s.logger.Warn("Invalid or expired verification token", zap.Error(err))
		return utils.NewBadRequestError("Invalid or expired verification token", err)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return utils.NewInternalError("Failed to verify email", err)
	}

	// Check if already verified
	if user.EmailVerified {
		s.logger.Info("Email already verified", zap.String("user_id", userID))
		return nil
	}

	// Update email verified status
	user.EmailVerified = true
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user", zap.Error(err))
		return utils.NewInternalError("Failed to verify email", err)
	}

	// Delete verification token
	if err := s.tokenStorage.DeleteVerificationToken(ctx, req.Token); err != nil {
		s.logger.Error("Failed to delete verification token", zap.Error(err))
		// Continue anyway
	}

	// Get profile for welcome email
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.Error(err))
		// Continue without welcome email
	} else {
		// Send welcome email
		name := ""
		if profile.FirstName != nil && profile.LastName != nil {
			name = *profile.FirstName + " " + *profile.LastName
		}
		if err := s.emailService.SendWelcomeEmail(user.Email, name); err != nil {
			s.logger.Error("Failed to send welcome email", zap.Error(err))
			// Continue anyway
		}
	}

	s.logger.Info("Email verified successfully", zap.String("user_id", userID))
	return nil
}

// ForgotPassword initiates password reset flow
func (s *AuthService) ForgotPassword(ctx context.Context, req *models.ForgotPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal if email exists
		s.logger.Warn("Password reset requested for non-existent email", zap.String("email", email))
		return nil
	}

	// Generate reset token
	resetToken, err := s.jwtService.GeneratePasswordResetToken()
	if err != nil {
		s.logger.Error("Failed to generate reset token", zap.Error(err))
		return utils.NewInternalError("Failed to process password reset", err)
	}

	// Store reset token in Redis (valid for 15 minutes)
	if err := s.tokenStorage.StorePasswordResetToken(ctx, user.ID, resetToken, 15*time.Minute); err != nil {
		s.logger.Error("Failed to store reset token", zap.Error(err))
		return utils.NewInternalError("Failed to process password reset", err)
	}

	// Get profile for personalized email
	profile, err := s.userRepo.GetProfileByUserID(ctx, user.ID)
	name := ""
	if err == nil && profile.FirstName != nil && profile.LastName != nil {
		name = *profile.FirstName + " " + *profile.LastName
	}

	// Send password reset email
	if err := s.emailService.SendPasswordResetEmail(user.Email, name, resetToken); err != nil {
		s.logger.Error("Failed to send password reset email", zap.Error(err))
		// Continue anyway - token is stored
	}

	s.logger.Info("Password reset requested",
		zap.String("user_id", user.ID),
	)

	return nil
}

// ResetPassword resets a user's password
func (s *AuthService) ResetPassword(ctx context.Context, req *models.ResetPasswordRequest) error {
	// Get user ID from reset token
	userID, err := s.tokenStorage.GetUserIDFromPasswordResetToken(ctx, req.Token)
	if err != nil {
		s.logger.Warn("Invalid or expired reset token", zap.Error(err))
		return utils.NewBadRequestError("Invalid or expired password reset token", err)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return utils.NewInternalError("Failed to reset password", err)
	}

	// Validate new password strength
	if err := s.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		return utils.NewBadRequestError(err.Error(), err)
	}

	// Hash new password
	newPasswordHash, err := s.passwordService.Hash(req.NewPassword)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return utils.NewInternalError("Failed to reset password", err)
	}

	// Update password
	user.PasswordHash = &newPasswordHash
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user password", zap.Error(err))
		return utils.NewInternalError("Failed to reset password", err)
	}

	// Revoke all sessions
	if err := s.userRepo.RevokeAllUserSessions(ctx, userID); err != nil {
		s.logger.Error("Failed to revoke sessions", zap.Error(err))
		// Continue anyway
	}

	// Delete reset token
	if err := s.tokenStorage.DeletePasswordResetToken(ctx, req.Token); err != nil {
		s.logger.Error("Failed to delete reset token", zap.Error(err))
		// Continue anyway
	}

	// Get profile for personalized email
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	name := ""
	if err == nil && profile.FirstName != nil && profile.LastName != nil {
		name = *profile.FirstName + " " + *profile.LastName
	}

	// Send password changed notification email
	if err := s.emailService.SendPasswordChangedEmail(user.Email, name); err != nil {
		s.logger.Error("Failed to send password changed email", zap.Error(err))
		// Continue anyway
	}

	s.logger.Info("Password reset successfully", zap.String("user_id", userID))
	return nil
}

// ChangePassword changes a user's password (authenticated)
func (s *AuthService) ChangePassword(ctx context.Context, userID string, req *models.ChangePasswordRequest) error {
	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return utils.NewInternalError("Failed to change password", err)
	}

	// Verify current password
	if user.PasswordHash == nil || !s.passwordService.Verify(req.CurrentPassword, *user.PasswordHash) {
		return utils.NewUnauthorizedError("Current password is incorrect", nil)
	}

	// Validate new password strength
	if err := s.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		return utils.NewBadRequestError(err.Error(), err)
	}

	// Check if new password is different from current
	if req.CurrentPassword == req.NewPassword {
		return utils.NewBadRequestError("New password must be different from current password", nil)
	}

	// Hash new password
	newPasswordHash, err := s.passwordService.Hash(req.NewPassword)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return utils.NewInternalError("Failed to change password", err)
	}

	// Update password
	user.PasswordHash = &newPasswordHash
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user password", zap.Error(err))
		return utils.NewInternalError("Failed to change password", err)
	}

	// Revoke all sessions except current one
	// TODO: Pass current session ID to exclude it
	if err := s.userRepo.RevokeAllUserSessions(ctx, userID); err != nil {
		s.logger.Error("Failed to revoke sessions", zap.Error(err))
		// Continue anyway
	}

	// Get profile for personalized email
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	name := ""
	if err == nil && profile.FirstName != nil && profile.LastName != nil {
		name = *profile.FirstName + " " + *profile.LastName
	}

	// Send password changed notification email
	if err := s.emailService.SendPasswordChangedEmail(user.Email, name); err != nil {
		s.logger.Error("Failed to send password changed email", zap.Error(err))
		// Continue anyway
	}

	s.logger.Info("Password changed successfully", zap.String("user_id", userID))
	return nil
}

// GetActiveSessions returns all active sessions for a user
func (s *AuthService) GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	sessions, err := s.userRepo.GetActiveSessions(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get active sessions", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get sessions", err)
	}

	return sessions, nil
}

// GenerateTokensForUser generates tokens for a user (used by OAuth and other flows)
func (s *AuthService) GenerateTokensForUser(
	ctx context.Context,
	user *models.User,
	aal string,
	deviceInfo, ipAddress, userAgent *string,
) (*models.TokenPair, error) {
	// Convert string AAL to int
	aalLevel := models.AAL1
	if aal == "AAL2" {
		aalLevel = models.AAL2
	}

	// Generate token pair
	sessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(user.ID, user.Email, aalLevel, sessionID)
	if err != nil {
		s.logger.Error("Failed to generate tokens", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	// Create session
	now := time.Now()
	session := &models.UserSession{
		ID:              sessionID,
		UserID:          user.ID,
		RefreshToken:    tokenPair.RefreshToken,
		AccessTokenHash: s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:      deviceInfo,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		ExpiresAt:       now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:         false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create session", err)
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		s.logger.Error("Failed to update last login", zap.Error(err))
		// Continue anyway
	}

	return tokenPair, nil
}

// generateAuthResponse generates a complete auth response with tokens and user info
func (s *AuthService) generateAuthResponse(
	ctx context.Context,
	user *models.User,
	aal int,
	deviceInfo, ipAddress, userAgent *string,
) (*models.AuthResponse, error) {
	// Get profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, user.ID)
	if err != nil {
		s.logger.Error("Failed to get profile", zap.Error(err))
		return nil, utils.NewInternalError("Failed to complete login", err)
	}

	// Generate token pair
	sessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(user.ID, user.Email, aal, sessionID)
	if err != nil {
		s.logger.Error("Failed to generate tokens", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	// Create session
	now := time.Now()
	session := &models.UserSession{
		ID:              sessionID,
		UserID:          user.ID,
		RefreshToken:    tokenPair.RefreshToken,
		AccessTokenHash: s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:      deviceInfo,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		ExpiresAt:       now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:         false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create session", err)
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		s.logger.Error("Failed to update last login", zap.Error(err))
		// Continue anyway
	}

	s.logger.Info("User logged in successfully",
		zap.String("user_id", user.ID),
		zap.Int("aal", aal),
		zap.String("session_id", sessionID),
	)

	return &models.AuthResponse{
		User: &models.UserResponse{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			MFAEnabled:    user.MFAEnabled,
			CreatedAt:     user.CreatedAt,
		},
		Profile: &models.ProfileResponse{
			ID:         profile.ID,
			FirstName:  profile.FirstName,
			LastName:   profile.LastName,
			Avatar:     profile.Avatar,
			IsComplete: profile.IsComplete,
		},
		Tokens: tokenPair,
	}, nil
}
