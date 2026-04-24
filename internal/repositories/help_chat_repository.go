package repositories

import (
	"context"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// HelpChatRepository handles help_chat_messages persistence.
type HelpChatRepository interface {
	CreateMessage(ctx context.Context, msg *models.HelpChatMessage) error
	GetMessages(ctx context.Context, userID string, limit, offset int) ([]*models.HelpChatMessage, int64, error)
	GetAllUserThreads(ctx context.Context, limit, offset int) ([]*models.HelpChatThread, int64, error)
	GetUserMessages(ctx context.Context, userID string, limit, offset int) ([]*models.HelpChatMessage, int64, error)
}

type helpChatRepository struct {
	db *database.DB
}

// NewHelpChatRepository creates a new HelpChatRepository.
func NewHelpChatRepository(db *database.DB) HelpChatRepository {
	return &helpChatRepository{db: db}
}

func (r *helpChatRepository) CreateMessage(ctx context.Context, msg *models.HelpChatMessage) error {
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO help_chat_messages (user_id, content, is_from_user, app_version, device_info, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, msg.UserID, msg.Content, msg.IsFromUser, msg.AppVersion, msg.DeviceInfo, time.Now()).
		Scan(&msg.ID, &msg.CreatedAt)
	return err
}

func (r *helpChatRepository) GetMessages(ctx context.Context, userID string, limit, offset int) ([]*models.HelpChatMessage, int64, error) {
	var total int64
	_ = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM help_chat_messages WHERE user_id = $1`, userID).Scan(&total)

	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, content, is_from_user, app_version, device_info, created_at
		FROM help_chat_messages
		WHERE user_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var msgs []*models.HelpChatMessage
	for rows.Next() {
		m := &models.HelpChatMessage{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.IsFromUser, &m.AppVersion, &m.DeviceInfo, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		msgs = append(msgs, m)
	}
	return msgs, total, rows.Err()
}

// GetAllUserThreads returns the latest message per user for the admin inbox view.
func (r *helpChatRepository) GetAllUserThreads(ctx context.Context, limit, offset int) ([]*models.HelpChatThread, int64, error) {
	var total int64
	_ = r.db.Pool.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM help_chat_messages`).Scan(&total)

	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (h.user_id)
			h.user_id,
			COALESCE(p.first_name, '') || ' ' || COALESCE(p.last_name, '') AS full_name,
			u.email,
			h.content AS last_message,
			h.is_from_user AS last_is_from_user,
			h.created_at AS last_at
		FROM help_chat_messages h
		JOIN users u ON h.user_id = u.id
		LEFT JOIN profiles p ON h.user_id = p.id
		ORDER BY h.user_id, h.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var threads []*models.HelpChatThread
	for rows.Next() {
		t := &models.HelpChatThread{}
		if err := rows.Scan(&t.UserID, &t.FullName, &t.Email, &t.LastMessage, &t.LastIsFromUser, &t.LastAt); err != nil {
			return nil, 0, err
		}
		threads = append(threads, t)
	}
	return threads, total, rows.Err()
}

func (r *helpChatRepository) GetUserMessages(ctx context.Context, userID string, limit, offset int) ([]*models.HelpChatMessage, int64, error) {
	return r.GetMessages(ctx, userID, limit, offset)
}
