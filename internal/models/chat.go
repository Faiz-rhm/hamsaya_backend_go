package models

import "time"

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeText  MessageType = "TEXT"
	MessageTypeImage MessageType = "IMAGE"
	MessageTypeFile  MessageType = "FILE"
)

// Conversation represents a chat conversation between two users
type Conversation struct {
	ID             string     `json:"id"`
	Participant1ID string     `json:"participant1_id"`
	Participant2ID string     `json:"participant2_id"`
	LastMessageAt  *time.Time `json:"last_message_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Message represents a chat message
type Message struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversation_id"`
	SenderID       string       `json:"sender_id"`
	Content        *string      `json:"content"`
	MessageType    MessageType  `json:"message_type"`
	ReadAt         *time.Time   `json:"read_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	DeletedAt      *time.Time   `json:"deleted_at,omitempty"`
}

// ConversationResponse is the API response for a conversation
type ConversationResponse struct {
	ID               string         `json:"id"`
	OtherParticipant *UserInfo      `json:"other_participant"`
	LastMessage      *MessageInfo   `json:"last_message,omitempty"`
	UnreadCount      int            `json:"unread_count"`
	LastMessageAt    *time.Time     `json:"last_message_at"`
	CreatedAt        time.Time      `json:"created_at"`
}

// MessageResponse is the API response for a message
type MessageResponse struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversation_id"`
	Sender         *UserInfo    `json:"sender"`
	Content        *string      `json:"content"`
	MessageType    MessageType  `json:"message_type"`
	IsRead         bool         `json:"is_read"`
	CreatedAt      time.Time    `json:"created_at"`
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
	UserID    string  `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	FullName  string  `json:"full_name"`
	Avatar    *Photo  `json:"avatar,omitempty"`
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	RecipientID string      `json:"recipient_id" validate:"required,uuid"`
	Content     *string     `json:"content,omitempty" validate:"omitempty,min=1,max=5000"`
	MessageType MessageType `json:"message_type" validate:"required,oneof=TEXT IMAGE FILE"`
}

// GetConversationsFilter represents filters for listing conversations
type GetConversationsFilter struct {
	UserID string
	Limit  int
	Offset int
}

// GetMessagesFilter represents filters for listing messages
type GetMessagesFilter struct {
	ConversationID string
	Limit          int
	Offset         int
}

// WSMessage represents a WebSocket message for real-time communication
type WSMessage struct {
	Type    string      `json:"type"` // "message", "typing", "read", "error"
	Payload interface{} `json:"payload"`
}

// WSMessagePayload represents the payload for a new message over WebSocket
type WSMessagePayload struct {
	ConversationID string       `json:"conversation_id"`
	MessageID      string       `json:"message_id"`
	SenderID       string       `json:"sender_id"`
	Content        *string      `json:"content"`
	MessageType    MessageType  `json:"message_type"`
	CreatedAt      time.Time    `json:"created_at"`
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
