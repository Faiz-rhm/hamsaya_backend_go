package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// FeatureFlag is the persisted shape of a platform-wide boolean toggle.
type FeatureFlag struct {
	Key         string    `json:"key"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	UpdatedBy   *string   `json:"updated_by,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FeatureFlagRepository abstracts feature_flags table access. Implemented
// against pgx; backed by an in-process cache in the service layer.
type FeatureFlagRepository interface {
	List(ctx context.Context) ([]FeatureFlag, error)
	Get(ctx context.Context, key string) (*FeatureFlag, error)
	Set(ctx context.Context, key string, enabled bool, updatedBy string) error
}

type featureFlagRepository struct {
	db *database.DB
}

// NewFeatureFlagRepository constructs a repo bound to the shared DB pool.
func NewFeatureFlagRepository(db *database.DB) FeatureFlagRepository {
	return &featureFlagRepository{db: db}
}

func (r *featureFlagRepository) List(ctx context.Context) ([]FeatureFlag, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT key, enabled, COALESCE(description, ''), updated_by::text, updated_at
		FROM feature_flags
		ORDER BY key
	`)
	if err != nil {
		return nil, fmt.Errorf("feature_flags list: %w", err)
	}
	defer rows.Close()

	var out []FeatureFlag
	for rows.Next() {
		var f FeatureFlag
		var updatedBy *string
		if err := rows.Scan(&f.Key, &f.Enabled, &f.Description, &updatedBy, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("feature_flags scan: %w", err)
		}
		f.UpdatedBy = updatedBy
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *featureFlagRepository) Get(ctx context.Context, key string) (*FeatureFlag, error) {
	var f FeatureFlag
	var updatedBy *string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT key, enabled, COALESCE(description, ''), updated_by::text, updated_at
		FROM feature_flags
		WHERE key = $1
	`, key).Scan(&f.Key, &f.Enabled, &f.Description, &updatedBy, &f.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("feature_flags get: %w", err)
	}
	f.UpdatedBy = updatedBy
	return &f, nil
}

// Set updates the flag. Refuses to insert new keys via this path — keys must
// originate from migrations (so the catalog of flags lives in source).
func (r *featureFlagRepository) Set(ctx context.Context, key string, enabled bool, updatedBy string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE feature_flags
		SET enabled = $2, updated_by = $3, updated_at = NOW()
		WHERE key = $1
	`, key, enabled, updatedBy)
	if err != nil {
		return fmt.Errorf("feature_flags set: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("feature_flags set: unknown key %q", key)
	}
	return nil
}
