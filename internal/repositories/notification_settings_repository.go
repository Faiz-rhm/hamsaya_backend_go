package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// NotificationSettingsRepository defines the interface for notification settings operations
type NotificationSettingsRepository interface {
	// Settings CRUD
	GetByProfileID(ctx context.Context, profileID string) ([]*models.NotificationSetting, error)
	GetByProfileAndCategory(ctx context.Context, profileID string, category models.NotificationCategory) (*models.NotificationSetting, error)
	Upsert(ctx context.Context, setting *models.NotificationSetting) error
	UpdateCategory(ctx context.Context, profileID string, category models.NotificationCategory, pushPref bool) error

	// Bulk operations
	InitializeDefaults(ctx context.Context, profileID string) error
}

type notificationSettingsRepository struct {
	db *database.DB
}

// NewNotificationSettingsRepository creates a new notification settings repository
func NewNotificationSettingsRepository(db *database.DB) NotificationSettingsRepository {
	return &notificationSettingsRepository{db: db}
}

// GetByProfileID retrieves all notification settings for a profile
func (r *notificationSettingsRepository) GetByProfileID(ctx context.Context, profileID string) ([]*models.NotificationSetting, error) {
	query := `
		SELECT id, profile_id, category, push_pref, created_at, updated_at
		FROM notification_settings
		WHERE profile_id = $1
		ORDER BY category ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification settings: %w", err)
	}
	defer rows.Close()

	var settings []*models.NotificationSetting
	for rows.Next() {
		setting := &models.NotificationSetting{}
		err := rows.Scan(
			&setting.ID,
			&setting.ProfileID,
			&setting.Category,
			&setting.PushPref,
			&setting.CreatedAt,
			&setting.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification setting: %w", err)
		}
		settings = append(settings, setting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notification settings: %w", err)
	}

	return settings, nil
}

// GetByProfileAndCategory retrieves a specific notification setting
func (r *notificationSettingsRepository) GetByProfileAndCategory(ctx context.Context, profileID string, category models.NotificationCategory) (*models.NotificationSetting, error) {
	query := `
		SELECT id, profile_id, category, push_pref, created_at, updated_at
		FROM notification_settings
		WHERE profile_id = $1 AND category = $2
	`

	setting := &models.NotificationSetting{}
	err := r.db.Pool.QueryRow(ctx, query, profileID, category).Scan(
		&setting.ID,
		&setting.ProfileID,
		&setting.Category,
		&setting.PushPref,
		&setting.CreatedAt,
		&setting.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("notification setting not found")
		}
		return nil, fmt.Errorf("failed to get notification setting: %w", err)
	}

	return setting, nil
}

// Upsert creates or updates a notification setting
func (r *notificationSettingsRepository) Upsert(ctx context.Context, setting *models.NotificationSetting) error {
	query := `
		INSERT INTO notification_settings (
			id, profile_id, category, push_pref, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (profile_id, category)
		DO UPDATE SET
			push_pref = EXCLUDED.push_pref,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.Pool.Exec(ctx, query,
		setting.ID,
		setting.ProfileID,
		setting.Category,
		setting.PushPref,
		setting.CreatedAt,
		setting.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert notification setting: %w", err)
	}

	return nil
}

// UpdateCategory updates a specific category setting
func (r *notificationSettingsRepository) UpdateCategory(ctx context.Context, profileID string, category models.NotificationCategory, pushPref bool) error {
	query := `
		UPDATE notification_settings
		SET push_pref = $3, updated_at = $4
		WHERE profile_id = $1 AND category = $2
	`

	result, err := r.db.Pool.Exec(ctx, query, profileID, category, pushPref, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update notification setting: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("notification setting not found")
	}

	return nil
}

// InitializeDefaults creates default notification settings for a new profile
func (r *notificationSettingsRepository) InitializeDefaults(ctx context.Context, profileID string) error {
	categories := []models.NotificationCategory{
		models.NotificationCategoryPosts,
		models.NotificationCategoryMessages,
		models.NotificationCategoryEvents,
		models.NotificationCategorySales,
		models.NotificationCategoryBusiness,
	}

	query := `
		INSERT INTO notification_settings (id, profile_id, category, push_pref, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (profile_id, category) DO NOTHING
	`

	now := time.Now()
	for _, category := range categories {
		setting := &models.NotificationSetting{
			ID:        fmt.Sprintf("%s-%s", profileID, category),
			ProfileID: profileID,
			Category:  category,
			PushPref:  true, // Default to enabled
			CreatedAt: now,
			UpdatedAt: now,
		}

		_, err := r.db.Pool.Exec(ctx, query,
			setting.ID,
			setting.ProfileID,
			setting.Category,
			setting.PushPref,
			setting.CreatedAt,
			setting.UpdatedAt,
		)

		if err != nil {
			return fmt.Errorf("failed to initialize default setting for category %s: %w", category, err)
		}
	}

	return nil
}
