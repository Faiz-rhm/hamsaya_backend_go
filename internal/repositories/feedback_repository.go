package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"go.uber.org/zap"
)

// FeedbackRepository defines the interface for feedback operations
type FeedbackRepository interface {
	Create(ctx context.Context, feedback *models.Feedback) error
	GetUserFeedbackStatus(ctx context.Context, userID string) (bool, *time.Time, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.Feedback, error)
}

type feedbackRepository struct {
	db     *database.DB
	logger *zap.SugaredLogger
}

// NewFeedbackRepository creates a new feedback repository
func NewFeedbackRepository(db *database.DB) FeedbackRepository {
	return &feedbackRepository{
		db:     db,
		logger: utils.GetLogger(),
	}
}

func (r *feedbackRepository) Create(ctx context.Context, feedback *models.Feedback) error {
	feedback.ID = uuid.New().String()
	feedback.CreatedAt = time.Now()

	r.logger.Infow("Creating user feedback",
		"feedback_id", feedback.ID,
		"user_id", feedback.UserID,
		"rating", feedback.Rating,
		"type", feedback.Type,
	)

	query := `
		INSERT INTO user_feedback (id, user_id, rating, type, message, app_version, device_info, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		feedback.ID,
		feedback.UserID,
		feedback.Rating,
		feedback.Type,
		feedback.Message,
		feedback.AppVersion,
		feedback.DeviceInfo,
		feedback.CreatedAt,
	)

	if err != nil {
		r.logger.Errorw("Failed to create feedback", "error", err)
		return err
	}

	r.logger.Infow("Feedback created successfully", "feedback_id", feedback.ID)
	return nil
}

func (r *feedbackRepository) GetUserFeedbackStatus(ctx context.Context, userID string) (bool, *time.Time, error) {
	query := `
		SELECT created_at 
		FROM user_feedback 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT 1
	`

	var lastFeedback time.Time
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&lastFeedback)
	
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil, nil
		}
		r.logger.Errorw("Failed to get feedback status", "user_id", userID, "error", err)
		return false, nil, err
	}

	return true, &lastFeedback, nil
}

func (r *feedbackRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.Feedback, error) {
	query := `
		SELECT id, user_id, rating, type, message, app_version, device_info, created_at
		FROM user_feedback
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		r.logger.Errorw("Failed to get user feedback", "user_id", userID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var feedbacks []*models.Feedback
	for rows.Next() {
		var f models.Feedback
		err := rows.Scan(
			&f.ID,
			&f.UserID,
			&f.Rating,
			&f.Type,
			&f.Message,
			&f.AppVersion,
			&f.DeviceInfo,
			&f.CreatedAt,
		)
		if err != nil {
			r.logger.Errorw("Failed to scan feedback row", "error", err)
			continue
		}
		feedbacks = append(feedbacks, &f)
	}

	return feedbacks, nil
}
