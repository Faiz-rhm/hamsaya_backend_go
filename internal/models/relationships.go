package models

import "time"

// UserFollow represents a follow relationship between users
type UserFollow struct {
	ID          string    `json:"id"`
	FollowerID  string    `json:"follower_id"`
	FollowingID string    `json:"following_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserBlock represents a block relationship between users
type UserBlock struct {
	ID        string    `json:"id"`
	BlockerID string    `json:"blocker_id"`
	BlockedID string    `json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}

// FollowRequest represents a follow request
type FollowRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

// FollowerResponse represents a follower in the response
type FollowerResponse struct {
	UserID    string     `json:"user_id"`
	FirstName *string    `json:"first_name,omitempty"`
	LastName  *string    `json:"last_name,omitempty"`
	FullName  string     `json:"full_name"`
	Avatar    *Photo     `json:"avatar,omitempty"`
	Province  *string    `json:"province,omitempty"`
	CreatedAt time.Time  `json:"followed_at"`

	// Relationship status (relative to authenticated user)
	IsFollowing  bool `json:"is_following"`
	IsFollowedBy bool `json:"is_followed_by"`
}

// FollowingResponse represents a following user in the response
type FollowingResponse struct {
	UserID    string     `json:"user_id"`
	FirstName *string    `json:"first_name,omitempty"`
	LastName  *string    `json:"last_name,omitempty"`
	FullName  string     `json:"full_name"`
	Avatar    *Photo     `json:"avatar,omitempty"`
	Province  *string    `json:"province,omitempty"`
	CreatedAt time.Time  `json:"following_since"`

	// Relationship status (relative to authenticated user)
	IsFollowing  bool `json:"is_following"`
	IsFollowedBy bool `json:"is_followed_by"`
}

// BlockedUserResponse represents a blocked user in the response
type BlockedUserResponse struct {
	UserID    string     `json:"user_id"`
	FirstName *string    `json:"first_name,omitempty"`
	LastName  *string    `json:"last_name,omitempty"`
	FullName  string     `json:"full_name"`
	Avatar    *Photo     `json:"avatar,omitempty"`
	Province  *string    `json:"province,omitempty"`
	CreatedAt time.Time  `json:"blocked_at"`
}

// RelationshipStatus represents the relationship status between two users
type RelationshipStatus struct {
	IsFollowing  bool `json:"is_following"`
	IsFollowedBy bool `json:"is_followed_by"`
	IsBlocked    bool `json:"is_blocked"`
	HasBlockedMe bool `json:"has_blocked_me"`
}
