package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TokenStorageService handles storing and retrieving tokens in Redis
type TokenStorageService struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewTokenStorageService creates a new token storage service
func NewTokenStorageService(redisClient *redis.Client, logger *zap.Logger) *TokenStorageService {
	return &TokenStorageService{
		redis:  redisClient,
		logger: logger,
	}
}

// StoreVerificationToken stores an email verification token
func (s *TokenStorageService) StoreVerificationToken(ctx context.Context, userID, token string, ttl time.Duration) error {
	key := fmt.Sprintf("verify:email:%s", token)
	err := s.redis.Set(ctx, key, userID, ttl).Err()
	if err != nil {
		s.logger.Error("Failed to store verification token",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to store verification token: %w", err)
	}

	s.logger.Info("Verification token stored",
		zap.String("user_id", userID),
		zap.Duration("ttl", ttl),
	)
	return nil
}

// GetUserIDFromVerificationToken retrieves user ID from verification token
func (s *TokenStorageService) GetUserIDFromVerificationToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("verify:email:%s", token)
	userID, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("verification token not found or expired")
	}
	if err != nil {
		s.logger.Error("Failed to get verification token",
			zap.String("token", token),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to get verification token: %w", err)
	}

	return userID, nil
}

// DeleteVerificationToken removes a verification token
func (s *TokenStorageService) DeleteVerificationToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("verify:email:%s", token)
	err := s.redis.Del(ctx, key).Err()
	if err != nil {
		s.logger.Error("Failed to delete verification token",
			zap.String("token", token),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete verification token: %w", err)
	}

	return nil
}

// StorePasswordResetToken stores a password reset token
func (s *TokenStorageService) StorePasswordResetToken(ctx context.Context, userID, token string, ttl time.Duration) error {
	key := fmt.Sprintf("reset:password:%s", token)
	err := s.redis.Set(ctx, key, userID, ttl).Err()
	if err != nil {
		s.logger.Error("Failed to store password reset token",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to store password reset token: %w", err)
	}

	s.logger.Info("Password reset token stored",
		zap.String("user_id", userID),
		zap.Duration("ttl", ttl),
	)
	return nil
}

// GetUserIDFromPasswordResetToken retrieves user ID from password reset token
func (s *TokenStorageService) GetUserIDFromPasswordResetToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("reset:password:%s", token)
	userID, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("password reset token not found or expired")
	}
	if err != nil {
		s.logger.Error("Failed to get password reset token",
			zap.String("token", token),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to get password reset token: %w", err)
	}

	return userID, nil
}

// DeletePasswordResetToken removes a password reset token
func (s *TokenStorageService) DeletePasswordResetToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("reset:password:%s", token)
	err := s.redis.Del(ctx, key).Err()
	if err != nil {
		s.logger.Error("Failed to delete password reset token",
			zap.String("token", token),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete password reset token: %w", err)
	}

	return nil
}

// StoreMFAChallenge stores an MFA challenge
func (s *TokenStorageService) StoreMFAChallenge(ctx context.Context, challengeID, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("mfa:challenge:%s", challengeID)
	err := s.redis.Set(ctx, key, userID, ttl).Err()
	if err != nil {
		s.logger.Error("Failed to store MFA challenge",
			zap.String("challenge_id", challengeID),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to store MFA challenge: %w", err)
	}

	s.logger.Info("MFA challenge stored",
		zap.String("challenge_id", challengeID),
		zap.String("user_id", userID),
		zap.Duration("ttl", ttl),
	)
	return nil
}

// GetUserIDFromMFAChallenge retrieves user ID from MFA challenge
func (s *TokenStorageService) GetUserIDFromMFAChallenge(ctx context.Context, challengeID string) (string, error) {
	key := fmt.Sprintf("mfa:challenge:%s", challengeID)
	userID, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("MFA challenge not found or expired")
	}
	if err != nil {
		s.logger.Error("Failed to get MFA challenge",
			zap.String("challenge_id", challengeID),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to get MFA challenge: %w", err)
	}

	return userID, nil
}

// DeleteMFAChallenge removes an MFA challenge
func (s *TokenStorageService) DeleteMFAChallenge(ctx context.Context, challengeID string) error {
	key := fmt.Sprintf("mfa:challenge:%s", challengeID)
	err := s.redis.Del(ctx, key).Err()
	if err != nil {
		s.logger.Error("Failed to delete MFA challenge",
			zap.String("challenge_id", challengeID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete MFA challenge: %w", err)
	}

	return nil
}

// BlacklistToken adds a token to the blacklist (for revoked access tokens)
func (s *TokenStorageService) BlacklistToken(ctx context.Context, tokenHash string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:token:%s", tokenHash)
	err := s.redis.Set(ctx, key, "1", ttl).Err()
	if err != nil {
		s.logger.Error("Failed to blacklist token",
			zap.String("token_hash", tokenHash),
			zap.Error(err),
		)
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	s.logger.Info("Token blacklisted",
		zap.String("token_hash", tokenHash),
		zap.Duration("ttl", ttl),
	)
	return nil
}

// IsTokenBlacklisted checks if a token is blacklisted
func (s *TokenStorageService) IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", tokenHash)
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		s.logger.Error("Failed to check if token is blacklisted",
			zap.String("token_hash", tokenHash),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to check token blacklist: %w", err)
	}

	return exists > 0, nil
}

// Session cache constants
const (
	sessionCachePrefix = "session:cache:"
	sessionCacheTTL    = 60 * time.Second // Cache sessions for 60 seconds
)

// CacheSession stores session data in Redis cache to avoid repeated DB lookups.
// The cache has a short TTL so revocations take effect within a minute.
func (s *TokenStorageService) CacheSession(ctx context.Context, sessionID string, data *SessionCacheData) error {
	key := sessionCachePrefix + sessionID
	value := fmt.Sprintf("%s|%s|%t|%d",
		data.UserID, data.AccessTokenHash, data.Revoked, data.ExpiresAt.Unix())

	if err := s.redis.Set(ctx, key, value, sessionCacheTTL).Err(); err != nil {
		// Cache failure is not critical, just log it
		s.logger.Debug("Failed to cache session", zap.String("session_id", sessionID), zap.Error(err))
		return err
	}
	return nil
}

// GetCachedSession retrieves cached session data from Redis.
// Returns nil if the session is not in cache (cache miss).
func (s *TokenStorageService) GetCachedSession(ctx context.Context, sessionID string) (*SessionCacheData, error) {
	key := sessionCachePrefix + sessionID
	value, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	data := &SessionCacheData{}
	var expiresUnix int64
	_, err = fmt.Sscanf(value, "%s|%s|%t|%d",
		&data.UserID, &data.AccessTokenHash, &data.Revoked, &expiresUnix)
	if err != nil {
		// Parse error — treat as cache miss, delete bad entry
		s.redis.Del(ctx, key)
		return nil, nil
	}
	data.ExpiresAt = time.Unix(expiresUnix, 0)
	return data, nil
}

// InvalidateSessionCache removes a session from cache (used on logout/revoke).
func (s *TokenStorageService) InvalidateSessionCache(ctx context.Context, sessionID string) error {
	key := sessionCachePrefix + sessionID
	return s.redis.Del(ctx, key).Err()
}

// SessionCacheData holds the minimal session data needed for auth verification.
type SessionCacheData struct {
	UserID          string
	AccessTokenHash string
	Revoked         bool
	ExpiresAt       time.Time
}
