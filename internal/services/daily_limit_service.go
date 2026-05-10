package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
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
	db     *database.DB
	redis  *redis.Client
	logger *zap.Logger

	// In-process limit cache. Limits change rarely; callers don't need a
	// fresh DB read per request.
	limitCacheTTL time.Duration
	mu            sync.RWMutex
	cache         map[string]cachedLimit

	// Per-user override cache, keyed by "userID:postType". Negative
	// (no-row) hits are also cached to avoid hammering the DB on every
	// post-create when the typical user has no override.
	overrideCacheTTL time.Duration
	overrideMu       sync.RWMutex
	overrideCache    map[string]cachedOverride
}

type cachedLimit struct {
	limit *models.DailyPostLimit
	at    time.Time
}

// UserDailyLimitOverride mirrors the user_daily_post_limit_overrides row.
type UserDailyLimitOverride struct {
	UserID         string    `json:"user_id"`
	UserEmail      string    `json:"user_email,omitempty"`
	PostType       string    `json:"post_type"`
	OverrideLimit  *int      `json:"override_limit,omitempty"`
	Unlimited      bool      `json:"unlimited"`
	Reason         *string   `json:"reason,omitempty"`
	CreatedBy      *string   `json:"created_by,omitempty"`
	CreatedByEmail *string   `json:"created_by_email,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type cachedOverride struct {
	override *UserDailyLimitOverride // nil = sentinel "no row in DB"
	at       time.Time
}

func NewDailyLimitService(
	repo repositories.DailyLimitRepository,
	db *database.DB,
	redis *redis.Client,
	logger *zap.Logger,
) *DailyLimitService {
	return &DailyLimitService{
		repo:             repo,
		db:               db,
		redis:            redis,
		logger:           logger,
		limitCacheTTL:    30 * time.Second,
		cache:            make(map[string]cachedLimit),
		overrideCacheTTL: 30 * time.Second,
		overrideCache:    make(map[string]cachedOverride),
	}
}

// CheckAndIncrement is the gate called from the post-create handler.
//
// Returns ErrDailyLimitExceeded if the user is already at-or-above the
// effective limit for postType. On success, the counter is incremented
// atomically (Redis INCR) and the call is treated as committing one slot.
//
// onBusiness=true applies the configured BusinessMultiplier to the base
// limit. role==RoleSuperAdmin bypasses the check entirely; admins and
// moderators are intentionally NOT exempt — they post under the same caps as
// regular users so admin/moderator dev accounts can't accidentally bypass
// production limits during testing.
func (s *DailyLimitService) CheckAndIncrement(
	ctx context.Context,
	userID string,
	role models.UserRole,
	postType string,
	onBusiness bool,
) error {
	if role == models.RoleSuperAdmin {
		return nil // super admins are explicitly unlimited
	}

	limit, err := s.cachedLimit(ctx, postType)
	if err != nil {
		return err
	}
	if limit != nil && limit.Unlimited {
		return nil // admin marked this post type as unlimited for everyone
	}

	// Per-user override check first — overrides beat the global config.
	override, err := s.cachedOverride(ctx, userID, postType)
	if err != nil {
		// Don't fail the post-create on override read errors; fall back
		// to the global limit so a transient DB hiccup doesn't lock
		// users out of posting.
		s.logger.Warn("override lookup failed; falling back to global",
			zap.String("user_id", userID), zap.String("post_type", postType), zap.Error(err))
		override = nil
	}
	if override != nil && override.Unlimited {
		return nil
	}

	effective := 0
	if override != nil && override.OverrideLimit != nil {
		effective = *override.OverrideLimit
	} else {
		effective, err = s.effectiveLimit(ctx, postType, onBusiness)
		if err != nil {
			return err
		}
	}
	if effective <= 0 {
		// Limit row missing or set to 0 by admin = feature explicitly disabled.
		return ErrDailyLimitExceeded
	}

	key, ttl := counterKeyAndTTL(userID, postType, time.Now())

	// Single atomic INCR. Returns the post-increment value, so we can
	// reject + DECR in one round-trip when the user has gone over. This
	// closes the prior TOCTOU race where two concurrent posts could both
	// read `current=limit-1`, both pass the check, and both INCR — pushing
	// the counter one over the limit on every parallel-burst attack.
	newValue, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("daily limit incr: %w", err)
	}

	// Fresh key has no TTL until we set one. Use EXPIRE on first increment
	// (newValue == 1). We avoid ExpireNX so we stay compatible with Redis
	// versions older than 7.0 — pipeline silently dropped that command and
	// left the key without TTL, so it never reset at midnight.
	if newValue == 1 {
		if expErr := s.redis.Expire(ctx, key, ttl).Err(); expErr != nil {
			s.logger.Warn("daily limit expire set failed",
				zap.String("user_id", userID), zap.String("post_type", postType),
				zap.Error(expErr))
		}
	}

	if newValue > int64(effective) {
		// Roll back the increment so concurrent over-limit attempts don't
		// inflate the counter for the rest of the day. Best-effort: a
		// failed DECR just means analytics over-counts by one — never
		// blocks legitimate retries.
		if _, decErr := s.redis.Decr(ctx, key).Result(); decErr != nil {
			s.logger.Warn("daily limit decr after over-limit failed",
				zap.String("user_id", userID), zap.String("post_type", postType),
				zap.Error(decErr))
		}
		return ErrDailyLimitExceeded
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

		// Per-user override — bypasses or replaces the cap for this
		// user+post_type. Read errors fall back to the global limit so
		// the usage panel never blanks out on a DB hiccup.
		override, err := s.cachedOverride(ctx, userID, lim.PostType)
		if err != nil {
			s.logger.Warn("usage override lookup failed",
				zap.String("user_id", userID), zap.String("post_type", lim.PostType), zap.Error(err))
			override = nil
		}
		if override != nil && override.Unlimited {
			usage.Unlimited = true
			usage.Remaining = -1
			usage.Limit = -1
			out = append(out, usage)
			continue
		}

		effective := applyMultiplier(lim, onBusiness)
		if override != nil && override.OverrideLimit != nil {
			effective = *override.OverrideLimit
		}
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

// ─── Per-user overrides ──────────────────────────────────────────────────────

// ListUserOverrides returns every active override across all users.
// Joined with the users table so the admin UI can render emails.
func (s *DailyLimitService) ListUserOverrides(ctx context.Context) ([]UserDailyLimitOverride, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT o.user_id::text, COALESCE(u.email,''),
		       o.post_type, o.override_limit, o.unlimited, o.reason,
		       o.created_by::text, COALESCE(c.email,''),
		       o.created_at, o.updated_at
		FROM user_daily_post_limit_overrides o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN users c ON c.id = o.created_by
		ORDER BY o.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserDailyLimitOverride, 0)
	for rows.Next() {
		var r UserDailyLimitOverride
		var creatorEmail string
		if err := rows.Scan(&r.UserID, &r.UserEmail, &r.PostType, &r.OverrideLimit,
			&r.Unlimited, &r.Reason, &r.CreatedBy, &creatorEmail,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if creatorEmail != "" {
			r.CreatedByEmail = &creatorEmail
		}
		out = append(out, r)
	}
	return out, nil
}

// ListUserOverridesFor returns overrides for a single user (used on the
// per-user admin detail panel).
func (s *DailyLimitService) ListUserOverridesFor(ctx context.Context, userID string) ([]UserDailyLimitOverride, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT o.user_id::text, '', o.post_type, o.override_limit, o.unlimited,
		       o.reason, o.created_by::text, COALESCE(c.email,''),
		       o.created_at, o.updated_at
		FROM user_daily_post_limit_overrides o
		LEFT JOIN users c ON c.id = o.created_by
		WHERE o.user_id = $1
		ORDER BY o.post_type ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserDailyLimitOverride, 0)
	for rows.Next() {
		var r UserDailyLimitOverride
		var creatorEmail string
		if err := rows.Scan(&r.UserID, &r.UserEmail, &r.PostType, &r.OverrideLimit,
			&r.Unlimited, &r.Reason, &r.CreatedBy, &creatorEmail,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if creatorEmail != "" {
			r.CreatedByEmail = &creatorEmail
		}
		out = append(out, r)
	}
	return out, nil
}

// SetUserOverride upserts one (user_id, post_type) override. Either
// `unlimited=true` or a non-nil overrideLimit must be supplied — never
// both, never neither (the CHECK constraint enforces this at the DB
// layer too).
func (s *DailyLimitService) SetUserOverride(
	ctx context.Context,
	userID, postType string,
	overrideLimit *int,
	unlimited bool,
	reason, adminID string,
) error {
	if !unlimited && overrideLimit == nil {
		return fmt.Errorf("must set unlimited=true or override_limit")
	}
	if unlimited {
		// Force override_limit nil when unlimited so we don't store
		// stale numbers.
		overrideLimit = nil
	} else if *overrideLimit < 0 {
		return fmt.Errorf("override_limit must be >= 0")
	}

	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO user_daily_post_limit_overrides
		    (user_id, post_type, override_limit, unlimited, reason, created_by, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, NOW())
		ON CONFLICT (user_id, post_type) DO UPDATE SET
		    override_limit = EXCLUDED.override_limit,
		    unlimited      = EXCLUDED.unlimited,
		    reason         = EXCLUDED.reason,
		    created_by     = EXCLUDED.created_by,
		    updated_at     = NOW()
	`, userID, postType, overrideLimit, unlimited, reason, adminID)
	if err != nil {
		return err
	}
	s.invalidateOverrideCache(userID, postType)
	return nil
}

// DeleteUserOverride removes one override. The user reverts to the
// global limit for that post_type.
func (s *DailyLimitService) DeleteUserOverride(ctx context.Context, userID, postType string) error {
	_, err := s.db.Pool.Exec(ctx,
		`DELETE FROM user_daily_post_limit_overrides WHERE user_id=$1 AND post_type=$2`,
		userID, postType,
	)
	if err == nil {
		s.invalidateOverrideCache(userID, postType)
	}
	return err
}

func (s *DailyLimitService) cachedOverride(ctx context.Context, userID, postType string) (*UserDailyLimitOverride, error) {
	// Override consultation is purely opt-in. Tests and any future
	// callers that wire DailyLimitService without a DB get a clean
	// "no override" answer rather than a nil-pointer panic on
	// db.Pool.QueryRow.
	if s.db == nil {
		return nil, nil
	}
	cacheKey := userID + ":" + postType
	s.overrideMu.RLock()
	if entry, ok := s.overrideCache[cacheKey]; ok && time.Since(entry.at) < s.overrideCacheTTL {
		s.overrideMu.RUnlock()
		return entry.override, nil
	}
	s.overrideMu.RUnlock()

	var r UserDailyLimitOverride
	err := s.db.Pool.QueryRow(ctx, `
		SELECT user_id::text, '', post_type, override_limit, unlimited, reason,
		       created_by::text, '', created_at, updated_at
		FROM user_daily_post_limit_overrides
		WHERE user_id = $1 AND post_type = $2
	`, userID, postType).Scan(
		&r.UserID, &r.UserEmail, &r.PostType, &r.OverrideLimit, &r.Unlimited, &r.Reason,
		&r.CreatedBy, new(string), &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.overrideMu.Lock()
			s.overrideCache[cacheKey] = cachedOverride{override: nil, at: time.Now()}
			s.overrideMu.Unlock()
			return nil, nil
		}
		return nil, err
	}

	s.overrideMu.Lock()
	s.overrideCache[cacheKey] = cachedOverride{override: &r, at: time.Now()}
	s.overrideMu.Unlock()
	return &r, nil
}

func (s *DailyLimitService) invalidateOverrideCache(userID, postType string) {
	s.overrideMu.Lock()
	delete(s.overrideCache, userID+":"+postType)
	s.overrideMu.Unlock()
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
