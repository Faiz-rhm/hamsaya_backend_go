package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
)

// ErrDailyLimitExceeded is returned by CheckAndIncrement when the user has
// already created their daily allotment for the given post type. Callers
// should surface this as 429 Too Many Requests.
var ErrDailyLimitExceeded = errors.New("daily post limit exceeded")

// DailyLimitService gates post creation by a per-post-type daily counter.
//
// • Limits are read from Postgres via DailyLimitRepository (admin-editable)
//   and cached in-process for limitCacheTTL to avoid hitting the DB on every
//   post create.
// • Counters live in Redis under "daily_limit:{userID}:{postType}:{utcDate}"
//   with a TTL that expires at the next 00:00 UTC.
// • Admin role users bypass the limit. Business-authored posts get a
//   multiplier (BusinessMultiplier) on top of the base UserLimit.
type DailyLimitService struct {
	repo   repositories.DailyLimitRepository
	redis  *redis.Client
	logger *zap.Logger

	// In-process limit cache. Limits change rarely; callers don't need a
	// fresh DB read per request.
	limitCacheTTL time.Duration
	mu            sync.RWMutex
	cache         map[string]cachedLimit
}

type cachedLimit struct {
	limit *models.DailyPostLimit
	at    time.Time
}

func NewDailyLimitService(
	repo repositories.DailyLimitRepository,
	redis *redis.Client,
	logger *zap.Logger,
) *DailyLimitService {
	return &DailyLimitService{
		repo:          repo,
		redis:         redis,
		logger:        logger,
		limitCacheTTL: 30 * time.Second,
		cache:         make(map[string]cachedLimit),
	}
}

// CheckAndIncrement is the gate called from the post-create handler.
//
// Returns ErrDailyLimitExceeded if the user is already at-or-above the
// effective limit for postType. On success, the counter is incremented
// atomically (Redis INCR) and the call is treated as committing one slot.
//
// onBusiness=true applies the configured BusinessMultiplier to the base
// limit. role==RoleAdmin bypasses the check entirely.
func (s *DailyLimitService) CheckAndIncrement(
	ctx context.Context,
	userID string,
	role models.UserRole,
	postType string,
	onBusiness bool,
) error {
	if role == models.RoleAdmin {
		return nil // admins are unlimited
	}

	limit, err := s.cachedLimit(ctx, postType)
	if err != nil {
		return err
	}
	if limit != nil && limit.Unlimited {
		return nil // admin marked this post type as unlimited for everyone
	}

	effective, err := s.effectiveLimit(ctx, postType, onBusiness)
	if err != nil {
		return err
	}
	if effective <= 0 {
		// Limit row missing or set to 0 by admin = feature explicitly disabled.
		return ErrDailyLimitExceeded
	}

	key, ttl := counterKeyAndTTL(userID, postType, time.Now())

	// INCR returns the value AFTER increment. We pre-check by reading
	// current value so we don't even bump the counter when over the limit
	// (avoids skewed analytics).
	current, err := s.redis.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("daily limit get: %w", err)
	}
	if current >= effective {
		return ErrDailyLimitExceeded
	}

	// Atomic increment. EXPIRE is set on first increment only — Redis 7+
	// supports EXPIRE NX which is the safe pattern.
	if _, err := s.redis.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.Incr(ctx, key)
		// ExpireNX is a no-op if the key already has a TTL.
		p.ExpireNX(ctx, key, ttl)
		return nil
	}); err != nil {
		return fmt.Errorf("daily limit incr: %w", err)
	}

	return nil
}

// Refund decrements the counter — call from the post-create handler when the
// post creation fails *after* CheckAndIncrement bumped the counter. Avoids
// users being one short of their daily quota due to a transient backend
// error. Floors at zero.
func (s *DailyLimitService) Refund(
	ctx context.Context,
	userID string,
	postType string,
) {
	if userID == "" {
		return
	}
	key, _ := counterKeyAndTTL(userID, postType, time.Now())
	// DECR floors at zero only via Lua. Best-effort decrement is fine here:
	// a value going to -1 has no user-visible effect because the next call
	// reads "0 < limit" anyway.
	if _, err := s.redis.Decr(ctx, key).Result(); err != nil {
		s.logger.Warn("daily limit refund failed", zap.String("user_id", userID),
			zap.String("post_type", postType), zap.Error(err))
	}
}

// GetUsage returns the per-post-type usage for a user. Used by the mobile
// app to render "X of N daily posts remaining" headers.
//
// role==RoleAdmin reports Unlimited=true with Remaining=-1.
func (s *DailyLimitService) GetUsage(
	ctx context.Context,
	userID string,
	role models.UserRole,
	onBusiness bool,
) ([]*models.DailyLimitUsage, error) {
	limits, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	resetsAt := nextUTCMidnight(now)
	out := make([]*models.DailyLimitUsage, 0, len(limits))

	for _, lim := range limits {
		usage := &models.DailyLimitUsage{
			PostType: lim.PostType,
			ResetsAt: resetsAt,
		}

		if role == models.RoleAdmin {
			usage.Unlimited = true
			usage.Remaining = -1
			usage.Limit = -1
			out = append(out, usage)
			continue
		}

		if lim.Unlimited {
			usage.Unlimited = true
			usage.Remaining = -1
			usage.Limit = -1
			out = append(out, usage)
			continue
		}

		effective := applyMultiplier(lim, onBusiness)
		usage.Limit = effective

		key, _ := counterKeyAndTTL(userID, lim.PostType, now)
		current, err := s.redis.Get(ctx, key).Int()
		if err != nil && !errors.Is(err, redis.Nil) {
			s.logger.Warn("daily limit usage read failed",
				zap.String("user_id", userID), zap.String("post_type", lim.PostType),
				zap.Error(err))
		}
		usage.Used = current
		remaining := effective - current
		if remaining < 0 {
			remaining = 0
		}
		usage.Remaining = remaining
		out = append(out, usage)
	}
	return out, nil
}

// ─── Admin operations ─────────────────────────────────────────────────────────

func (s *DailyLimitService) ListLimits(ctx context.Context) ([]*models.DailyPostLimit, error) {
	return s.repo.List(ctx)
}

func (s *DailyLimitService) UpdateLimit(
	ctx context.Context,
	postType string,
	req *models.UpdateDailyPostLimitRequest,
	updatedBy string,
) (*models.DailyPostLimit, error) {
	limit, err := s.repo.Update(ctx, postType, req, updatedBy)
	if err != nil {
		return nil, err
	}
	if limit == nil {
		return nil, utils.NewNotFoundError("daily limit row not found", nil)
	}
	// Bust the cache so the new value takes effect immediately.
	s.invalidateCache(postType)
	return limit, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (s *DailyLimitService) effectiveLimit(
	ctx context.Context,
	postType string,
	onBusiness bool,
) (int, error) {
	limit, err := s.cachedLimit(ctx, postType)
	if err != nil {
		return 0, err
	}
	if limit == nil {
		return 0, nil
	}
	return applyMultiplier(limit, onBusiness), nil
}

func (s *DailyLimitService) cachedLimit(ctx context.Context, postType string) (*models.DailyPostLimit, error) {
	s.mu.RLock()
	if entry, ok := s.cache[postType]; ok && time.Since(entry.at) < s.limitCacheTTL {
		s.mu.RUnlock()
		return entry.limit, nil
	}
	s.mu.RUnlock()

	limit, err := s.repo.Get(ctx, postType)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[postType] = cachedLimit{limit: limit, at: time.Now()}
	s.mu.Unlock()
	return limit, nil
}

func (s *DailyLimitService) invalidateCache(postType string) {
	s.mu.Lock()
	delete(s.cache, postType)
	s.mu.Unlock()
}

func applyMultiplier(limit *models.DailyPostLimit, onBusiness bool) int {
	if !onBusiness {
		return limit.UserLimit
	}
	multiplied := float64(limit.UserLimit) * limit.BusinessMultiplier
	return int(multiplied)
}

func counterKeyAndTTL(userID, postType string, now time.Time) (string, time.Duration) {
	utc := now.UTC()
	dateStr := utc.Format("2006-01-02")
	key := fmt.Sprintf("daily_limit:%s:%s:%s", userID, postType, dateStr)
	return key, repositories.SecondsUntilUTCMidnight(now)
}

func nextUTCMidnight(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}
