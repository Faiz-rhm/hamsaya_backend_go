package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/utils"
)

// JWTService handles JWT token operations
type JWTService struct {
	cfg *config.JWTConfig
}

// NewJWTService creates a new JWT service
func NewJWTService(cfg *config.JWTConfig) *JWTService {
	return &JWTService{
		cfg: cfg,
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (s *JWTService) GenerateTokenPair(userID, email string, aal int, sessionID string) (*models.TokenPair, error) {
	// Generate access token
	accessToken, expiresAt, err := s.GenerateAccessToken(userID, email, aal, sessionID)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken, err := s.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, nil
}

// GenerateAccessToken generates a new JWT access token
func (s *JWTService) GenerateAccessToken(userID, email string, aal int, sessionID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.cfg.AccessTokenDuration)

	claims := jwt.MapClaims{
		"user_id":    userID,
		"email":      email,
		"aal":        aal,
		"session_id": sessionID,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"iss":        "hamsaya",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, expiresAt, nil
}

// GenerateRefreshToken generates a cryptographically secure refresh token
func (s *JWTService) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ValidateAccessToken validates and parses an access token
func (s *JWTService) ValidateAccessToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})

	if err != nil {
		return nil, utils.NewUnauthorizedError("Invalid token", err)
	}

	if !token.Valid {
		return nil, utils.NewUnauthorizedError("Token is not valid", nil)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token claims", nil)
	}

	// Extract claims
	jwtClaims := &models.JWTClaims{
		UserID:    claims["user_id"].(string),
		Email:     claims["email"].(string),
		AAL:       int(claims["aal"].(float64)),
		SessionID: claims["session_id"].(string),
		IssuedAt:  int64(claims["iat"].(float64)),
		ExpiresAt: int64(claims["exp"].(float64)),
		Issuer:    claims["iss"].(string),
	}

	// Verify not expired
	if time.Now().Unix() > jwtClaims.ExpiresAt {
		return nil, utils.NewUnauthorizedError("Token has expired", nil)
	}

	return jwtClaims, nil
}

// HashToken creates a SHA-256 hash of a token for storage
func (s *JWTService) HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// GenerateVerificationToken generates a token for email verification
func (s *JWTService) GenerateVerificationToken() (string, error) {
	return uuid.New().String(), nil
}

// GeneratePasswordResetToken generates a token for password reset
func (s *JWTService) GeneratePasswordResetToken() (string, error) {
	return uuid.New().String(), nil
}

// GenerateMFAChallengeID generates a unique ID for MFA challenges
func (s *JWTService) GenerateMFAChallengeID() string {
	return uuid.New().String()
}
