package repositories

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// EventRepository defines the interface for event interest operations
type EventRepository interface {
	// Event interest operations
	SetInterest(ctx context.Context, interest *models.EventInterest) error
	GetUserInterest(ctx context.Context, userID, postID string) (*models.EventInterest, error)
	DeleteInterest(ctx context.Context, userID, postID string) error

	// Get interested/going users
	GetInterestedUsers(ctx context.Context, postID string, state models.EventInterestState, limit, offset int) ([]*models.EventInterest, error)
	CountByState(ctx context.Context, postID string, state models.EventInterestState) (int, error)
}

type eventRepository struct {
	db *database.DB
}

// NewEventRepository creates a new event repository
func NewEventRepository(db *database.DB) EventRepository {
	return &eventRepository{db: db}
}

// SetInterest sets or updates a user's interest in an event
func (r *eventRepository) SetInterest(ctx context.Context, interest *models.EventInterest) error {
	query := `
		INSERT INTO event_interests (id, post_id, user_id, event_state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (post_id, user_id)
		DO UPDATE SET
			event_state = EXCLUDED.event_state,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.Pool.Exec(ctx, query,
		interest.ID,
		interest.PostID,
		interest.UserID,
		interest.EventState,
		interest.CreatedAt,
		interest.UpdatedAt,
	)

	return err
}

// GetUserInterest gets a user's interest in an event
func (r *eventRepository) GetUserInterest(ctx context.Context, userID, postID string) (*models.EventInterest, error) {
	query := `
		SELECT id, post_id, user_id, event_state, created_at, updated_at
		FROM event_interests
		WHERE user_id = $1 AND post_id = $2
	`

	interest := &models.EventInterest{}
	err := r.db.Pool.QueryRow(ctx, query, userID, postID).Scan(
		&interest.ID,
		&interest.PostID,
		&interest.UserID,
		&interest.EventState,
		&interest.CreatedAt,
		&interest.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not an error, user just hasn't expressed interest
	}

	return interest, err
}

// DeleteInterest removes a user's interest from an event
func (r *eventRepository) DeleteInterest(ctx context.Context, userID, postID string) error {
	query := `
		DELETE FROM event_interests
		WHERE user_id = $1 AND post_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, postID)
	return err
}

// GetInterestedUsers gets users who expressed interest in an event
func (r *eventRepository) GetInterestedUsers(ctx context.Context, postID string, state models.EventInterestState, limit, offset int) ([]*models.EventInterest, error) {
	query := `
		SELECT id, post_id, user_id, event_state, created_at, updated_at
		FROM event_interests
		WHERE post_id = $1 AND event_state = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Pool.Query(ctx, query, postID, state, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interests []*models.EventInterest
	for rows.Next() {
		interest := &models.EventInterest{}
		err := rows.Scan(
			&interest.ID,
			&interest.PostID,
			&interest.UserID,
			&interest.EventState,
			&interest.CreatedAt,
			&interest.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		interests = append(interests, interest)
	}

	return interests, rows.Err()
}

// CountByState counts users by their interest state
func (r *eventRepository) CountByState(ctx context.Context, postID string, state models.EventInterestState) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM event_interests
		WHERE post_id = $1 AND event_state = $2
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, postID, state).Scan(&count)
	return count, err
}
