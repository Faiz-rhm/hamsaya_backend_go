package models

import "time"

// HelpChatMessage is a single message in the help center thread (user or support).
// Content may include [Image: url] placeholders for attachments.
type HelpChatMessage struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Content     string    `json:"content"`
	IsFromUser  bool      `json:"is_from_user"`
	AppVersion  *string   `json:"app_version,omitempty"`
	DeviceInfo  *string   `json:"device_info,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateHelpChatMessageRequest is the request to send a help message.
// Message can contain [Image: url] placeholders (client uploads images first, then sends URLs in content).
type CreateHelpChatMessageRequest struct {
	Message    string  `json:"message" validate:"required,max=10000"`
	AppVersion *string `json:"app_version,omitempty"`
	DeviceInfo *string `json:"device_info,omitempty"`
}

// HelpChatMessageResponse is a single message in API responses.
type HelpChatMessageResponse struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	IsFromUser bool      `json:"is_from_user"`
	CreatedAt  time.Time `json:"created_at"`
}

// HelpChatSendResponse is returned after sending a message.
type HelpChatSendResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
