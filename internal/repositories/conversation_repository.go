package repositories

import (
	"context"
	"fmt"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// ConversationRepository defines the interface for conversation operations
type ConversationRepository interface {
	// Create or get existing conversation. businessID nil = personal user-to-user
	// chat; non-nil = business-scoped chat.
	GetOrCreate(ctx context.Context, userID1, userID2 string, businessID *string) (*models.Conversation, error)
	GetByID(ctx context.Context, conversationID string) (*models.Conversation, error)
	GetByParticipants(ctx context.Context, userID1, userID2 string, businessID *string) (*models.Conversation, error)
	List(ctx context.Context, filter *models.GetConversationsFilter) ([]*models.Conversation, error)
	UpdateLastMessageAt(ctx context.Context, conversationID string) error
	Delete(ctx context.Context, conversationID string) error

	// Participant checks
	IsParticipant(ctx context.Context, conversationID, userID string) (bool, error)
	GetOtherParticipantID(ctx context.Context, conversationID, userID string) (string, error)
}

type conversationRepository struct {
	db *database.DB
}

// NewConversationRepository creates a new conversation repository
func NewConversationRepository(db *database.DB) ConversationRepository {
	return &conversationRepository{db: db}
}

// GetOrCreate gets an existing conversation or creates a new one
func (r *conversationRepository) GetOrCreate(ctx context.Context, userID1, userID2 string, businessID *string) (*models.Conversation, error) {
	// Ensure participant1 < participant2 for consistency
	participant1, participant2 := userID1, userID2
	if userID1 > userID2 {
		participant1, participant2 = userID2, userID1
	}

	// Try to get existing conversation
	existing, err := r.GetByParticipants(ctx, participant1, participant2, businessID)
	if err == nil {
		return existing, nil
	}

	// Create new conversation
	query := `
		INSERT INTO conversations (participant1_id, participant2_id, business_id, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, participant1_id, participant2_id, business_id, last_message_at, created_at
	`

	conversation := &models.Conversation{}
	err = r.db.Pool.QueryRow(ctx, query, participant1, participant2, businessID).Scan(
		&conversation.ID,
		&conversation.Participant1ID,
		&conversation.Participant2ID,
		&conversation.BusinessID,
		&conversation.LastMessageAt,
		&conversation.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conversation, nil
}

// GetByID retrieves a conversation by ID
func (r *conversationRepository) GetByID(ctx context.Context, conversationID string) (*models.Conversation, error) {
	query := `
		SELECT id, participant1_id, participant2_id, business_id, last_message_at, created_at
		FROM conversations
		WHERE id = $1
	`

	conversation := &models.Conversation{}
	err := r.db.Pool.QueryRow(ctx, query, conversationID).Scan(
		&conversation.ID,
		&conversation.Participant1ID,
		&conversation.Participant2ID,
		&conversation.BusinessID,
		&conversation.LastMessageAt,
		&conversation.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return conversation, nil
}

// GetByParticipants retrieves a conversation by its participants and optional business scope.
// businessID nil → matches the personal chat (business_id IS NULL).
func (r *conversationRepository) GetByParticipants(ctx context.Context, userID1, userID2 string, businessID *string) (*models.Conversation, error) {
	// Ensure participant1 < participant2
	participant1, participant2 := userID1, userID2
	if userID1 > userID2 {
		participant1, participant2 = userID2, userID1
	}

	var query string
	var args []interface{}
	if businessID == nil {
		query = `
			SELECT id, participant1_id, participant2_id, business_id, last_message_at, created_at
			FROM conversations
			WHERE participant1_id = $1 AND participant2_id = $2 AND business_id IS NULL
		`
		args = []interface{}{participant1, participant2}
	} else {
		query = `
			SELECT id, participant1_id, participant2_id, business_id, last_message_at, created_at
			FROM conversations
			WHERE participant1_id = $1 AND participant2_id = $2 AND business_id = $3
		`
		args = []interface{}{participant1, participant2, *businessID}
	}

	conversation := &models.Conversation{}
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(
		&conversation.ID,
		&conversation.Participant1ID,
		&conversation.Participant2ID,
		&conversation.BusinessID,
		&conversation.LastMessageAt,
		&conversation.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return conversation, nil
}

// List retrieves all conversations for a user. When BusinessID is nil, returns
// only personal chats (business_id IS NULL). When non-nil, returns chats scoped
// to that business.
func (r *conversationRepository) List(ctx context.Context, filter *models.GetConversationsFilter) ([]*models.Conversation, error) {
	var query string
	var args []interface{}
	if filter.BusinessID == nil {
		query = `
			SELECT id, participant1_id, participant2_id, business_id, last_message_at, created_at
			FROM conversations
			WHERE (participant1_id = $1 OR participant2_id = $1) AND business_id IS NULL
			ORDER BY COALESCE(last_message_at, created_at) DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{filter.UserID, filter.Limit, filter.Offset}
	} else {
		query = `
			SELECT id, participant1_id, participant2_id, business_id, last_message_at, created_at
			FROM conversations
			WHERE (participant1_id = $1 OR participant2_id = $1) AND business_id = $2
			ORDER BY COALESCE(last_message_at, created_at) DESC
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{filter.UserID, *filter.BusinessID, filter.Limit, filter.Offset}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*models.Conversation
	for rows.Next() {
		conversation := &models.Conversation{}
		err := rows.Scan(
			&conversation.ID,
			&conversation.Participant1ID,
			&conversation.Participant2ID,
			&conversation.BusinessID,
			&conversation.LastMessageAt,
			&conversation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, conversation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversations: %w", err)
	}

	return conversations, nil
}

// UpdateLastMessageAt updates the last_message_at timestamp for a conversation
func (r *conversationRepository) UpdateLastMessageAt(ctx context.Context, conversationID string) error {
	query := `
		UPDATE conversations
		SET last_message_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, conversationID)
	if err != nil {
		return fmt.Errorf("failed to update last_message_at: %w", err)
	}

	return nil
}

// Delete deletes a conversation (soft delete could be added later)
func (r *conversationRepository) Delete(ctx context.Context, conversationID string) error {
	query := `DELETE FROM conversations WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("conversation not found")
	}

	return nil
}

// IsParticipant checks if a user is a participant in a conversation
func (r *conversationRepository) IsParticipant(ctx context.Context, conversationID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM conversations
			WHERE id = $1 AND (participant1_id = $2 OR participant2_id = $2)
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, conversationID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check participant: %w", err)
	}

	return exists, nil
}

// GetOtherParticipantID gets the other participant's user ID
func (r *conversationRepository) GetOtherParticipantID(ctx context.Context, conversationID, userID string) (string, error) {
	query := `
		SELECT
			CASE
				WHEN participant1_id = $2 THEN participant2_id
				WHEN participant2_id = $2 THEN participant1_id
				ELSE NULL
			END as other_participant_id
		FROM conversations
		WHERE id = $1
	`

	var otherParticipantID *string
	err := r.db.Pool.QueryRow(ctx, query, conversationID, userID).Scan(&otherParticipantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("conversation not found")
		}
		return "", fmt.Errorf("failed to get other participant: %w", err)
	}

	if otherParticipantID == nil {
		return "", fmt.Errorf("user is not a participant")
	}

	return *otherParticipantID, nil
}
