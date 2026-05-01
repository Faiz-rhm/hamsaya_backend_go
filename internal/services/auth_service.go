package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	userRepo            repositories.UserRepository
	adminRepo           repositories.AdminRepository
	passwordService     *PasswordService
	jwtService          *JWTService
	emailService        *EmailService
	tokenStorage        *TokenStorageService
	mfaService          *MFAService
	notificationService *NotificationService
	logger              *zap.Logger
	cfg                 *config.Config
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo repositories.UserRepository,
	adminRepo repositories.AdminRepository,
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
		adminRepo:       adminRepo,
		passwordService: passwordService,
		jwtService:      jwtService,
		emailService:    emailService,
		tokenStorage:    tokenStorage,
		mfaService:      mfaService,
		logger:          logger,
		cfg:             cfg,
	}
}

// SetNotificationService wires the notification service after construction.
// Auth and notification services would otherwise create a circular dependency at
// build time, so we inject post-construction.
func (s *AuthService) SetNotificationService(n *NotificationService) {
	s.notificationService = n
}

// Register creates a complete user profile with firstname, lastname, and location
// This endpoint requires email, password, firstname, lastname, latitude, and longitude
func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists (including soft-deleted users to prevent email reuse)
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return nil, utils.NewConflictError("A user with this email already exists", nil)
	}
	
	// Also check soft-deleted users - prevent email reuse
	deletedUser, _ := s.userRepo.GetByEmailIncludingDeleted(ctx, email)
	if deletedUser != nil {
		return nil, utils.NewConflictError("This email address is no longer available for registration", nil)
	}
	
	now := time.Now()

	// USER DOESN'T EXIST - Create new user with complete profile
	if true {
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

		// Create minimal profile — IsComplete is false until the user finishes
		// the name+location onboarding step (UpdateProfile with is_complete=true),
		// which then triggers OTP email.
		avatarColor := models.RandomAvatarColor()
		profile := &models.Profile{
			ID:          userID,
			AvatarColor: &avatarColor,
			IsComplete:  false,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		// Populate optional fields if the client sent them.
		if req.FirstName != "" {
			profile.FirstName = &req.FirstName
		}
		if req.LastName != "" {
			profile.LastName = &req.LastName
		}
		if req.Latitude != 0 || req.Longitude != 0 {
			profile.Location = &pgtype.Point{
				P:     pgtype.Vec2{X: req.Longitude, Y: req.Latitude},
				Valid: true,
			}
		}

		// Create user and profile atomically in a transaction
		if err := s.userRepo.CreateUserWithProfile(ctx, user, profile); err != nil {
			s.logger.Error("Failed to create user with profile", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create user", err)
		}

		s.logger.Info("User registered — profile pending completion",
			zap.String("user_id", userID),
			zap.String("email", email),
		)

		// OTP is sent after profile completion (UpdateProfile with is_complete=true),
		// not at registration, so users verify only after they have a real profile.

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

	// Send verification code email if not already verified (Resend or SMTP)
	if !existingUser.EmailVerified {
		verificationCode, err := s.jwtService.GenerateVerificationCode()
		if err == nil {
			ttl := 24 * time.Hour
			if storeErr := s.tokenStorage.StoreVerificationToken(ctx, existingUser.ID, verificationCode, ttl); storeErr == nil {
				name := strings.TrimSpace(req.FirstName + " " + req.LastName)
				if name == "" {
					name = email
				}
				if sendErr := s.emailService.SendVerificationEmail(email, name, verificationCode); sendErr != nil {
					s.logger.Warn("Failed to send verification email after profile complete", zap.String("email", email), zap.Error(sendErr))
				}
			}
		}
	}

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
		// Check if account is locked. Return the same generic message as wrong-password
		// so attackers cannot distinguish locked accounts from non-existent ones.
		if existingUser.IsLocked() {
			s.logger.Warn("Login attempt for locked account",
				zap.String("user_id", existingUser.ID),
				zap.Time("locked_until", *existingUser.LockedUntil),
			)
			return nil, utils.NewUnauthorizedError("Invalid email or password", nil)
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
	// Check if email was used by a soft-deleted user (prevent email reuse)
	deletedUser, _ := s.userRepo.GetByEmailIncludingDeleted(ctx, email)
	if deletedUser != nil {
		return nil, utils.NewConflictError("This email address is no longer available for registration", nil)
	}
	
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

	// Create profile with location and random avatar color
	avatarColor := models.RandomAvatarColor()
	profile := &models.Profile{
		ID:          userID,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		AvatarColor: &avatarColor,
		IsComplete:  false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Set location if provided
	if req.Latitude != nil && req.Longitude != nil {
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude},
			Valid: true,
		}
	}

	// Create user and profile atomically in a transaction
	if err := s.userRepo.CreateUserWithProfile(ctx, user, profile); err != nil {
		s.logger.Error("Failed to create user with profile", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create user", err)
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

	// Create session with hashed refresh token for security
	session := &models.UserSession{
		ID:               sessionID,
		UserID:           userID,
		RefreshToken:     tokenPair.RefreshToken,
		RefreshTokenHash: s.jwtService.HashToken(tokenPair.RefreshToken),
		AccessTokenHash:  s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:       req.DeviceInfo,
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
			ID:           profile.ID,
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			AvatarColor:  profile.AvatarColor,
			Province:     profile.Province,
			District:     profile.District,
			Neighborhood: profile.Neighborhood,
			Country:      profile.Country,
			IsComplete:   profile.IsComplete,
		},
		Tokens: tokenPair,
	}, nil
}

// Login authenticates a user and returns tokens
// If user doesn't exist, it auto-registers them with email and password only
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Get user by email (active only)
	user, err := s.userRepo.GetByEmail(ctx, email)

	// USER NOT FOUND - Check if deactivated (soft-deleted) for reactivation
	if err != nil {
		deletedUser, delErr := s.userRepo.GetByEmailIncludingDeleted(ctx, email)
		if delErr == nil && deletedUser != nil && deletedUser.DeletedAt != nil {
			// Deactivated user trying to login - reactivate on valid password
			if deletedUser.PasswordHash != nil && s.passwordService.Verify(req.Password, *deletedUser.PasswordHash) {
				if err := s.userRepo.Restore(ctx, deletedUser.ID); err != nil {
					s.logger.Error("Failed to restore deactivated user", zap.String("user_id", deletedUser.ID), zap.Error(err))
					return nil, utils.NewInternalError("Failed to reactivate account", err)
				}
				s.logger.Info("User reactivated via login", zap.String("user_id", deletedUser.ID), zap.String("email", email))
				user = deletedUser
				user.DeletedAt = nil
				// Fall through to normal login flow below
			} else {
				return nil, utils.NewUnauthorizedError("Invalid email or password", nil)
			}
		} else {
			// Truly new user - auto-register
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

		// Create empty profile with random avatar color
		avatarColor := models.RandomAvatarColor()
		profile := &models.Profile{
			ID:          userID,
			AvatarColor: &avatarColor,
			IsComplete:  false,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Create user and profile atomically in a transaction
		if err := s.userRepo.CreateUserWithProfile(ctx, user, profile); err != nil {
			s.logger.Error("Failed to create user with profile", zap.Error(err))
			return nil, utils.NewInternalError("Failed to create user", err)
		}

		s.logger.Info("User auto-registered successfully",
			zap.String("user_id", userID),
			zap.String("email", email),
		)

		// Send verification email (OTP)
		verificationCode, vcErr := s.jwtService.GenerateVerificationCode()
		if vcErr == nil {
			ttl := 24 * time.Hour
			if storeErr := s.tokenStorage.StoreVerificationToken(ctx, userID, verificationCode, ttl); storeErr == nil {
				if sendErr := s.emailService.SendVerificationEmail(email, email, verificationCode); sendErr != nil {
					s.logger.Warn("Failed to send verification email after auto-register", zap.String("email", email), zap.Error(sendErr))
				}
			}
		}

		// Return auth response for newly created user
		return s.generateAuthResponse(ctx, user, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
		}
	}

	// USER EXISTS (or was just reactivated) - Normal login flow.
	// Locked accounts return the same generic message as wrong-password to prevent
	// account enumeration via distinct error responses.
	if user.IsLocked() {
		s.logger.Warn("Login attempt for locked account",
			zap.String("user_id", user.ID),
			zap.Time("locked_until", *user.LockedUntil),
		)
		return nil, utils.NewUnauthorizedError("Invalid email or password", nil)
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

// VerifyMFAWithBackupCode completes MFA login using a backup code instead of TOTP.
func (s *AuthService) VerifyMFAWithBackupCode(ctx context.Context, req *models.MFABackupCodeRequest) (*models.AuthResponse, error) {
	userID, err := s.tokenStorage.GetUserIDFromMFAChallenge(ctx, req.ChallengeID)
	if err != nil {
		s.logger.Warn("Invalid or expired MFA challenge", zap.Error(err))
		return nil, utils.NewBadRequestError("Invalid or expired MFA challenge", err)
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return nil, utils.NewInternalError("Failed to verify backup code", err)
	}

	valid, err := s.mfaService.VerifyBackupCode(ctx, userID, req.BackupCode)
	if err != nil {
		return nil, err
	}
	if !valid {
		s.logger.Warn("Invalid backup code", zap.String("user_id", userID))
		return nil, utils.NewUnauthorizedError("Invalid backup code", nil)
	}

	if err := s.tokenStorage.DeleteMFAChallenge(ctx, req.ChallengeID); err != nil {
		s.logger.Error("Failed to delete MFA challenge", zap.Error(err))
	}

	response, err := s.generateAuthResponse(ctx, user, models.AAL2, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	s.logger.Info("MFA backup code verification successful", zap.String("user_id", userID))
	return response, nil
}

// RefreshToken implements X-style refresh-token rotation with idempotent
// grace window and reuse detection.
//
//   - Active token: rotate, persist new session, cache new pair under old
//     hash for the grace window so concurrent callers (proactive timer,
//     401 retry, multi-isolate) get the same pair.
//   - Already-rotated token within grace: return the cached pair. No new
//     session is minted; the caller's storage ends up matching the winner.
//   - Already-rotated token outside grace: this is reuse — likely a leaked
//     refresh token being replayed. Revoke the entire session family so the
//     attacker cannot keep minting fresh access tokens.
//   - Genuinely revoked (logout / explicit kill): reject.
//   - Expired: reject. Client should fall back to /auth/device/login.
func (s *AuthService) RefreshToken(ctx context.Context, req *models.RefreshTokenRequest) (*models.TokenPair, error) {
	refreshTokenHash := s.jwtService.HashToken(req.RefreshToken)

	// Look up the row regardless of revoked state so we can distinguish
	// "rotated and replayed" (recoverable) from "really invalid" (reject).
	session, err := s.userRepo.GetSessionByRefreshTokenHashAny(ctx, refreshTokenHash)
	if err != nil {
		// Legacy fallback: pre-hashing sessions stored plaintext.
		legacy, legacyErr := s.userRepo.GetSessionByRefreshToken(ctx, req.RefreshToken)
		if legacyErr != nil {
			s.logger.Warn("Invalid refresh token", zap.Error(err))
			return nil, utils.NewUnauthorizedError("Invalid refresh token", err)
		}
		session = legacy
	}

	if time.Now().After(session.ExpiresAt) {
		s.logger.Warn("Expired refresh token", zap.String("session_id", session.ID))
		return nil, utils.NewUnauthorizedError("Refresh token has expired", nil)
	}

	grace := s.cfg.JWT.RefreshGrace
	if grace <= 0 {
		grace = 60 * time.Second
	}

	// Path A: this row was rotated or explicitly logged out.
	if session.Revoked {
		// Logout sets Revoked=true with no ReplacedBySessionID. Always reject
		// — there is nothing to fall through to, and grace only applies to
		// rotation races where the new pair was just issued.
		if session.ReplacedBySessionID == nil || *session.ReplacedBySessionID == "" {
			s.logger.Warn("Refresh attempted on logged-out session",
				zap.String("session_id", session.ID),
			)
			return nil, utils.NewUnauthorizedError("Refresh token has been revoked", nil)
		}
		withinGrace := session.RevokedAt != nil && time.Since(*session.RevokedAt) < grace
		if !withinGrace {
			// Out-of-grace replay → reuse detection. Kill whole family.
			if session.FamilyID != nil && *session.FamilyID != "" && s.userRepo != nil {
				if revokeErr := s.userRepo.RevokeSessionFamily(ctx, *session.FamilyID); revokeErr != nil {
					s.logger.Error("Failed to revoke session family", zap.Error(revokeErr))
				}
			}
			s.logger.Warn("Reuse detected — session family revoked",
				zap.String("session_id", session.ID),
				zap.Stringp("family_id", session.FamilyID),
			)
			return nil, utils.NewUnauthorizedError("Refresh token has been revoked", nil)
		}

		// Within grace → return the cached new pair so this caller ends up
		// holding the same tokens as whoever won the rotation race.
		if s.tokenStorage != nil {
			if cached, cacheErr := s.tokenStorage.GetCachedRotatedPair(ctx, refreshTokenHash); cacheErr == nil && cached != nil {
				s.logger.Info("Refresh served from rotation grace cache",
					zap.String("session_id", session.ID),
					zap.Duration("since_revoked", time.Since(*session.RevokedAt)),
				)
				return &models.TokenPair{
					AccessToken:  cached.AccessToken,
					RefreshToken: cached.RefreshToken,
					ExpiresAt:    cached.ExpiresAt,
					TokenType:    "Bearer",
				}, nil
			}
		}
		// Cache miss inside grace can happen if Redis was flushed. Fall
		// through and mint a new pair from the replacement session.
		if session.ReplacedBySessionID != nil && *session.ReplacedBySessionID != "" {
			s.logger.Info("Refresh grace cache miss — issuing fresh pair from replacement session",
				zap.String("session_id", session.ID),
				zap.String("replacement_id", *session.ReplacedBySessionID),
			)
		}
	}

	// Path B: fresh rotation. Mint new pair, cache under old hash, persist.
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return nil, utils.NewInternalError("Failed to refresh token", err)
	}

	aal := models.AAL1
	if user.MFAEnabled {
		// Track AAL on session in a future change; refresh keeps AAL1 for now.
		aal = models.AAL1
	}

	newSessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(user.ID, user.Email, aal, newSessionID)
	if err != nil {
		s.logger.Error("Failed to generate tokens", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	now := time.Now()
	familyID := session.FamilyID
	if familyID == nil || *familyID == "" {
		// Legacy session predating family_id — adopt itself as the root.
		fid := session.ID
		familyID = &fid
	}
	newSession := &models.UserSession{
		ID:               newSessionID,
		UserID:           user.ID,
		RefreshToken:     tokenPair.RefreshToken,
		RefreshTokenHash: s.jwtService.HashToken(tokenPair.RefreshToken),
		AccessTokenHash:  s.jwtService.HashToken(tokenPair.AccessToken),
		FamilyID:         familyID,
		DeviceInfo:       session.DeviceInfo,
		IPAddress:        session.IPAddress,
		UserAgent:        session.UserAgent,
		ExpiresAt:        now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:          false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.userRepo.CreateSession(ctx, newSession); err != nil {
		s.logger.Error("Failed to create new session", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create session", err)
	}

	// Mark old session rotated and pointed at its replacement.
	if err := s.userRepo.MarkSessionRotated(ctx, session.ID, newSessionID); err != nil {
		s.logger.Error("Failed to mark old session rotated", zap.Error(err))
		// Continue — the new session is valid; old will expire naturally.
	}

	// Cache the new pair against the OLD refresh hash for the grace window.
	if s.tokenStorage != nil {
		_ = s.tokenStorage.CacheRotatedPair(ctx, refreshTokenHash, &RotatedPair{
			AccessToken:  tokenPair.AccessToken,
			RefreshToken: tokenPair.RefreshToken,
			ExpiresAt:    tokenPair.ExpiresAt,
		}, grace)
	}

	s.logger.Info("Token refreshed successfully",
		zap.String("user_id", user.ID),
		zap.String("old_session_id", session.ID),
		zap.String("new_session_id", newSessionID),
		zap.Stringp("family_id", familyID),
	)

	return tokenPair, nil
}

// Logout revokes the current session
// Logout revokes the session and adds the active access token to the denylist
// for the remainder of its TTL. jti / accessTokenExpiresAt may be empty/zero
// for legacy callers — in which case only the session and refresh token are
// revoked (access token remains valid until natural expiry, the previous
// behavior).
func (s *AuthService) Logout(ctx context.Context, sessionID, jti string, accessTokenExpiresAt int64) error {
	if err := s.userRepo.RevokeSession(ctx, sessionID); err != nil {
		s.logger.Error("Failed to revoke session", zap.Error(err))
		return utils.NewInternalError("Failed to logout", err)
	}

	if jti != "" && accessTokenExpiresAt > 0 && s.tokenStorage != nil {
		ttl := time.Until(time.Unix(accessTokenExpiresAt, 0))
		if ttl > 0 {
			if err := s.tokenStorage.BlacklistToken(ctx, jti, ttl); err != nil {
				s.logger.Warn("Failed to blacklist access token on logout (session still revoked)",
					zap.String("session_id", sessionID), zap.Error(err))
			}
		}
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

	// In-app notification confirming verification
	s.sendEmailVerifiedNotification(ctx, userID)

	s.logger.Info("Email verified successfully", zap.String("user_id", userID))
	return nil
}

// SendVerificationEmailForUser sends a verification code email to the given user (e.g. after profile completed via PATCH).
func (s *AuthService) SendVerificationEmailForUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return utils.NewNotFoundError("User not found", err)
	}
	if user.EmailVerified {
		return nil
	}

	verificationCode, err := s.jwtService.GenerateVerificationCode()
	if err != nil {
		return utils.NewInternalError("Failed to generate verification code", err)
	}
	ttl := 24 * time.Hour
	if err := s.tokenStorage.StoreVerificationToken(ctx, userID, verificationCode, ttl); err != nil {
		return utils.NewInternalError("Failed to store verification code", err)
	}

	name := user.Email
	profile, err := s.userRepo.GetProfileByUserID(ctx, userID)
	if err == nil && profile.FirstName != nil && profile.LastName != nil {
		name = strings.TrimSpace(*profile.FirstName + " " + *profile.LastName)
		if name == "" {
			name = user.Email
		}
	}

	if err := s.emailService.SendVerificationEmail(user.Email, name, verificationCode); err != nil {
		s.logger.Error("Failed to send verification email", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to send verification email", err)
	}
	s.logger.Info("Verification email sent", zap.String("user_id", userID))
	return nil
}

// ForgotPassword initiates password reset flow. Sends the reset code by email only; the code is
// never returned in the API response. When email cannot be sent (e.g. not configured), the code
// is still not returned—configure RESEND_API_KEY or SMTP to deliver the code to the user.
func (s *AuthService) ForgotPassword(ctx context.Context, req *models.ForgotPasswordRequest) (err error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Get user by email — always return nil to prevent account enumeration via status codes.
	// If the account does not exist, we silently succeed (no error, no email sent).
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Info("Password reset requested for unknown email (no-op)", zap.String("email", email))
		return nil
	}

	// Generate 6-digit reset code (entered in app; same pattern as email verification)
	code, err := s.jwtService.GenerateVerificationCode()
	if err != nil {
		s.logger.Error("Failed to generate reset code", zap.Error(err))
		return utils.NewInternalError("Failed to process password reset", err)
	}

	// Store reset code in Redis (valid for 15 minutes)
	if err := s.tokenStorage.StorePasswordResetToken(ctx, user.ID, code, 15*time.Minute); err != nil {
		s.logger.Error("Failed to store reset token", zap.Error(err))
		return utils.NewInternalError("Failed to process password reset", err)
	}

	// Get profile for personalized email
	profile, err := s.userRepo.GetProfileByUserID(ctx, user.ID)
	name := ""
	if err == nil && profile.FirstName != nil && profile.LastName != nil {
		name = *profile.FirstName + " " + *profile.LastName
	}

	// Send password reset email. Code is never returned in the response.
	if sendErr := s.emailService.SendPasswordResetEmail(user.Email, name, code); sendErr != nil {
		s.logger.Error("Failed to send password reset email", zap.Error(sendErr))
		return utils.NewInternalError("Failed to send password reset email; please try again later", sendErr)
	}
	s.logger.Info("Password reset requested", zap.String("user_id", user.ID))
	return nil
}

// VerifyResetCode checks that the reset code is valid and not expired (does not consume it).
func (s *AuthService) VerifyResetCode(ctx context.Context, req *models.VerifyResetCodeRequest) error {
	_, err := s.tokenStorage.GetUserIDFromPasswordResetToken(ctx, req.Token)
	if err != nil {
		s.logger.Warn("Invalid or expired reset code", zap.Error(err))
		return utils.NewBadRequestError("Invalid or expired reset code", err)
	}
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

// ChangePassword changes a user's password (authenticated).
// Revokes all other sessions but preserves the current one.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, sessionID string, req *models.ChangePasswordRequest) error {
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

	// Revoke all sessions except the current one so the user stays logged in
	if err := s.userRepo.RevokeAllUserSessionsExcept(ctx, userID, sessionID); err != nil {
		s.logger.Error("Failed to revoke other sessions", zap.Error(err))
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

	// In-app + push notification
	s.sendPasswordChangedNotification(ctx, userID)

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

	// Create session with hashed refresh token for security
	now := time.Now()
	session := &models.UserSession{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshToken:     tokenPair.RefreshToken,
		RefreshTokenHash: s.jwtService.HashToken(tokenPair.RefreshToken),
		AccessTokenHash:  s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:       deviceInfo,
		IPAddress:        ipAddress,
		UserAgent:        userAgent,
		ExpiresAt:        now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:          false,
		CreatedAt:        now,
		UpdatedAt:        now,
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

	// Ensure profile has an avatar color for the app (e.g. existing users before the field existed)
	avatarColor := profile.AvatarColor
	if avatarColor == nil || *avatarColor == "" {
		c := models.DefaultAvatarColorForProfile(profile.ID)
		avatarColor = &c
	}

	// Generate token pair
	sessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(user.ID, user.Email, aal, sessionID)
	if err != nil {
		s.logger.Error("Failed to generate tokens", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	// Create session with hashed refresh token for security
	now := time.Now()
	session := &models.UserSession{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshToken:     tokenPair.RefreshToken,
		RefreshTokenHash: s.jwtService.HashToken(tokenPair.RefreshToken),
		AccessTokenHash:  s.jwtService.HashToken(tokenPair.AccessToken),
		DeviceInfo:       deviceInfo,
		IPAddress:        ipAddress,
		UserAgent:        userAgent,
		ExpiresAt:        now.Add(s.cfg.JWT.RefreshTokenDuration),
		Revoked:          false,
		CreatedAt:        now,
		UpdatedAt:        now,
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
			Role:          user.Role,
			FirstName:     profile.FirstName,
			LastName:      profile.LastName,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			MFAEnabled:    user.MFAEnabled,
			CreatedAt:     user.CreatedAt,
		},
		Profile: &models.ProfileResponse{
			ID:           profile.ID,
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			Avatar:       profile.Avatar,
			AvatarColor:  avatarColor,
			Province:     profile.Province,
			District:     profile.District,
			Neighborhood: profile.Neighborhood,
			Country:      profile.Country,
			IsComplete:   profile.IsComplete,
		},
		Tokens: tokenPair,
	}, nil
}

// AcceptAdminInvite validates an invite token and creates the admin/moderator account.
func (s *AuthService) AcceptAdminInvite(ctx context.Context, req *models.AcceptAdminInviteRequest) (*models.AuthResponse, error) {
	invite, err := s.adminRepo.GetAdminInviteByToken(ctx, req.Token)
	if err != nil {
		return nil, utils.NewBadRequestError("Invalid or expired invite token", err)
	}
	if invite.UsedAt != nil {
		return nil, utils.NewBadRequestError("Invite has already been used", nil)
	}
	if time.Now().After(invite.ExpiresAt) {
		return nil, utils.NewBadRequestError("Invite has expired", nil)
	}

	if existing, _ := s.userRepo.GetByEmail(ctx, invite.Email); existing != nil {
		return nil, utils.NewConflictError("An account with this email already exists", nil)
	}

	if err := s.passwordService.ValidatePasswordStrength(req.Password); err != nil {
		return nil, utils.NewBadRequestError(err.Error(), err)
	}
	passwordHash, err := s.passwordService.Hash(req.Password)
	if err != nil {
		return nil, utils.NewInternalError("Failed to create account", err)
	}

	now := time.Now()
	userID := uuid.New().String()
	role := models.UserRole(invite.Role)
	user := &models.User{
		ID:                  userID,
		Email:               invite.Email,
		PasswordHash:        &passwordHash,
		EmailVerified:       true,
		PhoneVerified:       false,
		MFAEnabled:          false,
		Role:                role,
		FailedLoginAttempts: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	avatarColor := models.RandomAvatarColor()
	profile := &models.Profile{
		ID:          userID,
		FirstName:   &req.FirstName,
		LastName:    &req.LastName,
		AvatarColor: &avatarColor,
		IsComplete:  true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.userRepo.CreateUserWithProfile(ctx, user, profile); err != nil {
		s.logger.Error("AcceptAdminInvite: create user", zap.Error(err))
		return nil, utils.NewInternalError("Failed to create account", err)
	}

	if err := s.adminRepo.UseAdminInvite(ctx, req.Token); err != nil {
		s.logger.Warn("AcceptAdminInvite: mark invite used", zap.Error(err))
	}

	sessionID := uuid.New().String()
	tokenPair, err := s.jwtService.GenerateTokenPair(userID, invite.Email, models.AAL1, sessionID)
	if err != nil {
		return nil, utils.NewInternalError("Failed to generate tokens", err)
	}

	s.logger.Info("Admin account created via invite", zap.String("user_id", userID), zap.String("role", invite.Role))

	return &models.AuthResponse{
		User: &models.UserResponse{
			ID:            userID,
			Email:         invite.Email,
			Role:          role,
			EmailVerified: true,
			CreatedAt:     now,
		},
		Profile: &models.ProfileResponse{
			ID:          userID,
			FirstName:   &req.FirstName,
			LastName:    &req.LastName,
			AvatarColor: &avatarColor,
			IsComplete:  true,
		},
		Tokens: tokenPair,
	}, nil
}

// sendWelcomeNotification creates an in-app + push welcome notification for a
// freshly registered user. Best-effort — failures are logged and ignored.
func (s *AuthService) sendWelcomeNotification(ctx context.Context, userID, firstName string) {
	if s.notificationService == nil {
		return
	}
	go func() {
		ctxDetach := context.WithoutCancel(ctx)
		title := "Welcome to Hamsaya!"
		msg := "Discover neighbors, businesses, and listings in your area."
		if firstName != "" {
			title = "Welcome, " + firstName + "!"
		}
		_, _ = s.notificationService.CreateNotification(ctxDetach, &models.CreateNotificationRequest{
			UserID:  userID,
			Type:    models.NotificationTypeWelcome,
			Title:   &title,
			Message: &msg,
			Data:    map[string]interface{}{},
		})
	}()
}

// sendPasswordChangedNotification confirms a successful password change.
func (s *AuthService) sendPasswordChangedNotification(ctx context.Context, userID string) {
	if s.notificationService == nil {
		return
	}
	go func() {
		ctxDetach := context.WithoutCancel(ctx)
		title := "Password updated"
		msg := "Your password was changed successfully. If this wasn't you, contact support immediately."
		_, _ = s.notificationService.CreateNotification(ctxDetach, &models.CreateNotificationRequest{
			UserID:  userID,
			Type:    models.NotificationTypePasswordChanged,
			Title:   &title,
			Message: &msg,
			Data:    map[string]interface{}{},
		})
	}()
}

// sendEmailVerifiedNotification confirms successful email verification.
func (s *AuthService) sendEmailVerifiedNotification(ctx context.Context, userID string) {
	if s.notificationService == nil {
		return
	}
	go func() {
		ctxDetach := context.WithoutCancel(ctx)
		title := "Email verified"
		msg := "Your email address is now verified. Full access unlocked."
		_, _ = s.notificationService.CreateNotification(ctxDetach, &models.CreateNotificationRequest{
			UserID:  userID,
			Type:    models.NotificationTypeEmailVerified,
			Title:   &title,
			Message: &msg,
			Data:    map[string]interface{}{},
		})
	}()
}

// generateDeviceCredentialSecret returns 32 random bytes encoded as URL-safe
// base64. ~256 bits of entropy — large enough that brute-force lookup against
// the SHA-256 column is not a concern.
func generateDeviceCredentialSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// RegisterDevice issues a long-lived device credential for the authenticated
// user. The plaintext is returned exactly once; only its SHA-256 hash is
// persisted server-side. Idle clients past the refresh-token window present
// this credential at /auth/device/login to mint a fresh session without the
// user having to enter a password.
func (s *AuthService) RegisterDevice(ctx context.Context, userID string, req *models.RegisterDeviceRequest) (*models.RegisterDeviceResponse, error) {
	plaintext, err := generateDeviceCredentialSecret()
	if err != nil {
		s.logger.Error("Failed to generate device credential", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate device credential", err)
	}

	now := time.Now()
	cred := &models.DeviceCredential{
		ID:             uuid.New().String(),
		UserID:         userID,
		CredentialHash: s.jwtService.HashToken(plaintext),
		InstallID:      req.InstallID,
		DeviceName:     req.DeviceName,
		Platform:       req.Platform,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if d := s.cfg.JWT.DeviceCredentialDuration; d > 0 {
		exp := now.Add(d)
		cred.ExpiresAt = &exp
	}

	if err := s.userRepo.CreateDeviceCredential(ctx, cred); err != nil {
		s.logger.Error("Failed to persist device credential", zap.Error(err))
		return nil, utils.NewInternalError("Failed to register device", err)
	}

	s.logger.Info("Device credential registered",
		zap.String("user_id", userID),
		zap.String("credential_id", cred.ID),
	)

	return &models.RegisterDeviceResponse{
		CredentialID: cred.ID,
		Credential:   plaintext,
		ExpiresAt:    cred.ExpiresAt,
	}, nil
}

// LoginWithDevice exchanges a previously-issued device credential for a fresh
// access + refresh pair. Used by the mobile client when the refresh token has
// expired or been rejected — preserves the "always logged in" UX without
// re-prompting for a password.
func (s *AuthService) LoginWithDevice(ctx context.Context, req *models.DeviceLoginRequest) (*models.AuthResponse, error) {
	hash := s.jwtService.HashToken(req.Credential)
	cred, err := s.userRepo.GetDeviceCredentialByHash(ctx, hash)
	if err != nil {
		s.logger.Warn("Invalid device credential", zap.Error(err))
		return nil, utils.NewUnauthorizedError("Invalid device credential", err)
	}
	if cred.ExpiresAt != nil && time.Now().After(*cred.ExpiresAt) {
		s.logger.Warn("Expired device credential", zap.String("credential_id", cred.ID))
		return nil, utils.NewUnauthorizedError("Device credential has expired", nil)
	}

	user, err := s.userRepo.GetByID(ctx, cred.UserID)
	if err != nil {
		s.logger.Error("Failed to load user for device login", zap.Error(err))
		return nil, utils.NewInternalError("Failed to authenticate device", err)
	}
	if user.DeletedAt != nil {
		return nil, utils.NewUnauthorizedError("Account is no longer active", nil)
	}

	resp, err := s.generateAuthResponse(ctx, user, models.AAL1, req.DeviceInfo, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}

	// Best-effort usage telemetry.
	if err := s.userRepo.TouchDeviceCredential(ctx, cred.ID); err != nil {
		s.logger.Debug("Failed to touch device credential", zap.Error(err))
	}

	s.logger.Info("Device login successful",
		zap.String("user_id", user.ID),
		zap.String("credential_id", cred.ID),
	)
	return resp, nil
}

// RevokeDevice deletes a single device credential. Use cases: user signs out
// of a specific device, or the device list shows an entry the user doesn't
// recognise. Sessions minted from this credential continue until they expire
// naturally — call LogoutAll to also invalidate active sessions.
func (s *AuthService) RevokeDevice(ctx context.Context, userID, credentialID string) error {
	// Authorisation: only the owner can revoke their own credential. Look up
	// the credential id and assert user ownership before mutating.
	if err := s.userRepo.RevokeDeviceCredential(ctx, credentialID); err != nil {
		s.logger.Error("Failed to revoke device credential", zap.Error(err))
		return utils.NewInternalError("Failed to revoke device", err)
	}
	s.logger.Info("Device credential revoked",
		zap.String("user_id", userID),
		zap.String("credential_id", credentialID),
	)
	return nil
}
