package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// DailyLimitRepository persists per-post-type daily creation caps.
// Counters live in Redis (services.DailyLimitService); this repo only
// reads/writes the *limit*, not usage.
type DailyLimitRepository interface {
	List(ctx context.Context) ([]*models.DailyPostLimit, error)
	Get(ctx context.Context, postType string) (*models.DailyPostLimit, error)
	Update(ctx context.Context, postType string, req *models.UpdateDailyPostLimitRequest, updatedBy string) (*models.DailyPostLimit, error)
}

type dailyLimitRepository struct {
	db *database.DB
}

func NewDailyLimitRepository(db *database.DB) DailyLimitRepository {
	return &dailyLimitRepository{db: db}
}

func (r *dailyLimitRepository) List(ctx context.Context) ([]*models.DailyPostLimit, error) {
	const query = `
		SELECT post_type, user_limit, business_multiplier, unlimited, description, updated_at, updated_by
		FROM daily_post_limits
		ORDER BY post_type ASC
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list daily limits: %w", err)
	}
	defer rows.Close()

	var limits []*models.DailyPostLimit
	for rows.Next() {
		l := &models.DailyPostLimit{}
		if err := rows.Scan(&l.PostType, &l.UserLimit, &l.BusinessMultiplier,
			&l.Unlimited, &l.Description, &l.UpdatedAt, &l.UpdatedBy); err != nil {
			return nil, fmt.Errorf("scan daily limit: %w", err)
		}
		limits = append(limits, l)
	}
	return limits, rows.Err()
}

func (r *dailyLimitRepository) Get(ctx context.Context, postType string) (*models.DailyPostLimit, error) {
	const query = `
		SELECT post_type, user_limit, business_multiplier, unlimited, description, updated_at, updated_by
		FROM daily_post_limits
		WHERE post_type = $1
	`
	l := &models.DailyPostLimit{}
	err := r.db.Pool.QueryRow(ctx, query, postType).Scan(
		&l.PostType, &l.UserLimit, &l.BusinessMultiplier,
		&l.Unlimited, &l.Description, &l.UpdatedAt, &l.UpdatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get daily limit: %w", err)
	}
	return l, nil
}

func (r *dailyLimitRepository) Update(
	ctx context.Context,
	postType string,
	req *models.UpdateDailyPostLimitRequest,
	updatedBy string,
) (*models.DailyPostLimit, error) {
	// Build dynamic SET clause so callers can update only the fields they care about.
	const query = `
		UPDATE daily_post_limits
		SET user_limit          = COALESCE($2, user_limit),
		    business_multiplier = COALESCE($3, business_multiplier),
		    unlimited           = COALESCE($4, unlimited),
		    description         = COALESCE($5, description),
		    updated_at          = NOW(),
		    updated_by          = $6
		WHERE post_type = $1
		RETURNING post_type, user_limit, business_multiplier, unlimited, description, updated_at, updated_by
	`
	l := &models.DailyPostLimit{}
	err := r.db.Pool.QueryRow(ctx, query, postType,
		req.UserLimit, req.BusinessMultiplier, req.Unlimited, req.Description, updatedBy,
	).Scan(&l.PostType, &l.UserLimit, &l.BusinessMultiplier,
		&l.Unlimited, &l.Description, &l.UpdatedAt, &l.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update daily limit: %w", err)
	}
	return l, nil
}

// secondsUntilUTCMidnight returns the TTL to use for the daily counter so
// it expires exactly at the next 00:00 UTC rollover. Unused here but
// shared with the service via a single helper.
func secondsUntilUTCMidnight(now time.Time) time.Duration {
	tomorrow := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day()+1, 0, 0, 0, 0, time.UTC)
	return tomorrow.Sub(now.UTC())
}

// SecondsUntilUTCMidnight is the public wrapper used by the service.
func SecondsUntilUTCMidnight(now time.Time) time.Duration {
	return secondsUntilUTCMidnight(now)
}
