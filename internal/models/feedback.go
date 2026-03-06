package models

import "time"

// FeedbackType represents the type of feedback
type FeedbackType string

const (
	FeedbackTypeGeneral     FeedbackType = "GENERAL"
	FeedbackTypeBug         FeedbackType = "BUG"
	FeedbackTypeFeature     FeedbackType = "FEATURE"
	FeedbackTypeImprovement FeedbackType = "IMPROVEMENT"
)

// FeedbackRating represents user satisfaction rating
type FeedbackRating int

const (
	FeedbackRatingVeryBad  FeedbackRating = 1
	FeedbackRatingBad      FeedbackRating = 2
	FeedbackRatingNeutral  FeedbackRating = 3
	FeedbackRatingGood     FeedbackRating = 4
	FeedbackRatingExcellent FeedbackRating = 5
)

// Feedback represents user feedback
type Feedback struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Rating      FeedbackRating `json:"rating"`
	Type        FeedbackType   `json:"type"`
	Message     string         `json:"message"`
	AppVersion  *string        `json:"app_version,omitempty"`
	DeviceInfo  *string        `json:"device_info,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// CreateFeedbackRequest is the request to create feedback
type CreateFeedbackRequest struct {
	Rating     FeedbackRating `json:"rating" validate:"required,min=1,max=5"`
	Type       FeedbackType   `json:"type" validate:"required,oneof=GENERAL BUG FEATURE IMPROVEMENT"`
	Message    string         `json:"message" validate:"required,min=1,max=2000"`
	AppVersion *string        `json:"app_version,omitempty"`
	DeviceInfo *string        `json:"device_info,omitempty"`
}

// FeedbackResponse is the response for feedback submission
type FeedbackResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// FeedbackStatusResponse returns whether user has submitted feedback
type FeedbackStatusResponse struct {
	HasSubmitted bool       `json:"has_submitted"`
	LastFeedback *time.Time `json:"last_feedback,omitempty"`
}
