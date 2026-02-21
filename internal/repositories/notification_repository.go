package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// NotificationRepository defines the interface for notification operations
type NotificationRepository interface {
	// Notification CRUD
	Create(ctx context.Context, notification *models.Notification) error
	GetByID(ctx context.Context, notificationID string) (*models.Notification, error)
	List(ctx context.Context, filter *models.GetNotificationsFilter) ([]*models.Notification, error)
	MarkAsRead(ctx context.Context, notificationID string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, notificationID string) error

	// Unread count. When businessID is set, count only notifications for that business.
	GetUnreadCount(ctx context.Context, userID string, businessID *string) (int, error)
}

type notificationRepository struct {
	db *database.DB
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *database.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

// Create creates a new notification
func (r *notificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %w", err)
	}

	query := `
		INSERT INTO notifications (
			id, user_id, type, title, message, data, read, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = r.db.Pool.Exec(ctx, query,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Message,
		dataJSON,
		notification.Read,
		notification.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// GetByID retrieves a notification by ID
func (r *notificationRepository) GetByID(ctx context.Context, notificationID string) (*models.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, data, read, created_at
		FROM notifications
		WHERE id = $1
	`

	notification := &models.Notification{}
	var dataJSON []byte

	err := r.db.Pool.QueryRow(ctx, query, notificationID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&dataJSON,
		&notification.Read,
		&notification.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	// Unmarshal data
	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal notification data: %w", err)
		}
	}

	return notification, nil
}

// List retrieves notifications with filters
func (r *notificationRepository) List(ctx context.Context, filter *models.GetNotificationsFilter) ([]*models.Notification, error) {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT id, user_id, type, title, message, data, read, created_at
		FROM notifications
		WHERE user_id = $1
	`)

	args := []interface{}{filter.UserID}
	argCount := 2

	// Apply type filter
	if filter.Type != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND type = $%d", argCount))
		args = append(args, *filter.Type)
		argCount++
	}

	// Apply unread filter
	if filter.UnreadOnly {
		queryBuilder.WriteString(" AND read = false")
	}

	// Business scope: when filter.BusinessID is set, only that business's notifications;
	// when not set (user feed), show user-level notifications AND NEW_POST (so "Faiz store posted" appears in main list).
	if filter.BusinessID != nil && *filter.BusinessID != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND data->>'business_id' = $%d", argCount))
		args = append(args, *filter.BusinessID)
		argCount++
	} else {
		queryBuilder.WriteString(" AND (data->>'business_id' IS NULL OR data->>'business_id' = '' OR type = 'NEW_POST')")
	}

	// Order by created_at DESC
	queryBuilder.WriteString(" ORDER BY created_at DESC")

	// Pagination
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1))
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Pool.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*models.Notification
	for rows.Next() {
		notification := &models.Notification{}
		var dataJSON []byte

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&notification.Title,
			&notification.Message,
			&dataJSON,
			&notification.Read,
			&notification.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		// Unmarshal data
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal notification data: %w", err)
			}
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, nil
}

// MarkAsRead marks a notification as read
func (r *notificationRepository) MarkAsRead(ctx context.Context, notificationID string) error {
	query := `
		UPDATE notifications
		SET read = true
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, notificationID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// MarkAllAsRead marks all notifications for a user as read
func (r *notificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	query := `
		UPDATE notifications
		SET read = true
		WHERE user_id = $1 AND read = false
	`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// Delete deletes a notification
func (r *notificationRepository) Delete(ctx context.Context, notificationID string) error {
	query := `DELETE FROM notifications WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, notificationID)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// GetUnreadCount gets the count of unread notifications for a user.
// When businessID is set, counts only notifications for that business.
// When businessID is nil, counts user-level and NEW_POST (so badge matches main list including "X posted").
func (r *notificationRepository) GetUnreadCount(ctx context.Context, userID string, businessID *string) (int, error) {
	var query string
	var args []interface{}
	if businessID != nil && *businessID != "" {
		query = `
			SELECT COUNT(*)
			FROM notifications
			WHERE user_id = $1 AND read = false
			  AND data->>'business_id' = $2
		`
		args = []interface{}{userID, *businessID}
	} else {
		query = `
			SELECT COUNT(*)
			FROM notifications
			WHERE user_id = $1 AND read = false
			  AND (data->>'business_id' IS NULL OR data->>'business_id' = '' OR type = 'NEW_POST')
		`
		args = []interface{}{userID}
	}

	var count int
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}
