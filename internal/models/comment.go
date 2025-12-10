package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// PostComment represents a comment on a post
type PostComment struct {
	ID              string        `json:"id"`
	PostID          string        `json:"post_id"`
	UserID          string        `json:"user_id"`
	ParentCommentID *string       `json:"parent_comment_id,omitempty"`
	Text            string        `json:"text"`
	Location        *pgtype.Point `json:"location,omitempty"`
	TotalLikes      int           `json:"total_likes"`
	TotalReplies    int           `json:"total_replies"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `json:"-"`
}

// CommentAttachment represents an attachment on a comment
type CommentAttachment struct {
	ID        string     `json:"id"`
	CommentID string     `json:"comment_id"`
	Photo     Photo      `json:"photo"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// CommentLike represents a like on a comment
type CommentLike struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CommentID string    `json:"comment_id"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	Text            string   `json:"text" validate:"required,min=1,max=1000"`
	ParentCommentID *string  `json:"parent_comment_id,omitempty" validate:"omitempty,uuid"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
	Attachments     []string `json:"attachments,omitempty"` // Photo URLs
}

// UpdateCommentRequest represents a request to update a comment
type UpdateCommentRequest struct {
	Text string `json:"text" validate:"required,min=1,max=1000"`
}

// CommentResponse represents a comment in API responses
type CommentResponse struct {
	ID              string            `json:"id"`
	PostID          string            `json:"post_id"`
	Text            string            `json:"text"`
	Author          *AuthorInfo       `json:"author,omitempty"`
	ParentCommentID *string           `json:"parent_comment_id,omitempty"`
	Attachments     []Photo           `json:"attachments,omitempty"`
	Location        *LocationInfo     `json:"location,omitempty"`
	TotalLikes      int               `json:"total_likes"`
	TotalReplies    int               `json:"total_replies"`
	LikedByMe       bool              `json:"liked_by_me"`
	IsMine          bool              `json:"is_mine"`
	Replies         []*CommentResponse `json:"replies,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// CommentFilter represents filters for fetching comments
type CommentFilter struct {
	PostID          string  `json:"post_id"`
	ParentCommentID *string `json:"parent_comment_id,omitempty"`
	Limit           int     `json:"limit"`
	Offset          int     `json:"offset"`
}
