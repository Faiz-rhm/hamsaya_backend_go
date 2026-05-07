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

// GenerateAccessToken generates a new JWT access token. Each token includes
// a unique JTI so it can be individually revoked via the access-token denylist.
func (s *JWTService) GenerateAccessToken(userID, email string, aal int, sessionID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.cfg.AccessTokenDuration)
	jti := uuid.New().String()

	claims := jwt.MapClaims{
		"user_id":    userID,
		"email":      email,
		"aal":        aal,
		"session_id": sessionID,
		"jti":        jti,
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
		// Explicitly require HS256 — reject none, RS256, and other algorithms
		// to prevent algorithm confusion attacks (e.g. alg:none, RS256 with HMAC key)
		if token.Method != jwt.SigningMethodHS256 {
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

	// Extract claims with safe `, ok` casts. A crafted token (correctly
	// signed but with malformed claim types) would otherwise panic on the
	// raw `.(string)` / `.(float64)` assertion → 500 DoS via gin recovery.
	// JTI is optional for backwards compatibility — empty JTI = "cannot be
	// denylisted", middleware skips that check.
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return nil, utils.NewUnauthorizedError("Invalid token: missing user_id", nil)
	}
	email, ok := claims["email"].(string)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token: missing email", nil)
	}
	aalFloat, ok := claims["aal"].(float64)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token: missing aal", nil)
	}
	sessionID, ok := claims["session_id"].(string)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token: missing session_id", nil)
	}
	iatFloat, ok := claims["iat"].(float64)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token: missing iat", nil)
	}
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token: missing exp", nil)
	}
	iss, ok := claims["iss"].(string)
	if !ok {
		return nil, utils.NewUnauthorizedError("Invalid token: missing iss", nil)
	}
	jti, _ := claims["jti"].(string)
	jwtClaims := &models.JWTClaims{
		UserID:    userID,
		Email:     email,
		AAL:       int(aalFloat),
		SessionID: sessionID,
		JTI:       jti,
		IssuedAt:  int64(iatFloat),
		ExpiresAt: int64(expFloat),
		Issuer:    iss,
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

// GenerateVerificationToken generates a token for email verification (legacy/link flow)
func (s *JWTService) GenerateVerificationToken() (string, error) {
	return uuid.New().String(), nil
}

// GenerateVerificationCode generates a 6-digit numeric code for email verification (entered in app)
func (s *JWTService) GenerateVerificationCode() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}
	// 0–999999 with uniform distribution
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	if n < 0 {
		n = -n
	}
	code := n % 1000000
	return fmt.Sprintf("%06d", code), nil
}

// GeneratePasswordResetToken generates a token for password reset
func (s *JWTService) GeneratePasswordResetToken() (string, error) {
	return uuid.New().String(), nil
}

// GenerateMFAChallengeID generates a unique ID for MFA challenges
func (s *JWTService) GenerateMFAChallengeID() string {
	return uuid.New().String()
}
