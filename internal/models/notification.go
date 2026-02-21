package models

import "time"

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeLike            NotificationType = "LIKE"
	NotificationTypeComment         NotificationType = "COMMENT"
	NotificationTypeFollow          NotificationType = "FOLLOW"
	NotificationTypeMessage         NotificationType = "MESSAGE"
	NotificationTypeMention         NotificationType = "MENTION"
	NotificationTypeEventInterest   NotificationType = "EVENT_INTEREST"
	NotificationTypeBusinessFollow  NotificationType = "BUSINESS_FOLLOW"
	NotificationTypePostShare       NotificationType = "POST_SHARE"
	NotificationTypePollVote        NotificationType = "POLL_VOTE"
	NotificationTypeNewPost         NotificationType = "NEW_POST"
)

// NotificationCategory represents notification category for settings
type NotificationCategory string

const (
	NotificationCategoryPosts    NotificationCategory = "POSTS"
	NotificationCategoryMessages NotificationCategory = "MESSAGES"
	NotificationCategoryEvents   NotificationCategory = "EVENTS"
	NotificationCategorySales    NotificationCategory = "SALES"
	NotificationCategoryBusiness NotificationCategory = "BUSINESS"
)

// Notification represents a user notification
type Notification struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Type      NotificationType       `json:"type"`
	Title     *string                `json:"title,omitempty"`
	Message   *string                `json:"message,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
}

// NotificationSetting represents user notification preferences
type NotificationSetting struct {
	ID         string               `json:"id"`
	ProfileID  string               `json:"profile_id"`
	Category   NotificationCategory `json:"category"`
	PushPref   bool                 `json:"push_pref"`
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
}

// NotificationResponse is the API response for a notification
type NotificationResponse struct {
	ID        string                 `json:"id"`
	Type      NotificationType       `json:"type"`
	Title     *string                `json:"title,omitempty"`
	Message   *string                `json:"message,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
}

// CreateNotificationRequest represents a request to create a notification
type CreateNotificationRequest struct {
	UserID  string                 `json:"user_id" validate:"required,uuid"`
	Type    NotificationType       `json:"type" validate:"required"`
	Title   *string                `json:"title,omitempty" validate:"omitempty,max=255"`
	Message *string                `json:"message,omitempty" validate:"omitempty,max=1000"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// UpdateNotificationSettingsRequest represents a request to update notification settings
type UpdateNotificationSettingsRequest struct {
	Category NotificationCategory `json:"category" validate:"required,oneof=POSTS MESSAGES EVENTS SALES BUSINESS"`
	PushPref bool                 `json:"push_pref"`
}

// GetNotificationsFilter represents filters for listing notifications
type GetNotificationsFilter struct {
	UserID      string
	Type        *NotificationType
	UnreadOnly  bool
	BusinessID  *string // when set, only notifications whose data.business_id matches (e.g. BUSINESS_FOLLOW)
	Limit       int
	Offset      int
}

// FCMTokenRequest represents a request to register/update FCM token
type FCMTokenRequest struct {
	Token      string  `json:"token" validate:"required,min=10"`
	DeviceName *string `json:"device_name,omitempty" validate:"omitempty,max=100"`
}

// PushNotificationPayload represents the payload for push notifications
type PushNotificationPayload struct {
	Title        string                 `json:"title"`
	Body         string                 `json:"body"`
	Data         map[string]interface{} `json:"data,omitempty"`
	ClickAction  string                 `json:"click_action,omitempty"`
	Sound        string                 `json:"sound,omitempty"`
	Badge        int                    `json:"badge,omitempty"`
	ImageURL     string                 `json:"image_url,omitempty"`
}

// ToNotificationResponse converts a Notification to NotificationResponse
func (n *Notification) ToNotificationResponse() *NotificationResponse {
	return &NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Message:   n.Message,
		Data:      n.Data,
		Read:      n.Read,
		CreatedAt: n.CreatedAt,
	}
}
