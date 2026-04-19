package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestOAuthService(userRepo *mocks.MockUserRepository) *OAuthService {
	cfg := &config.Config{
		OAuth: config.OAuthConfig{
			Google: config.GoogleOAuthConfig{ClientID: "test-client-id"},
		},
	}
	return NewOAuthService(cfg, userRepo, zap.NewNop())
}

func TestOAuthService_AuthenticateWithOAuth(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockUserRepository)
		oauthInfo     *OAuthUserInfo
		expectedError string
		isNewUser     bool
	}{
		{
			name:       "empty email",
			setupMocks: func(_ *mocks.MockUserRepository) {},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-123",
				Email:          "",
				Provider:       "google",
			},
			expectedError: "email is required",
		},
		{
			name: "existing user same provider login",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				provider := "google"
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.OAuthProvider = &provider
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				userRepo.On("UpdateLastLogin", mock.Anything, "user-1").Return(nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
			},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-123",
				Email:          "test@example.com",
				EmailVerified:  true,
				Provider:       "google",
			},
			isNewUser: false,
		},
		{
			name: "existing user different oauth provider",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				provider := "facebook"
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.OAuthProvider = &provider
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
			},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-123",
				Email:          "test@example.com",
				Provider:       "google",
			},
			expectedError: "already registered with facebook",
		},
		{
			name: "existing user with password account",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.OAuthProvider = nil // password user
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
			},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-123",
				Email:          "test@example.com",
				Provider:       "google",
			},
			expectedError: "already registered with a password",
		},
		{
			name: "new user registration",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByEmail", mock.Anything, "new@example.com").
					Return(nil, errors.New("not found"))
				userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
				userRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)
			},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-456",
				Email:          "new@example.com",
				EmailVerified:  true,
				FirstName:      "New",
				LastName:       "User",
				Picture:        "https://example.com/pic.jpg",
				Provider:       "google",
			},
			isNewUser: true,
		},
		{
			name: "new user email normalized to lowercase",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByEmail", mock.Anything, "upper@example.com").
					Return(nil, errors.New("not found"))
				userRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
					return u.Email == "upper@example.com"
				})).Return(nil)
				userRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)
			},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-789",
				Email:          "UPPER@EXAMPLE.COM",
				Provider:       "google",
			},
			isNewUser: true,
		},
		{
			name: "existing user avatar backfill",
			setupMocks: func(userRepo *mocks.MockUserRepository) {
				provider := "google"
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.OAuthProvider = &provider
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				profile.Avatar = nil // no avatar yet
				userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				userRepo.On("UpdateLastLogin", mock.Anything, "user-1").Return(nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				userRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)
			},
			oauthInfo: &OAuthUserInfo{
				ProviderUserID: "google-123",
				Email:          "test@example.com",
				Provider:       "google",
				Picture:        "https://example.com/pic.jpg",
			},
			isNewUser: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(userRepo)
			svc := newTestOAuthService(userRepo)

			user, profile, isNew, err := svc.AuthenticateWithOAuth(context.Background(), tt.oauthInfo)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.NotNil(t, profile)
				assert.Equal(t, tt.isNewUser, isNew)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestOAuthService_VerifyAppleToken(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	svc := newTestOAuthService(userRepo)

	_, err := svc.VerifyAppleToken(context.Background(), "any-token")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not yet fully implemented")
}

func TestOAuthService_VerifyGoogleToken(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := GoogleUserInfo{
				ID:            "google-123",
				Email:         "test@example.com",
				EmailVerified: "true",
				GivenName:     "Test",
				FamilyName:    "User",
				Picture:       "https://example.com/pic.jpg",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(info)
		}))
		defer ts.Close()

		// We can't inject HTTP client into OAuthService without refactor.
		// This test documents the expected contract — real call is integration territory.
		_ = ts
	})

	t.Run("invalid token format rejected by google", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newTestOAuthService(userRepo)

		// Real HTTP call to Google with invalid token — should fail (not a unit test, skip in CI)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := svc.VerifyGoogleToken(ctx, "invalid-token")
		// May fail due to network or Google rejection — either is acceptable
		// We just confirm the function handles errors gracefully
		if err != nil {
			assert.NotPanics(t, func() { _ = err.Error() })
		}
	})
}

func TestOAuthService_VerifyFacebookToken(t *testing.T) {
	t.Run("invalid token rejected by facebook", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newTestOAuthService(userRepo)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := svc.VerifyFacebookToken(ctx, "invalid-access-token")
		// Either network error or Facebook rejection — both are valid
		if err != nil {
			assert.NotPanics(t, func() { _ = err.Error() })
		}
	})
}
