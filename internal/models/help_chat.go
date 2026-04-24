package models

import "time"

// HelpChatMessage is one message in a user's support thread.
type HelpChatMessage struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Content    string    `json:"content"`
	IsFromUser bool      `json:"is_from_user"`
	AppVersion *string   `json:"app_version,omitempty"`
	DeviceInfo *string   `json:"device_info,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// HelpChatThread is a summary row shown in the admin inbox.
type HelpChatThread struct {
	UserID         string    `json:"user_id"`
	FullName       string    `json:"full_name"`
	Email          string    `json:"email"`
	LastMessage    string    `json:"last_message"`
	LastIsFromUser bool      `json:"last_is_from_user"`
	LastAt         time.Time `json:"last_at"`
}

// SendHelpMessageRequest is the user-facing request body.
type SendHelpMessageRequest struct {
	Content    string  `json:"content" validate:"required,min=1,max=2000"`
	AppVersion *string `json:"app_version,omitempty"`
	DeviceInfo *string `json:"device_info,omitempty"`
}

// AdminReplyRequest is the admin-facing request body.
type AdminReplyRequest struct {
	Content string `json:"content" validate:"required,min=1,max=2000"`
}
