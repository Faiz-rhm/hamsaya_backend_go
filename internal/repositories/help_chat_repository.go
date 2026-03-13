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

// HelpChatRepository defines the interface for help chat message operations.
type HelpChatRepository interface {
	Create(ctx context.Context, msg *models.HelpChatMessage) error
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.HelpChatMessage, error)
}

type helpChatRepository struct {
	db     *database.DB
	logger *zap.SugaredLogger
}

// NewHelpChatRepository creates a new help chat repository.
func NewHelpChatRepository(db *database.DB) HelpChatRepository {
	return &helpChatRepository{
		db:     db,
		logger: utils.GetLogger(),
	}
}

func (r *helpChatRepository) Create(ctx context.Context, msg *models.HelpChatMessage) error {
	msg.ID = uuid.New().String()
	msg.CreatedAt = time.Now()

	r.logger.Infow("Creating help chat message",
		"message_id", msg.ID,
		"user_id", msg.UserID,
		"is_from_user", msg.IsFromUser,
	)

	query := `
		INSERT INTO help_chat_messages (id, user_id, content, is_from_user, app_version, device_info, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		msg.ID,
		msg.UserID,
		msg.Content,
		msg.IsFromUser,
		msg.AppVersion,
		msg.DeviceInfo,
		msg.CreatedAt,
	)

	if err != nil {
		r.logger.Errorw("Failed to create help chat message", "error", err)
		return err
	}

	r.logger.Infow("Help chat message created", "message_id", msg.ID)
	return nil
}

func (r *helpChatRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.HelpChatMessage, error) {
	query := `
		SELECT id, user_id, content, is_from_user, app_version, device_info, created_at
		FROM help_chat_messages
		WHERE user_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		r.logger.Errorw("Failed to list help chat messages", "user_id", userID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var messages []*models.HelpChatMessage
	for rows.Next() {
		var m models.HelpChatMessage
		err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.Content,
			&m.IsFromUser,
			&m.AppVersion,
			&m.DeviceInfo,
			&m.CreatedAt,
		)
		if err != nil {
			r.logger.Errorw("Failed to scan help chat message row", "error", err)
			continue
		}
		messages = append(messages, &m)
	}

	return messages, nil
}
