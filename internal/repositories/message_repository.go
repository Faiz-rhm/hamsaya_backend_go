package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// MessageRepository defines the interface for message operations
type MessageRepository interface {
	// Message CRUD
	Create(ctx context.Context, message *models.Message) error
	GetByID(ctx context.Context, messageID string) (*models.Message, error)
	List(ctx context.Context, filter *models.GetMessagesFilter) ([]*models.Message, error)
	Delete(ctx context.Context, messageID string) error

	// Read receipts
	MarkAsRead(ctx context.Context, messageID string) error
	MarkConversationAsRead(ctx context.Context, conversationID, userID string) error

	// Unread counts
	GetUnreadCount(ctx context.Context, conversationID, userID string) (int, error)

	// Get last message in conversation
	GetLastMessage(ctx context.Context, conversationID string) (*models.Message, error)
}

type messageRepository struct {
	db *database.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *database.DB) MessageRepository {
	return &messageRepository{db: db}
}

// Create creates a new message
func (r *messageRepository) Create(ctx context.Context, message *models.Message) error {
	query := `
		INSERT INTO messages (
			id, conversation_id, sender_id, content, message_type, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		message.ID,
		message.ConversationID,
		message.SenderID,
		message.Content,
		message.MessageType,
		message.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	return nil
}

// GetByID retrieves a message by ID
func (r *messageRepository) GetByID(ctx context.Context, messageID string) (*models.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, message_type, read_at, created_at, deleted_at
		FROM messages
		WHERE id = $1 AND deleted_at IS NULL
	`

	message := &models.Message{}
	err := r.db.Pool.QueryRow(ctx, query, messageID).Scan(
		&message.ID,
		&message.ConversationID,
		&message.SenderID,
		&message.Content,
		&message.MessageType,
		&message.ReadAt,
		&message.CreatedAt,
		&message.DeletedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return message, nil
}

// List retrieves messages in a conversation
func (r *messageRepository) List(ctx context.Context, filter *models.GetMessagesFilter) ([]*models.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, message_type, read_at, created_at, deleted_at
		FROM messages
		WHERE conversation_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, filter.ConversationID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		message := &models.Message{}
		err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.SenderID,
			&message.Content,
			&message.MessageType,
			&message.ReadAt,
			&message.CreatedAt,
			&message.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// Delete soft deletes a message
func (r *messageRepository) Delete(ctx context.Context, messageID string) error {
	query := `
		UPDATE messages
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, messageID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("message not found")
	}

	return nil
}

// MarkAsRead marks a message as read
func (r *messageRepository) MarkAsRead(ctx context.Context, messageID string) error {
	query := `
		UPDATE messages
		SET read_at = NOW()
		WHERE id = $1 AND read_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as read: %w", err)
	}

	return nil
}

// MarkConversationAsRead marks all unread messages in a conversation as read for a user
func (r *messageRepository) MarkConversationAsRead(ctx context.Context, conversationID, userID string) error {
	query := `
		UPDATE messages
		SET read_at = NOW()
		WHERE conversation_id = $1
		  AND sender_id != $2
		  AND read_at IS NULL
		  AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, conversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark conversation as read: %w", err)
	}

	return nil
}

// GetUnreadCount gets the count of unread messages in a conversation for a user
func (r *messageRepository) GetUnreadCount(ctx context.Context, conversationID, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM messages
		WHERE conversation_id = $1
		  AND sender_id != $2
		  AND read_at IS NULL
		  AND deleted_at IS NULL
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, conversationID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// GetLastMessage retrieves the last message in a conversation
func (r *messageRepository) GetLastMessage(ctx context.Context, conversationID string) (*models.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, message_type, read_at, created_at, deleted_at
		FROM messages
		WHERE conversation_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	message := &models.Message{}
	err := r.db.Pool.QueryRow(ctx, query, conversationID).Scan(
		&message.ID,
		&message.ConversationID,
		&message.SenderID,
		&message.Content,
		&message.MessageType,
		&message.ReadAt,
		&message.CreatedAt,
		&message.DeletedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No messages yet
		}
		return nil, fmt.Errorf("failed to get last message: %w", err)
	}

	return message, nil
}
