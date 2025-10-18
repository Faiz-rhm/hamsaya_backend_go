package models

import "time"

// EventInterestState represents a user's interest level in an event
type EventInterestState string

const (
	EventInterestInterested    EventInterestState = "interested"
	EventInterestGoing         EventInterestState = "going"
	EventInterestNotInterested EventInterestState = "not_interested"
)

// EventInterest represents a user's interest in an event post
type EventInterest struct {
	ID         string             `json:"id"`
	PostID     string             `json:"post_id"`
	UserID     string             `json:"user_id"`
	EventState EventInterestState `json:"event_state"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

// EventInterestRequest represents a request to set event interest
type EventInterestRequest struct {
	EventState EventInterestState `json:"event_state" validate:"required,oneof=interested going not_interested"`
}

// EventInterestResponse represents event interest in API responses
type EventInterestResponse struct {
	PostID          string             `json:"post_id"`
	UserEventState  EventInterestState `json:"user_event_state"`
	InterestedCount int                `json:"interested_count"`
	GoingCount      int                `json:"going_count"`
}

// EventInterestedUser represents a user interested in an event
type EventInterestedUser struct {
	User      *AuthorInfo        `json:"user"`
	EventState EventInterestState `json:"event_state"`
	CreatedAt time.Time          `json:"created_at"`
}
