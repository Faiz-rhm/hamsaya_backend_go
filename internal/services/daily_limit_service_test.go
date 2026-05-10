package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
)

// stubLimitRepo lets tests inject custom limits without a real DB.
type stubLimitRepo struct {
	limits map[string]*models.DailyPostLimit
}

func (s *stubLimitRepo) List(_ context.Context) ([]*models.DailyPostLimit, error) {
	out := make([]*models.DailyPostLimit, 0, len(s.limits))
	for _, l := range s.limits {
		out = append(out, l)
	}
	return out, nil
}
func (s *stubLimitRepo) Get(_ context.Context, t string) (*models.DailyPostLimit, error) {
	return s.limits[t], nil
}
func (s *stubLimitRepo) Update(_ context.Context, t string, _ *models.UpdateDailyPostLimitRequest, _ string) (*models.DailyPostLimit, error) {
	return s.limits[t], nil
}

func newDailyLimitTest(t *testing.T, limits map[string]*models.DailyPostLimit) (*DailyLimitService, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := &stubLimitRepo{limits: limits}
	svc := NewDailyLimitService(repo, nil, rdb, zap.NewNop())
	return svc, func() { _ = rdb.Close() }
}

func TestDailyLimit_AllowsUpToCap(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED": {PostType: "FEED", UserLimit: 3, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))
	}

	err := svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false)
	require.True(t, errors.Is(err, ErrDailyLimitExceeded), "expected ErrDailyLimitExceeded, got %v", err)
}

func TestDailyLimit_SuperAdminBypass(t *testing.T) {
	// Only super_admin bypasses caps. Admin and moderator are intentionally
	// subject to the same daily limit as regular users so dev accounts with
	// admin role can't accidentally bypass production caps during testing.
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED": {PostType: "FEED", UserLimit: 1, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	ctx := context.Background()
	for i := 0; i < 50; i++ {
		require.NoError(t, svc.CheckAndIncrement(ctx, "su-1", models.RoleSuperAdmin, "FEED", false))
	}

	// Plain admin must NOT bypass.
	require.NoError(t, svc.CheckAndIncrement(ctx, "admin-1", models.RoleAdmin, "FEED", false))
	err := svc.CheckAndIncrement(ctx, "admin-1", models.RoleAdmin, "FEED", false)
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
}

func TestDailyLimit_BusinessMultiplier(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"SELL": {PostType: "SELL", UserLimit: 2, BusinessMultiplier: 3.0}, // business cap = 6
	})
	defer cleanup()

	ctx := context.Background()
	for i := 0; i < 6; i++ {
		require.NoError(t, svc.CheckAndIncrement(ctx, "biz-1", models.RoleUser, "SELL", true))
	}

	err := svc.CheckAndIncrement(ctx, "biz-1", models.RoleUser, "SELL", true)
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
}

func TestDailyLimit_PerUserIsolation(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED": {PostType: "FEED", UserLimit: 1, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))
	// User 2 must still have a slot — counters are per-user.
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-2", models.RoleUser, "FEED", false))
}

func TestDailyLimit_PerTypeIsolation(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED":  {PostType: "FEED", UserLimit: 1, BusinessMultiplier: 2.0},
		"EVENT": {PostType: "EVENT", UserLimit: 1, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))
	// EVENT slot still available even though FEED was used.
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "EVENT", false))
}

func TestDailyLimit_ZeroLimitMeansBlocked(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"PULL": {PostType: "PULL", UserLimit: 0, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	err := svc.CheckAndIncrement(context.Background(), "u-1", models.RoleUser, "PULL", false)
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
}

func TestDailyLimit_GetUsageReportsRemaining(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED": {PostType: "FEED", UserLimit: 5, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))

	usage, err := svc.GetUsage(ctx, "u-1", models.RoleUser, false)
	require.NoError(t, err)
	require.Len(t, usage, 1)
	require.Equal(t, 2, usage[0].Used)
	require.Equal(t, 5, usage[0].Limit)
	require.Equal(t, 3, usage[0].Remaining)
	require.False(t, usage[0].Unlimited)
	// Resets in <= 24h
	require.True(t, usage[0].ResetsAt.After(time.Now()))
	require.True(t, usage[0].ResetsAt.Before(time.Now().Add(25*time.Hour)))
}

func TestDailyLimit_GetUsageReportsAdminUnlimited(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED": {PostType: "FEED", UserLimit: 5, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	usage, err := svc.GetUsage(context.Background(), "admin-1", models.RoleAdmin, false)
	require.NoError(t, err)
	require.Len(t, usage, 1)
	require.True(t, usage[0].Unlimited)
	require.Equal(t, -1, usage[0].Remaining)
	require.Equal(t, -1, usage[0].Limit)
}

func TestDailyLimit_RefundDecrementsCounter(t *testing.T) {
	svc, cleanup := newDailyLimitTest(t, map[string]*models.DailyPostLimit{
		"FEED": {PostType: "FEED", UserLimit: 2, BusinessMultiplier: 2.0},
	})
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))

	// At cap.
	require.True(t, errors.Is(
		svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false),
		ErrDailyLimitExceeded,
	))

	// Refund one — slot should re-open.
	svc.Refund(ctx, "u-1", "FEED")
	require.NoError(t, svc.CheckAndIncrement(ctx, "u-1", models.RoleUser, "FEED", false))
}

// SetUserOverride validates inputs BEFORE touching the DB. We pass a
// nil-DB service so any path that reaches db.Pool.Exec would nil-deref;
// successful early returns confirm the validation guard fires up front.
func TestDailyLimit_SetUserOverride_ValidationFailsFast(t *testing.T) {
	svc := &DailyLimitService{db: nil}

	err := svc.SetUserOverride(context.Background(), "u-1", "FEED", nil, false, "", "admin-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must set unlimited=true or override_limit")

	negative := -3
	err = svc.SetUserOverride(context.Background(), "u-1", "FEED", &negative, false, "", "admin-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "override_limit must be >= 0")
}
