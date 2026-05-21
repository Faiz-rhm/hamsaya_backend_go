package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// failClosedKeyPrefixes lists rate-limit keys whose underlying endpoints are
// security-critical: when Redis errors out, refuse the request rather than
// silently letting unlimited traffic through. Endpoints that aren't on this
// list (e.g. ad-tracking, default browse limits) keep fail-open behaviour
// since blocking them on a Redis blip would degrade UX more than it'd help.
var failClosedKeyPrefixes = map[string]struct{}{
	"ratelimit:auth:":           {},
	"ratelimit:strict:":         {},
	"ratelimit:pwreset:":        {},
	"ratelimit:login:":          {},
	"ratelimit:login-email:":    {},
}

// shouldFailClosed reports whether a rate-limit error on this config should
// 503 the request instead of waving it through.
func shouldFailClosed(prefix string) bool {
	_, ok := failClosedKeyPrefixes[prefix]
	return ok
}

// RateLimitConfig defines rate limit configuration
type RateLimitConfig struct {
	MaxRequests int           // Maximum number of requests
	Window      time.Duration // Time window
	KeyPrefix   string        // Redis key prefix
}

// DefaultRateLimits defines default rate limits for different endpoint types.
//
// Caps were loosened across the board to reduce false-positive throttles
// from NAT'd users (mobile carrier IPs, office Wi-Fi, university networks
// where many devices share one IP) while keeping each limit well below
// what a botnet/script needs to be useful. Per-user limits leave headroom
// for power users (photo bursts in chat, prolific posters) without
// re-opening the abuse surface.
var DefaultRateLimits = map[string]RateLimitConfig{
	"default": {
		MaxRequests: 200,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:default:",
	},
	// auth: 10/min/IP — 5/min was tripping shared-IP users (NAT, carrier).
	// Still throttles credential-stuffing scripts well below useful speed.
	"auth": {
		MaxRequests: 10,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:auth:",
	},
	"strict": {
		MaxRequests: 5,
		Window:      5 * time.Minute,
		KeyPrefix:   "ratelimit:strict:",
	},
	"reports": {
		MaxRequests: 20,
		Window:      24 * time.Hour,
		KeyPrefix:   "ratelimit:reports:",
	},
	// password-reset: 5/10min/IP — covers fat-finger OTP entry on shared IPs.
	"password-reset": {
		MaxRequests: 5,
		Window:      10 * time.Minute,
		KeyPrefix:   "ratelimit:pwreset:",
	},
	// posts-create: 60/h/user — accommodates prolific posters (community
	// managers, business owners cross-posting events). Auth-plus-per-user
	// scope prevents bot floods even at this ceiling.
	"posts-create": {
		MaxRequests: 60,
		Window:      time.Hour,
		KeyPrefix:   "ratelimit:posts-create:",
	},
	// data-export: GDPR Article 20 dump is expensive (5k posts + 5k comments
	// + relationship lists). 2/24h/user — one retry slot for a broken zip
	// without re-opening abuse vector.
	"data-export": {
		MaxRequests: 2,
		Window:      24 * time.Hour,
		KeyPrefix:   "ratelimit:data-export:",
	},
	// ad-tracking: impression/click endpoints are public (no auth) so a
	// botnet could otherwise flood metric counters and inflate advertiser
	// charges. 120/min/IP covers an aggressive scroll-and-click user.
	"ad-tracking": {
		MaxRequests: 120,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:ad-tracking:",
	},
	// chat-send: 60/min/user — covers image-burst sends (a multi-photo
	// upload posts one message per image) and still blocks programmatic spam.
	"chat-send": {
		MaxRequests: 60,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:chat-send:",
	},
	// storage-stream: public proxy endpoint serving MinIO objects.
	// 300/min/IP comfortably covers a feed scroll with many images +
	// videos pulling Range chunks (one IP can request the same object
	// in 5+ HTTP requests for a 5MB video). Blocks bandwidth abuse.
	"storage-stream": {
		MaxRequests: 300,
		Window:      time.Minute,
		KeyPrefix:   "ratelimit:storage:",
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
			// Fail-closed for security-critical endpoints (login, password
			// reset, etc.) — otherwise an attacker could DoS Redis to disable
			// throttling and brute-force unimpeded. Non-critical endpoints
			// stay fail-open so a Redis blip doesn't make the app unusable.
			if shouldFailClosed(config.KeyPrefix) {
				utils.SendError(c, http.StatusServiceUnavailable,
					"Service temporarily unavailable. Please try again.", nil)
				c.Abort()
				return
			}
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

// LimitPostsCreate caps how many posts a single authenticated user can create
// per hour. Falls back to per-IP limiting for unauthenticated callers.
func (rl *RateLimiter) LimitPostsCreate() gin.HandlerFunc {
	config := DefaultRateLimits["posts-create"]
	return rl.LimitByUser(config)
}

// LimitDataExport gates GET /users/me/export at 1 request / 24h per user.
func (rl *RateLimiter) LimitDataExport() gin.HandlerFunc {
	config := DefaultRateLimits["data-export"]
	return rl.LimitByUser(config)
}

// LimitChatSend caps chat messages at 30/min/user. Spam guard — leaves
// plenty of headroom for normal conversation while blocking floods.
func (rl *RateLimiter) LimitChatSend() gin.HandlerFunc {
	config := DefaultRateLimits["chat-send"]
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
			if shouldFailClosed(config.KeyPrefix) {
				utils.SendError(c, http.StatusServiceUnavailable,
					"Service temporarily unavailable. Please try again.", nil)
				c.Abort()
				return
			}
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

// LimitLoginAttempts limits login attempts using TWO independent counters:
//
//  1. Per-IP, 3 attempts / 15 minutes — protects against single-host brute-
//     force.
//  2. Per-email (lowercased), 5 attempts / 15 minutes — defends against a
//     botnet rotating IPs to iterate one account's password. Combined with
//     the auth_service's account-lockout-after-5-fails, attackers can't
//     bypass either the IP throttle or the per-account counter.
//
// We peek at the request body to extract `email` without consuming it for the
// downstream handler.
func (rl *RateLimiter) LimitLoginAttempts() gin.HandlerFunc {
	ipCfg := RateLimitConfig{
		MaxRequests: 3,
		Window:      15 * time.Minute,
		KeyPrefix:   "ratelimit:login:",
	}
	emailCfg := RateLimitConfig{
		MaxRequests: 5,
		Window:      15 * time.Minute,
		KeyPrefix:   "ratelimit:login-email:",
	}
	return func(c *gin.Context) {
		// IP-based check first (uses request IP). Fail-closed: Redis errors
		// on login = 503 (don't let attacker DoS Redis to disable throttle).
		clientIP := c.ClientIP()
		ipKey := ipCfg.KeyPrefix + clientIP
		allowed, _, resetTime, err := rl.checkRateLimit(c.Request.Context(), ipKey, ipCfg)
		if err != nil {
			rl.logger.Error("Login IP rate-limit check failed", zap.Error(err))
			utils.SendError(c, http.StatusServiceUnavailable,
				"Service temporarily unavailable. Please try again.", nil)
			c.Abort()
			return
		}
		if !allowed {
			rl.logger.Warn("Login rate limit exceeded (IP)",
				zap.String("ip", clientIP),
			)
			c.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime).Seconds())))
			utils.SendError(c, http.StatusTooManyRequests, "Too many login attempts. Please try again later.", nil)
			c.Abort()
			return
		}

		// Email-based check. Peek body without consuming it.
		email := extractLoginEmail(c)
		if email != "" {
			emailKey := emailCfg.KeyPrefix + email
			allowed2, _, resetTime2, err2 := rl.checkRateLimit(c.Request.Context(), emailKey, emailCfg)
			if err2 != nil {
				rl.logger.Error("Login email rate-limit check failed", zap.Error(err2))
				utils.SendError(c, http.StatusServiceUnavailable,
					"Service temporarily unavailable. Please try again.", nil)
				c.Abort()
				return
			}
			if !allowed2 {
				rl.logger.Warn("Login rate limit exceeded (email)",
					zap.String("email", email),
				)
				c.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime2).Seconds())))
				utils.SendError(c, http.StatusTooManyRequests, "Too many login attempts. Please try again later.", nil)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// extractLoginEmail reads `email` from the JSON request body without
// consuming it for the downstream handler. Returns lowercased trimmed value
// or "" on any error / missing field.
func extractLoginEmail(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	const maxBody = 64 * 1024 // 64KB cap — login bodies are tiny.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBody))
	if err != nil {
		return ""
	}
	// Restore for downstream handler.
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payload.Email))
}

// LimitPasswordReset limits password-reset OTP verification attempts per IP.
// Limit: 3 attempts per 10 minutes — tight enough to prevent brute-forcing 6-digit codes.
func (rl *RateLimiter) LimitPasswordReset() gin.HandlerFunc {
	return rl.LimitByType("password-reset")
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
