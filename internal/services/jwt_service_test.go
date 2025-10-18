package services

import (
	"testing"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestJWTConfig() *config.JWTConfig {
	return &config.JWTConfig{
		Secret:               "test-secret-key-at-least-32-characters-long-for-security",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
	}
}

func TestJWTService_GenerateAccessToken(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())
	userID := "user-123"
	email := "test@example.com"
	sessionID := "session-123"

	tests := []struct {
		name      string
		userID    string
		email     string
		aal       int
		sessionID string
		wantErr   bool
	}{
		{
			name:      "valid AAL1 token",
			userID:    userID,
			email:     email,
			aal:       models.AAL1,
			sessionID: sessionID,
			wantErr:   false,
		},
		{
			name:      "valid AAL2 token",
			userID:    userID,
			email:     email,
			aal:       models.AAL2,
			sessionID: sessionID,
			wantErr:   false,
		},
		{
			name:      "empty user ID",
			userID:    "",
			email:     email,
			aal:       models.AAL1,
			sessionID: sessionID,
			wantErr:   false, // JWT allows empty claims
		},
		{
			name:      "empty email",
			userID:    userID,
			email:     "",
			aal:       models.AAL1,
			sessionID: sessionID,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, expiresAt, err := service.GenerateAccessToken(tt.userID, tt.email, tt.aal, tt.sessionID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
				assert.True(t, expiresAt.After(time.Now()))
				assert.True(t, expiresAt.Before(time.Now().Add(20*time.Minute)))
			}
		})
	}
}

func TestJWTService_ValidateAccessToken(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())
	userID := "user-123"
	email := "test@example.com"
	sessionID := "session-123"
	aal := models.AAL1

	// Generate a valid token
	token, _, err := service.GenerateAccessToken(userID, email, aal, sessionID)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   token,
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "invalid token format",
			token:   "invalid.token.format",
			wantErr: true,
		},
		{
			name:    "malformed token",
			token:   "not-a-jwt-token",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateAccessToken(tt.token)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, userID, claims.UserID)
				assert.Equal(t, email, claims.Email)
				assert.Equal(t, aal, claims.AAL)
				assert.Equal(t, sessionID, claims.SessionID)
				assert.Equal(t, "hamsaya", claims.Issuer)
			}
		})
	}
}

func TestJWTService_ValidateAccessToken_WrongSecret(t *testing.T) {
	// Generate token with one secret
	service1 := NewJWTService(getTestJWTConfig())
	token, _, err := service1.GenerateAccessToken("user-123", "test@example.com", models.AAL1, "session-123")
	require.NoError(t, err)

	// Try to validate with different secret
	config2 := getTestJWTConfig()
	config2.Secret = "different-secret-key-at-least-32-characters-long-for-security"
	service2 := NewJWTService(config2)

	claims, err := service2.ValidateAccessToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWTService_GenerateRefreshToken(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())

	token1, err1 := service.GenerateRefreshToken()
	token2, err2 := service.GenerateRefreshToken()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	// Tokens should be unique
	assert.NotEqual(t, token1, token2)
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())
	userID := "user-123"
	email := "test@example.com"
	sessionID := "session-123"

	tests := []struct {
		name    string
		aal     int
		wantErr bool
	}{
		{
			name:    "AAL1 token pair",
			aal:     models.AAL1,
			wantErr: false,
		},
		{
			name:    "AAL2 token pair",
			aal:     models.AAL2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenPair, err := service.GenerateTokenPair(userID, email, tt.aal, sessionID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tokenPair)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, tokenPair)
				assert.NotEmpty(t, tokenPair.AccessToken)
				assert.NotEmpty(t, tokenPair.RefreshToken)
				assert.Equal(t, "Bearer", tokenPair.TokenType)
				assert.True(t, tokenPair.ExpiresAt.After(time.Now()))

				// Validate the access token
				claims, err := service.ValidateAccessToken(tokenPair.AccessToken)
				assert.NoError(t, err)
				assert.Equal(t, userID, claims.UserID)
				assert.Equal(t, email, claims.Email)
				assert.Equal(t, tt.aal, claims.AAL)
			}
		})
	}
}

func TestJWTService_HashToken(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "normal token",
			token: "some.jwt.token",
		},
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "long token",
			token: "very.long.token.with.lots.of.characters.that.should.still.hash.correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := service.HashToken(tt.token)
			assert.NotEmpty(t, hash)
			// Hash should be different from original
			assert.NotEqual(t, tt.token, hash)
		})
	}

	// Test that same token produces same hash
	t.Run("same token same hash", func(t *testing.T) {
		token := "test.jwt.token"
		hash1 := service.HashToken(token)
		hash2 := service.HashToken(token)
		assert.Equal(t, hash1, hash2)
	})

	// Test that different tokens produce different hashes
	t.Run("different tokens different hashes", func(t *testing.T) {
		hash1 := service.HashToken("token1")
		hash2 := service.HashToken("token2")
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestJWTService_GenerateVerificationToken(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())

	token1, err1 := service.GenerateVerificationToken()
	token2, err2 := service.GenerateVerificationToken()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	// Tokens should be unique (UUIDs)
	assert.NotEqual(t, token1, token2)
}

func TestJWTService_GeneratePasswordResetToken(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())

	token1, err1 := service.GeneratePasswordResetToken()
	token2, err2 := service.GeneratePasswordResetToken()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	// Tokens should be unique (UUIDs)
	assert.NotEqual(t, token1, token2)
}

func TestJWTService_GenerateMFAChallengeID(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())

	id1 := service.GenerateMFAChallengeID()
	id2 := service.GenerateMFAChallengeID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	// IDs should be unique (UUIDs)
	assert.NotEqual(t, id1, id2)
}

func TestJWTService_TokenExpiration(t *testing.T) {
	// Create a service with very short token duration
	cfg := getTestJWTConfig()
	cfg.AccessTokenDuration = 1 * time.Millisecond
	service := NewJWTService(cfg)

	token, _, err := service.GenerateAccessToken("user-123", "test@example.com", models.AAL1, "session-123")
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Validation should fail for expired token
	claims, err := service.ValidateAccessToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "expired")
}

func TestJWTService_AALLevels(t *testing.T) {
	service := NewJWTService(getTestJWTConfig())

	// Generate tokens with different AAL levels
	token1, _, err := service.GenerateAccessToken("user-123", "test@example.com", models.AAL1, "session-123")
	require.NoError(t, err)

	token2, _, err := service.GenerateAccessToken("user-123", "test@example.com", models.AAL2, "session-123")
	require.NoError(t, err)

	// Validate and check AAL levels
	claims1, err := service.ValidateAccessToken(token1)
	require.NoError(t, err)
	assert.Equal(t, models.AAL1, claims1.AAL)

	claims2, err := service.ValidateAccessToken(token2)
	require.NoError(t, err)
	assert.Equal(t, models.AAL2, claims2.AAL)
}
