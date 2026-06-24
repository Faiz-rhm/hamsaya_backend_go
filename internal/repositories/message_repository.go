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
	// Delete soft-deletes for everyone (sets deleted_at). Sender-only.
	Delete(ctx context.Context, messageID string) error
	// DeleteForUser appends the user to deleted_for_user_ids so the message
	// disappears for that user only; other participants still see it.
	DeleteForUser(ctx context.Context, messageID, userID string) error

	// Read receipts
	MarkAsRead(ctx context.Context, messageID string) error
	MarkConversationAsRead(ctx context.Context, conversationID, userID string) error

	// Unread counts. viewerID is the requesting user — excludes messages
	// the user has individually delete-for-me'd so the badge matches their
	// thread view.
	GetUnreadCount(ctx context.Context, conversationID, userID string) (int, error)

	// Get last message in conversation. viewerID excludes per-user-deleted
	// rows so the conversation list preview reflects what the viewer sees.
	GetLastMessage(ctx context.Context, conversationID, viewerID string) (*models.Message, error)

	// Reactions
	AddReaction(ctx context.Context, messageID, userID, emoji string) error
	RemoveReaction(ctx context.Context, messageID, userID, emoji string) error
	// GetReactions aggregates reactions for the given messages into
	// per-emoji {emoji,count,reacted} lists keyed by message id. `reacted` is
	// relative to viewerID.
	GetReactions(ctx context.Context, messageIDs []string, viewerID string) (map[string][]models.MessageReaction, error)
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
			id, conversation_id, sender_id, content, message_type, product_id, reply_to_message_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		message.ID,
		message.ConversationID,
		message.SenderID,
		message.Content,
		message.MessageType,
		message.ProductID,
		message.ReplyToMessageID,
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
		SELECT id, conversation_id, sender_id, content, message_type, product_id, reply_to_message_id, read_at, created_at, deleted_at
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
		&message.ProductID,
		&message.ReplyToMessageID,
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

// List retrieves messages in a conversation, excluding messages the viewer
// has individually delete-for-me'd. Messages deleted-for-everyone are
// already filtered via `deleted_at IS NULL`.
func (r *messageRepository) List(ctx context.Context, filter *models.GetMessagesFilter) ([]*models.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, message_type, product_id, reply_to_message_id, read_at, created_at, deleted_at
		FROM messages
		WHERE conversation_id = $1
		  AND deleted_at IS NULL
		  AND NOT ($2::uuid = ANY(deleted_for_user_ids))
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Pool.Query(ctx, query, filter.ConversationID, filter.ViewerID, filter.Limit, filter.Offset)
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
			&message.ProductID,
			&message.ReplyToMessageID,
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

// Delete soft deletes a message for everyone (sets deleted_at).
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

// DeleteForUser hides the message for one user only by appending their id to
// the deleted_for_user_ids array. Idempotent — re-appending the same id is a
// no-op thanks to array_append-with-distinct-check.
func (r *messageRepository) DeleteForUser(ctx context.Context, messageID, userID string) error {
	query := `
		UPDATE messages
		SET deleted_for_user_ids = (
			SELECT ARRAY(SELECT DISTINCT unnest(array_append(deleted_for_user_ids, $2::uuid)))
		)
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, messageID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete message for user: %w", err)
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
		  AND NOT ($2::uuid = ANY(deleted_for_user_ids))
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, conversationID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// GetLastMessage retrieves the last message in a conversation that the
// viewer can still see (i.e. not in their per-user delete list).
func (r *messageRepository) GetLastMessage(ctx context.Context, conversationID, viewerID string) (*models.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, message_type, product_id, reply_to_message_id, read_at, created_at, deleted_at
		FROM messages
		WHERE conversation_id = $1
		  AND deleted_at IS NULL
		  AND NOT ($2::uuid = ANY(deleted_for_user_ids))
		ORDER BY created_at DESC
		LIMIT 1
	`

	message := &models.Message{}
	err := r.db.Pool.QueryRow(ctx, query, conversationID, viewerID).Scan(
		&message.ID,
		&message.ConversationID,
		&message.SenderID,
		&message.Content,
		&message.MessageType,
		&message.ProductID,
		&message.ReplyToMessageID,
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

// AddReaction adds an emoji reaction (idempotent — duplicate (message,user,emoji) is a no-op).
func (r *messageRepository) AddReaction(ctx context.Context, messageID, userID, emoji string) error {
	query := `
		INSERT INTO message_reactions (message_id, user_id, emoji)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id, emoji) DO NOTHING
	`
	if _, err := r.db.Pool.Exec(ctx, query, messageID, userID, emoji); err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}
	return nil
}

// RemoveReaction removes a user's specific emoji reaction.
func (r *messageRepository) RemoveReaction(ctx context.Context, messageID, userID, emoji string) error {
	query := `DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`
	if _, err := r.db.Pool.Exec(ctx, query, messageID, userID, emoji); err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}
	return nil
}

// GetReactions aggregates reactions for a set of messages, keyed by message id.
func (r *messageRepository) GetReactions(ctx context.Context, messageIDs []string, viewerID string) (map[string][]models.MessageReaction, error) {
	out := make(map[string][]models.MessageReaction)
	if len(messageIDs) == 0 {
		return out, nil
	}
	query := `
		SELECT message_id, emoji, COUNT(*) AS cnt,
		       BOOL_OR(user_id = $2) AS reacted
		FROM message_reactions
		WHERE message_id = ANY($1)
		GROUP BY message_id, emoji
		ORDER BY MIN(created_at)
	`
	rows, err := r.db.Pool.Query(ctx, query, messageIDs, viewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reactions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mid string
		var rx models.MessageReaction
		if err := rows.Scan(&mid, &rx.Emoji, &rx.Count, &rx.Reacted); err != nil {
			return nil, fmt.Errorf("failed to scan reaction: %w", err)
		}
		out[mid] = append(out[mid], rx)
	}
	return out, rows.Err()
}
