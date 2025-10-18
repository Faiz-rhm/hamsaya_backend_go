package models

import "time"

// Poll represents a poll attached to a PULL post
type Poll struct {
	ID        string     `json:"id"`
	PostID    string     `json:"post_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// PollOption represents an option in a poll
type PollOption struct {
	ID        string     `json:"id"`
	PollID    string     `json:"poll_id"`
	Option    string     `json:"option"`
	VoteCount int        `json:"vote_count"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// UserPoll represents a user's vote on a poll
type UserPoll struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	PollID       string     `json:"poll_id"`
	PollOptionID string     `json:"poll_option_id"`
	CreatedAt    time.Time  `json:"created_at"`
	DeletedAt    *time.Time `json:"-"`
}

// CreatePollRequest represents a request to create a poll
type CreatePollRequest struct {
	Options []string `json:"options" validate:"required,min=2,max=10,dive,required,min=1,max=100"`
}

// VotePollRequest represents a request to vote on a poll
type VotePollRequest struct {
	PollOptionID string `json:"poll_option_id" validate:"required,uuid"`
}

// PollResponse represents a poll in API responses
type PollResponse struct {
	ID           string               `json:"id"`
	PostID       string               `json:"post_id"`
	Options      []*PollOptionResponse `json:"options"`
	TotalVotes   int                  `json:"total_votes"`
	UserVote     *string              `json:"user_vote,omitempty"` // Poll option ID that user voted for
	HasVoted     bool                 `json:"has_voted"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// PollOptionResponse represents a poll option in API responses
type PollOptionResponse struct {
	ID         string  `json:"id"`
	Option     string  `json:"option"`
	VoteCount  int     `json:"vote_count"`
	Percentage float64 `json:"percentage"`
}
