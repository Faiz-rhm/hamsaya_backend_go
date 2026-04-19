package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testPasswordHash is a bcrypt hash for the string "password" (cost=12).
const testPasswordHash = "$2a$12$SK7HMTw9slXUVmPZtdMa6evdMIN5CBUFvQfwOBbLgcb.Tt8Bi9UpK"

// testStrongPasswordHash is a bcrypt hash for "CurrentPass1!" (cost=12).
// Used for tests that need a password that passes strength validation.
const testStrongPasswordHash = "$2a$12$9aomQGUJj.I5.pUMGQV0OuUpRy1Cz6j1TE6tuS3kFNLjwkZkRF5Gm"

func getTestConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:               "test-secret-key-at-least-32-characters-long-for-security",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenDuration: 7 * 24 * time.Hour,
		},
	}
}

func newTestTokenStorage(t *testing.T) (*TokenStorageService, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewTokenStorageService(rdb, zap.NewNop()), mr
}

func newFailingTokenStorage() *TokenStorageService {
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:0",
		MaxRetries:  0,
		DialTimeout: time.Millisecond,
	})
	return NewTokenStorageService(rdb, zap.NewNop())
}

func newTestAuthService(userRepo *mocks.MockUserRepository, tokenStorage *TokenStorageService) *AuthService {
	cfg := getTestConfig()
	jwtSvc := NewJWTService(&cfg.JWT)
	passwordSvc := NewPasswordService()
	emailSvc := NewEmailService(&config.EmailConfig{}, zap.NewNop())
	return NewAuthService(userRepo, passwordSvc, jwtSvc, emailSvc, tokenStorage, nil, cfg, zap.NewNop())
}

func TestAuthService_Login(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockUserRepository, *TokenStorageService)
		tokenStorage  func(t *testing.T) (*TokenStorageService, func())
		request       *models.LoginRequest
		expectedError string
		checkResponse func(*testing.T, *models.AuthResponse)
	}{
		{
			name: "account locked",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *TokenStorageService) {
				lockTime := time.Now().Add(30 * time.Minute)
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.LockedUntil = &lockTime
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
			},
			tokenStorage: func(t *testing.T) (*TokenStorageService, func()) {
				ts := newFailingTokenStorage()
				return ts, func() {}
			},
			request:       &models.LoginRequest{Email: "test@example.com", Password: "password"},
			expectedError: "locked",
		},
		{
			name: "wrong password",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *TokenStorageService) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				userRepo.On("UpdateLoginAttempts", mock.Anything, "user-1", 1, (*time.Time)(nil)).Return(nil)
			},
			tokenStorage: func(t *testing.T) (*TokenStorageService, func()) {
				ts := newFailingTokenStorage()
				return ts, func() {}
			},
			request:       &models.LoginRequest{Email: "test@example.com", Password: "wrongpassword"},
			expectedError: "invalid email or password",
		},
		{
			name: "account locks after max attempts",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *TokenStorageService) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.FailedLoginAttempts = 4
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				userRepo.On("UpdateLoginAttempts", mock.Anything, "user-1", 5, mock.MatchedBy(func(t *time.Time) bool {
					return t != nil
				})).Return(nil)
			},
			tokenStorage: func(t *testing.T) (*TokenStorageService, func()) {
				ts := newFailingTokenStorage()
				return ts, func() {}
			},
			request:       &models.LoginRequest{Email: "test@example.com", Password: "wrongpassword"},
			expectedError: "invalid email or password",
		},
		{
			name: "MFA required",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *TokenStorageService) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.MFAEnabled = true
				user.PasswordHash = func() *string { s := testPasswordHash; return &s }()
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
			},
			tokenStorage: func(t *testing.T) (*TokenStorageService, func()) {
				ts, mr := newTestTokenStorage(t)
				return ts, func() { mr.Close() }
			},
			request:       &models.LoginRequest{Email: "test@example.com", Password: "password"},
			expectedError: "",
			checkResponse: func(t *testing.T, resp *models.AuthResponse) {
				require.NotNil(t, resp)
				assert.True(t, resp.RequiresMFA)
				assert.NotNil(t, resp.MFAChallengeID)
				assert.NotEmpty(t, *resp.MFAChallengeID)
			},
		},
		{
			name: "successful login",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *TokenStorageService) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.PasswordHash = func() *string { s := testPasswordHash; return &s }()
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				userRepo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.UserSession")).Return(nil)
				userRepo.On("UpdateLastLogin", mock.Anything, "user-1").Return(nil)
			},
			tokenStorage: func(t *testing.T) (*TokenStorageService, func()) {
				ts := newFailingTokenStorage()
				return ts, func() {}
			},
			request:       &models.LoginRequest{Email: "test@example.com", Password: "password"},
			expectedError: "",
			checkResponse: func(t *testing.T, resp *models.AuthResponse) {
				require.NotNil(t, resp)
				assert.NotNil(t, resp.User)
				assert.NotNil(t, resp.Tokens)
				assert.NotEmpty(t, resp.Tokens.AccessToken)
				assert.NotEmpty(t, resp.Tokens.RefreshToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			ts, cleanup := tt.tokenStorage(t)
			defer cleanup()

			tt.setupMocks(userRepo, ts)

			svc := newTestAuthService(userRepo, ts)
			resp, err := svc.Login(context.Background(), tt.request)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockUserRepository)
		request       *models.RegisterRequest
		expectedError string
		checkResponse func(*testing.T, *models.AuthResponse)
	}{
		{
			name: "email already exists",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				existing := testutil.CreateTestUser("user-1", "test@example.com")
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(existing, nil)
			},
			request: &models.RegisterRequest{
				Email:     "test@example.com",
				Password:  "Password1!",
				FirstName: "Test",
				LastName:  "User",
				Latitude:  34.5,
				Longitude: 69.2,
			},
			expectedError: "already exists",
		},
		{
			name: "soft-deleted email",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByEmail", mock.Anything, "deleted@example.com").Return(nil, errors.New("not found"))
				deleted := testutil.CreateTestUser("user-2", "deleted@example.com")
				now := time.Now()
				deleted.DeletedAt = &now
				userRepo.On("GetByEmailIncludingDeleted", mock.Anything, "deleted@example.com").Return(deleted, nil)
			},
			request: &models.RegisterRequest{
				Email:     "deleted@example.com",
				Password:  "Password1!",
				FirstName: "Test",
				LastName:  "User",
				Latitude:  34.5,
				Longitude: 69.2,
			},
			expectedError: "no longer available",
		},
		{
			name: "weak password",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByEmail", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
				userRepo.On("GetByEmailIncludingDeleted", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
			},
			request: &models.RegisterRequest{
				Email:     "new@example.com",
				Password:  "weak",
				FirstName: "Test",
				LastName:  "User",
				Latitude:  34.5,
				Longitude: 69.2,
			},
			expectedError: "password",
		},
		{
			name: "successful registration",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByEmail", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
				userRepo.On("GetByEmailIncludingDeleted", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
				userRepo.On("CreateUserWithProfile", mock.Anything, mock.AnythingOfType("*models.User"), mock.AnythingOfType("*models.Profile")).Return(nil)
				profile := testutil.CreateTestProfile("any-id", "Test", "User")
				userRepo.On("GetProfileByUserID", mock.Anything, mock.Anything).Return(profile, nil)
				userRepo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.UserSession")).Return(nil)
				userRepo.On("UpdateLastLogin", mock.Anything, mock.Anything).Return(nil)
			},
			request: &models.RegisterRequest{
				Email:     "new@example.com",
				Password:  "StrongPass1!",
				FirstName: "Test",
				LastName:  "User",
				Latitude:  34.5,
				Longitude: 69.2,
			},
			expectedError: "",
			checkResponse: func(t *testing.T, resp *models.AuthResponse) {
				require.NotNil(t, resp)
				assert.NotNil(t, resp.Tokens)
				assert.NotEmpty(t, resp.Tokens.AccessToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(userRepo)

			ts := newFailingTokenStorage()
			svc := newTestAuthService(userRepo, ts)

			resp, err := svc.Register(context.Background(), tt.request)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	cfg := getTestConfig()
	jwtSvc := NewJWTService(&cfg.JWT)

	validRefreshToken, err := jwtSvc.GenerateRefreshToken()
	require.NoError(t, err)
	validTokenHash := jwtSvc.HashToken(validRefreshToken)

	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockUserRepository)
		request       *models.RefreshTokenRequest
		expectedError string
		checkResponse func(*testing.T, *models.TokenPair)
	}{
		{
			name: "invalid refresh token",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("GetSessionByRefreshTokenHash", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
				userRepo.On("GetSessionByRefreshToken", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
			},
			request:       &models.RefreshTokenRequest{RefreshToken: "invalid-token"},
			expectedError: "not found",
		},
		{
			name: "expired session",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				session := &models.UserSession{
					ID:               "session-1",
					UserID:           "user-1",
					RefreshToken:     validRefreshToken,
					RefreshTokenHash: validTokenHash,
					ExpiresAt:        time.Now().Add(-1 * time.Hour),
					Revoked:          false,
				}
				userRepo.On("GetSessionByRefreshTokenHash", mock.Anything, mock.Anything).Return(session, nil)
			},
			request:       &models.RefreshTokenRequest{RefreshToken: validRefreshToken},
			expectedError: "expired",
		},
		{
			name: "revoked session",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				session := &models.UserSession{
					ID:               "session-1",
					UserID:           "user-1",
					RefreshToken:     validRefreshToken,
					RefreshTokenHash: validTokenHash,
					ExpiresAt:        time.Now().Add(1 * time.Hour),
					Revoked:          true,
				}
				userRepo.On("GetSessionByRefreshTokenHash", mock.Anything, mock.Anything).Return(session, nil)
			},
			request:       &models.RefreshTokenRequest{RefreshToken: validRefreshToken},
			expectedError: "revoked",
		},
		{
			name: "successful refresh",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				session := &models.UserSession{
					ID:               "session-1",
					UserID:           "user-1",
					RefreshToken:     validRefreshToken,
					RefreshTokenHash: validTokenHash,
					ExpiresAt:        time.Now().Add(1 * time.Hour),
					Revoked:          false,
				}
				user := testutil.CreateTestUser("user-1", "test@example.com")
				userRepo.On("GetSessionByRefreshTokenHash", mock.Anything, mock.Anything).Return(session, nil)
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("RevokeSession", mock.Anything, "session-1").Return(nil)
				userRepo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.UserSession")).Return(nil)
			},
			request:       &models.RefreshTokenRequest{RefreshToken: validRefreshToken},
			expectedError: "",
			checkResponse: func(t *testing.T, tokens *models.TokenPair) {
				require.NotNil(t, tokens)
				assert.NotEmpty(t, tokens.AccessToken)
				assert.NotEmpty(t, tokens.RefreshToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(userRepo)

			ts := newFailingTokenStorage()
			svc := newTestAuthService(userRepo, ts)

			tokens, err := svc.RefreshToken(context.Background(), tt.request)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				if tt.checkResponse != nil {
					tt.checkResponse(t, tokens)
				}
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	tests := []struct {
		name          string
		sessionID     string
		setupMocks    func(*mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:      "successful logout",
			sessionID: "session-1",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("RevokeSession", mock.Anything, "session-1").Return(nil)
			},
			expectedError: "",
		},
		{
			name:      "logout fails",
			sessionID: "session-bad",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("RevokeSession", mock.Anything, "session-bad").Return(errors.New("db error"))
			},
			expectedError: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(userRepo)

			ts := newFailingTokenStorage()
			svc := newTestAuthService(userRepo, ts)

			err := svc.Logout(context.Background(), tt.sessionID)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_LogoutAll(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		setupMocks    func(*mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:   "successful logout all",
			userID: "user-1",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("RevokeAllUserSessions", mock.Anything, "user-1").Return(nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(userRepo)

			ts := newFailingTokenStorage()
			svc := newTestAuthService(userRepo, ts)

			err := svc.LogoutAll(context.Background(), tt.userID)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		sessionID     string
		setupMocks    func(*mocks.MockUserRepository)
		request       *models.ChangePasswordRequest
		expectedError string
	}{
		{
			name:      "wrong current password",
			userID:    "user-1",
			sessionID: "session-1",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.PasswordHash = func() *string { s := testPasswordHash; return &s }()
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
			},
			request: &models.ChangePasswordRequest{
				CurrentPassword: "wrongpassword",
				NewPassword:     "NewPassword1!",
			},
			expectedError: "incorrect",
		},
		{
			name:      "same as current",
			userID:    "user-1",
			sessionID: "session-1",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.PasswordHash = func() *string { s := testStrongPasswordHash; return &s }()
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
			},
			request: &models.ChangePasswordRequest{
				CurrentPassword: "CurrentPass1!",
				NewPassword:     "CurrentPass1!",
			},
			expectedError: "different",
		},
		{
			name:      "successful change",
			userID:    "user-1",
			sessionID: "session-1",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.PasswordHash = func() *string { s := testPasswordHash; return &s }()
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
				userRepo.On("RevokeAllUserSessionsExcept", mock.Anything, "user-1", "session-1").Return(nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
			},
			request: &models.ChangePasswordRequest{
				CurrentPassword: "password",
				NewPassword:     "NewPassword1!",
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(userRepo)

			ts := newFailingTokenStorage()
			svc := newTestAuthService(userRepo, ts)

			err := svc.ChangePassword(context.Background(), tt.userID, tt.sessionID, tt.request)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_VerifyEmail(t *testing.T) {
	t.Run("invalid token", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		ts := newFailingTokenStorage()
		svc := newTestAuthService(userRepo, ts)

		err := svc.VerifyEmail(context.Background(), &models.VerifyEmailRequest{Token: "invalid-token"})

		require.Error(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("already verified", func(t *testing.T) {
		ts, mr := newTestTokenStorage(t)
		defer mr.Close()

		userRepo := new(mocks.MockUserRepository)

		verificationCode := "123456"
		ctx := context.Background()
		err := ts.StoreVerificationToken(ctx, "user-1", verificationCode, 1*time.Hour)
		require.NoError(t, err)

		user := testutil.CreateTestUser("user-1", "test@example.com")
		user.EmailVerified = true
		userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)

		svc := newTestAuthService(userRepo, ts)
		err = svc.VerifyEmail(ctx, &models.VerifyEmailRequest{Token: verificationCode})

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("successful verification", func(t *testing.T) {
		ts, mr := newTestTokenStorage(t)
		defer mr.Close()

		userRepo := new(mocks.MockUserRepository)

		verificationCode := "654321"
		ctx := context.Background()
		err := ts.StoreVerificationToken(ctx, "user-2", verificationCode, 1*time.Hour)
		require.NoError(t, err)

		user := testutil.CreateTestUser("user-2", "verify@example.com")
		user.EmailVerified = false
		profile := testutil.CreateTestProfile("user-2", "Verify", "User")

		userRepo.On("GetByID", mock.Anything, "user-2").Return(user, nil)
		userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
		userRepo.On("GetProfileByUserID", mock.Anything, "user-2").Return(profile, nil)

		svc := newTestAuthService(userRepo, ts)
		err = svc.VerifyEmail(ctx, &models.VerifyEmailRequest{Token: verificationCode})

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestAuthService_ForgotPassword(t *testing.T) {
	t.Run("user not found", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		userRepo.On("GetByEmail", mock.Anything, "unknown@example.com").Return(nil, errors.New("not found"))

		ts := newFailingTokenStorage()
		svc := newTestAuthService(userRepo, ts)

		err := svc.ForgotPassword(context.Background(), &models.ForgotPasswordRequest{Email: "unknown@example.com"})

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestAuthService_ResetPassword(t *testing.T) {
	t.Run("invalid token", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		ts := newFailingTokenStorage()
		svc := newTestAuthService(userRepo, ts)

		err := svc.ResetPassword(context.Background(), &models.ResetPasswordRequest{
			Token:       "invalid-token",
			NewPassword: "NewPassword1!",
		})

		require.Error(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("weak new password", func(t *testing.T) {
		ts, mr := newTestTokenStorage(t)
		defer mr.Close()

		userRepo := new(mocks.MockUserRepository)

		resetToken := "reset123"
		ctx := context.Background()
		err := ts.StorePasswordResetToken(ctx, "user-1", resetToken, 1*time.Hour)
		require.NoError(t, err)

		user := testutil.CreateTestUser("user-1", "test@example.com")
		userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)

		svc := newTestAuthService(userRepo, ts)
		err = svc.ResetPassword(ctx, &models.ResetPasswordRequest{
			Token:       resetToken,
			NewPassword: "weak",
		})

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "password")
		userRepo.AssertExpectations(t)
	})

	t.Run("successful reset", func(t *testing.T) {
		ts, mr := newTestTokenStorage(t)
		defer mr.Close()

		userRepo := new(mocks.MockUserRepository)

		resetToken := "validreset"
		ctx := context.Background()
		err := ts.StorePasswordResetToken(ctx, "user-3", resetToken, 1*time.Hour)
		require.NoError(t, err)

		user := testutil.CreateTestUser("user-3", "reset@example.com")
		profile := testutil.CreateTestProfile("user-3", "Reset", "User")

		userRepo.On("GetByID", mock.Anything, "user-3").Return(user, nil)
		userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
		userRepo.On("RevokeAllUserSessions", mock.Anything, "user-3").Return(nil)
		userRepo.On("GetProfileByUserID", mock.Anything, "user-3").Return(profile, nil)

		svc := newTestAuthService(userRepo, ts)
		err = svc.ResetPassword(ctx, &models.ResetPasswordRequest{
			Token:       resetToken,
			NewPassword: "NewStrongPass1!",
		})

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestAuthService_UnifiedAuth_ExistingUser(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := NewTokenStorageService(rdb, zap.NewNop())

	t.Run("locked account", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		locked := time.Now().Add(1 * time.Hour)
		user := testutil.CreateTestUser("u-1", "test@example.com")
		user.LockedUntil = &locked
		userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)

		svc := newTestAuthService(userRepo, tokenStorage)
		_, err := svc.UnifiedAuth(context.Background(), &models.UnifiedAuthRequest{
			Email: "test@example.com", Password: "password",
		})
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "locked")
	})

	t.Run("wrong password", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		user := testutil.CreateTestUser("u-1", "test@example.com")
		hash := testPasswordHash
		user.PasswordHash = &hash
		userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
		userRepo.On("UpdateLoginAttempts", mock.Anything, "u-1", mock.Anything, mock.Anything).Return(nil)

		svc := newTestAuthService(userRepo, tokenStorage)
		_, err := svc.UnifiedAuth(context.Background(), &models.UnifiedAuthRequest{
			Email: "test@example.com", Password: "wrongpassword",
		})
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "invalid")
	})
}

func TestAuthService_UnifiedAuth_NewUser(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := NewTokenStorageService(rdb, zap.NewNop())

	t.Run("missing first name", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		userRepo.On("GetByEmail", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
		userRepo.On("GetByEmailIncludingDeleted", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))

		svc := newTestAuthService(userRepo, tokenStorage)
		_, err := svc.UnifiedAuth(context.Background(), &models.UnifiedAuthRequest{
			Email:    "new@example.com",
			Password: "StrongPass1!",
		})
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "first_name")
	})
}

func TestAuthService_VerifyMFA(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := NewTokenStorageService(rdb, zap.NewNop())

	t.Run("invalid challenge", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		mfaRepo := &mocks.MockMFARepository{}
		mfaSvc := NewMFAService(mfaRepo, userRepo, NewPasswordService(), zap.NewNop())
		cfg := getTestConfig()
		svc := NewAuthService(userRepo, NewPasswordService(), NewJWTService(&cfg.JWT), nil, tokenStorage, mfaSvc, cfg, zap.NewNop())

		_, err := svc.VerifyMFA(context.Background(), &models.MFAVerifyChallengeRequest{
			ChallengeID: "bad-challenge", Code: "123456",
		})
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "invalid")
	})
}

func TestAuthService_SendVerificationEmailForUser(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := NewTokenStorageService(rdb, zap.NewNop())

	t.Run("user not found", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		userRepo.On("GetByID", mock.Anything, "u-bad").Return(nil, errors.New("not found"))

		svc := newTestAuthService(userRepo, tokenStorage)
		err := svc.SendVerificationEmailForUser(context.Background(), "u-bad")
		require.Error(t, err)
	})

	t.Run("already verified — no-op", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		user := testutil.CreateTestUser("u-1", "test@example.com")
		user.EmailVerified = true
		userRepo.On("GetByID", mock.Anything, "u-1").Return(user, nil)

		svc := newTestAuthService(userRepo, tokenStorage)
		err := svc.SendVerificationEmailForUser(context.Background(), "u-1")
		require.NoError(t, err)
	})
}

func TestAuthService_VerifyResetCode(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := NewTokenStorageService(rdb, zap.NewNop())

	t.Run("invalid code", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newTestAuthService(userRepo, tokenStorage)
		err := svc.VerifyResetCode(context.Background(), &models.VerifyResetCodeRequest{Token: "bad-token"})
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "invalid")
	})

	t.Run("valid code", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		// Store a real token first
		_ = tokenStorage.StorePasswordResetToken(context.Background(), "u-1", "valid-token", 5*time.Minute)
		svc := newTestAuthService(userRepo, tokenStorage)
		err := svc.VerifyResetCode(context.Background(), &models.VerifyResetCodeRequest{Token: "valid-token"})
		require.NoError(t, err)
	})
}

func TestAuthService_GetActiveSessions(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := NewTokenStorageService(rdb, zap.NewNop())

	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		sessions := []*models.UserSession{{ID: "s-1", UserID: "u-1"}}
		userRepo.On("GetActiveSessions", mock.Anything, "u-1").Return(sessions, nil)

		svc := newTestAuthService(userRepo, tokenStorage)
		result, err := svc.GetActiveSessions(context.Background(), "u-1")
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("repo error", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		userRepo.On("GetActiveSessions", mock.Anything, "u-1").Return(nil, errors.New("db error"))

		svc := newTestAuthService(userRepo, tokenStorage)
		_, err := svc.GetActiveSessions(context.Background(), "u-1")
		require.Error(t, err)
	})
}
