package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimitConfig defines rate limit configuration
type RateLimitConfig struct {
	MaxRequests int           // Maximum number of requests
	Window      time.Duration // Time window
	KeyPrefix   string        // Redis key prefix
}

// DefaultRateLimits defines default rate limits for different endpoint types
var DefaultRateLimits = map[string]RateLimitConfig{
	"default": {
		MaxRequests: 100,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:default:",
	},
	"auth": {
		MaxRequests: 5,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:auth:",
	},
	"strict": {
		MaxRequests: 3,
		Window:      5 * time.Minute,
		KeyPrefix:   "ratelimit:strict:",
	},
	"reports": {
		MaxRequests: 10,
		Window:      24 * time.Hour,
		KeyPrefix:   "ratelimit:reports:",
	},
}

// RateLimiter handles rate limiting using Redis
type RateLimiter struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		logger: logger,
	}
}

// Limit creates a rate limiting middleware with the specified config
func (rl *RateLimiter) Limit(config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientIP := c.ClientIP()
		key := config.KeyPrefix + clientIP

		// Check rate limit
		allowed, remaining, resetTime, err := rl.checkRateLimit(c.Request.Context(), key, config)
		if err != nil {
			rl.logger.Error("Rate limit check failed",
				zap.String("key", key),
				zap.Error(err),
			)
			// On error, allow the request but log it
			c.Next()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", config.MaxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))

		if !allowed {
			rl.logger.Warn("Rate limit exceeded",
				zap.String("ip", clientIP),
				zap.String("path", c.Request.URL.Path),
				zap.Int("max_requests", config.MaxRequests),
			)

			c.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime).Seconds())))
			utils.SendError(c, http.StatusTooManyRequests,
				"Rate limit exceeded. Please try again later.",
				nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// LimitByType creates a rate limiting middleware by type name
func (rl *RateLimiter) LimitByType(limitType string) gin.HandlerFunc {
	config, exists := DefaultRateLimits[limitType]
	if !exists {
		config = DefaultRateLimits["default"]
	}
	return rl.Limit(config)
}

// LimitAuth is a convenience method for auth endpoints
func (rl *RateLimiter) LimitAuth() gin.HandlerFunc {
	return rl.LimitByType("auth")
}

// LimitStrict is a convenience method for sensitive endpoints
func (rl *RateLimiter) LimitStrict() gin.HandlerFunc {
	return rl.LimitByType("strict")
}

// LimitReports is a convenience method for report endpoints
// Limits users to 10 reports per 24 hours to prevent spam
func (rl *RateLimiter) LimitReports() gin.HandlerFunc {
	config := DefaultRateLimits["reports"]
	return rl.LimitByUser(config)
}

// checkRateLimit checks if a request is within rate limits using sliding window
func (rl *RateLimiter) checkRateLimit(
	ctx context.Context,
	key string,
	config RateLimitConfig,
) (allowed bool, remaining int, resetTime time.Time, err error) {
	now := time.Now()
	windowStart := now.Add(-config.Window)

	// Use Redis sorted set for sliding window
	// Score is timestamp, member is unique request ID

	pipe := rl.redis.Pipeline()

	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count current requests in window
	countCmd := pipe.ZCard(ctx, key)

	// Add current request
	requestID := fmt.Sprintf("%d", now.UnixNano())
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: requestID,
	})

	// Set expiration on the key
	pipe.Expire(ctx, key, config.Window+time.Minute)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, 0, now.Add(config.Window), err
	}

	// Get count
	count := countCmd.Val()

	// Check if within limit
	allowed = count < int64(config.MaxRequests)
	remaining = config.MaxRequests - int(count) - 1
	if remaining < 0 {
		remaining = 0
	}

	// Calculate reset time (end of current window)
	resetTime = now.Add(config.Window)

	return allowed, remaining, resetTime, nil
}

// LimitByUser creates a rate limiter based on user ID instead of IP
func (rl *RateLimiter) LimitByUser(config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			// If no user ID, fall back to IP-based limiting
			rl.Limit(config)(c)
			return
		}

		key := config.KeyPrefix + "user:" + userID.(string)

		// Check rate limit
		allowed, remaining, resetTime, err := rl.checkRateLimit(c.Request.Context(), key, config)
		if err != nil {
			rl.logger.Error("Rate limit check failed",
				zap.String("key", key),
				zap.Error(err),
			)
			c.Next()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", config.MaxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))

		if !allowed {
			rl.logger.Warn("Rate limit exceeded",
				zap.String("user_id", userID.(string)),
				zap.String("path", c.Request.URL.Path),
			)

			c.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime).Seconds())))
			utils.SendError(c, http.StatusTooManyRequests,
				"Rate limit exceeded. Please try again later.",
				nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// LimitLoginAttempts specifically limits login attempts per IP
func (rl *RateLimiter) LimitLoginAttempts() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use IP-based rate limiting for login attempts
		// This prevents brute force attacks on the login endpoint
		// Limit: 5 attempts per 15 minutes per IP

		config := RateLimitConfig{
			MaxRequests: 5,
			Window:      15 * time.Minute,
			KeyPrefix:   "ratelimit:login:",
		}

		rl.Limit(config)(c)
	}
}

// ClearRateLimit clears rate limit for a specific key (useful for testing or admin actions)
func (rl *RateLimiter) ClearRateLimit(ctx context.Context, keyPrefix, identifier string) error {
	key := keyPrefix + identifier
	return rl.redis.Del(ctx, key).Err()
}

// GetRateLimitInfo returns current rate limit info for a key
func (rl *RateLimiter) GetRateLimitInfo(
	ctx context.Context,
	key string,
	config RateLimitConfig,
) (requests int64, resetTime time.Time, err error) {
	now := time.Now()
	windowStart := now.Add(-config.Window)

	// Remove old entries
	err = rl.redis.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano())).Err()
	if err != nil {
		return 0, time.Time{}, err
	}

	// Count current requests
	requests, err = rl.redis.ZCard(ctx, key).Result()
	if err != nil {
		return 0, time.Time{}, err
	}

	resetTime = now.Add(config.Window)
	return requests, resetTime, nil
}
