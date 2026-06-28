package models

import "time"

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeText     MessageType = "TEXT"
	MessageTypeImage    MessageType = "IMAGE"
	MessageTypeFile     MessageType = "FILE"
	MessageTypeLocation MessageType = "LOCATION"
	MessageTypeVoice    MessageType = "VOICE"
)

// Conversation represents a chat conversation between two users (optionally
// scoped to a business so a customer can have a separate thread per business).
type Conversation struct {
	ID             string     `json:"id"`
	Participant1ID string     `json:"participant1_id"`
	Participant2ID string     `json:"participant2_id"`
	BusinessID     *string    `json:"business_id,omitempty"`
	LastMessageAt  *time.Time `json:"last_message_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Message represents a chat message
type Message struct {
	ID               string      `json:"id"`
	ConversationID   string      `json:"conversation_id"`
	SenderID         string      `json:"sender_id"`
	Content          *string     `json:"content"`
	MessageType      MessageType `json:"message_type"`
	ProductID        *string     `json:"product_id,omitempty"`
	ReplyToMessageID *string     `json:"reply_to_message_id,omitempty"`
	ReadAt           *time.Time  `json:"read_at,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	DeletedAt        *time.Time  `json:"deleted_at,omitempty"`
}

// MessageReplyPreview is the quoted message shown above a reply.
type MessageReplyPreview struct {
	ID          string      `json:"id"`
	SenderID    string      `json:"sender_id"`
	Content     *string     `json:"content"`
	MessageType MessageType `json:"message_type"`
}

// MessageReaction is an aggregated emoji reaction on a message.
type MessageReaction struct {
	Emoji   string `json:"emoji"`
	Count   int    `json:"count"`
	Reacted bool   `json:"reacted"` // true if the requesting user reacted with this emoji
}

// ConversationResponse is the API response for a conversation
type ConversationResponse struct {
	ID               string              `json:"id"`
	OtherParticipant *UserInfo           `json:"other_participant"`
	Business         *ConversationBizRef `json:"business,omitempty"`
	LastMessage      *MessageInfo        `json:"last_message,omitempty"`
	UnreadCount      int                 `json:"unread_count"`
	LastMessageAt    *time.Time          `json:"last_message_at"`
	CreatedAt        time.Time           `json:"created_at"`
}

// ConversationBizRef is a brief business reference shown next to a conversation
// when the chat is scoped to a business.
type ConversationBizRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Avatar *Photo  `json:"avatar,omitempty"`
}

// MessageResponse is the API response for a message
type MessageResponse struct {
	ID             string               `json:"id"`
	ConversationID string               `json:"conversation_id"`
	Sender         *UserInfo            `json:"sender"`
	Content        *string              `json:"content"`
	MessageType    MessageType          `json:"message_type"`
	ProductID      *string              `json:"product_id,omitempty"`
	ReplyTo        *MessageReplyPreview `json:"reply_to,omitempty"`
	Reactions      []MessageReaction    `json:"reactions,omitempty"`
	IsRead         bool                 `json:"is_read"`
	CreatedAt      time.Time            `json:"created_at"`
}

// MessageInfo is a brief message summary for conversation lists
type MessageInfo struct {
	ID          string      `json:"id"`
	Content     *string     `json:"content"`
	MessageType MessageType `json:"message_type"`
	SenderID    string      `json:"sender_id"`
	CreatedAt   time.Time   `json:"created_at"`
}

// UserInfo represents brief user information for chat
type UserInfo struct {
	UserID      string  `json:"user_id"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	FullName    string  `json:"full_name"`
	Avatar      *Photo  `json:"avatar,omitempty"`
	AvatarColor *string `json:"avatar_color,omitempty"`
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	RecipientID      string      `json:"recipient_id" validate:"required,uuid"`
	Content          *string     `json:"content,omitempty" validate:"omitempty,min=1,max=5000"`
	MessageType      MessageType `json:"message_type" validate:"required"`
	ProductID        *string     `json:"product_id,omitempty" validate:"omitempty,uuid"`
	BusinessID       *string     `json:"business_id,omitempty" validate:"omitempty,uuid"`
	ReplyToMessageID *string     `json:"reply_to_message_id,omitempty" validate:"omitempty,uuid"`
}

// ReactToMessageRequest toggles an emoji reaction on a message.
type ReactToMessageRequest struct {
	Emoji string `json:"emoji" validate:"required,min=1,max=16"`
}

// GetConversationsFilter represents filters for listing conversations
type GetConversationsFilter struct {
	UserID     string
	BusinessID *string // nil = personal chats only; non-nil = chats scoped to that business
	Limit      int
	Offset     int
}

// GetMessagesFilter represents filters for listing messages.
// ViewerID is set so per-user soft-deleted rows (delete-for-me) are excluded
// for the requesting user while remaining visible to other participants.
type GetMessagesFilter struct {
	ConversationID string
	ViewerID       string
	Limit          int
	Offset         int
}

// WSMessage represents a WebSocket message for real-time communication
type WSMessage struct {
	Type    string      `json:"type"` // "message", "typing", "read", "error"
	Payload interface{} `json:"payload"`
}

// WSMessagePayload represents the payload for a new message over WebSocket.
// BusinessID identifies which scope the conversation belongs to. The mobile
// client uses it to invalidate the correct conversationsProvider (personal
// vs business-scoped); without it, business-chat unread badges never update
// in real time.
type WSMessagePayload struct {
	ConversationID string      `json:"conversation_id"`
	MessageID      string      `json:"message_id"`
	SenderID       string      `json:"sender_id"`
	BusinessID     *string     `json:"business_id,omitempty"`
	Content        *string     `json:"content"`
	MessageType    MessageType `json:"message_type"`
	CreatedAt      time.Time   `json:"created_at"`
}

// WSMessageDeletedPayload notifies the other participant that a message was
// deleted-for-everyone so their UI can remove the bubble in real time.
// Fired only on "delete for everyone" — "delete for me" is local to the
// initiator and never broadcast.
type WSMessageDeletedPayload struct {
	ConversationID string  `json:"conversation_id"`
	MessageID      string  `json:"message_id"`
	BusinessID     *string `json:"business_id,omitempty"`
}

// WSReactionPayload notifies the other participant that a reaction was added or
// removed on a message, so their UI updates the bubble's reactions in real time.
type WSReactionPayload struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	UserID         string `json:"user_id"`
	Emoji          string `json:"emoji"`
	Added          bool   `json:"added"` // true = reaction added, false = removed
}

// WSTypingPayload represents the payload for typing indicators
type WSTypingPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	IsTyping       bool   `json:"is_typing"`
}

// WSReadPayload represents the payload for read receipts
type WSReadPayload struct {
	ConversationID string    `json:"conversation_id"`
	MessageID      string    `json:"message_id"`
	ReadAt         time.Time `json:"read_at"`
}

// WSErrorPayload represents an error message over WebSocket
type WSErrorPayload struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}
