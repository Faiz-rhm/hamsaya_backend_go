package models

import "time"

// ReportStatus represents the status of a report
type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "PENDING"
	ReportStatusReviewing ReportStatus = "REVIEWING"
	ReportStatusResolved  ReportStatus = "RESOLVED"
	ReportStatusRejected  ReportStatus = "REJECTED"
)

// PostReport represents a report for a post
type PostReport struct {
	ID                 string       `json:"id"`
	UserID             string       `json:"user_id"`
	PostID             string       `json:"post_id"`
	Reason             string       `json:"reason"`
	AdditionalComments *string      `json:"additional_comments,omitempty"`
	ReportStatus       ReportStatus `json:"report_status"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

// CommentReport represents a report for a comment
type CommentReport struct {
	ID                 string       `json:"id"`
	UserID             string       `json:"user_id"`
	CommentID          string       `json:"comment_id"`
	Reason             string       `json:"reason"`
	AdditionalComments *string      `json:"additional_comments,omitempty"`
	ReportStatus       ReportStatus `json:"report_status"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

// UserReport represents a report for a user
type UserReport struct {
	ID             string    `json:"id"`
	ReportedUser   string    `json:"reported_user"`
	ReportedByID   string    `json:"reported_by_id"`
	Reason         string    `json:"reason"`
	Description    *string   `json:"description,omitempty"`
	Resolved       bool      `json:"resolved"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BusinessReport represents a report for a business
type BusinessReport struct {
	ID                 string       `json:"id"`
	BusinessID         string       `json:"business_id"`
	UserID             string       `json:"user_id"`
	Reason             string       `json:"reason"`
	AdditionalComments *string      `json:"additional_comments,omitempty"`
	ReportStatus       ReportStatus `json:"report_status"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

// CreatePostReportRequest represents a request to report a post
type CreatePostReportRequest struct {
	Reason             string  `json:"reason" validate:"required,max=100"`
	AdditionalComments *string `json:"additional_comments,omitempty" validate:"omitempty,max=500"`
}

// CreateCommentReportRequest represents a request to report a comment
type CreateCommentReportRequest struct {
	Reason             string  `json:"reason" validate:"required,max=100"`
	AdditionalComments *string `json:"additional_comments,omitempty" validate:"omitempty,max=500"`
}

// CreateUserReportRequest represents a request to report a user
type CreateUserReportRequest struct {
	Reason      string  `json:"reason" validate:"required,max=100"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// CreateBusinessReportRequest represents a request to report a business
type CreateBusinessReportRequest struct {
	Reason             string  `json:"reason" validate:"required,max=100"`
	AdditionalComments *string `json:"additional_comments,omitempty" validate:"omitempty,max=500"`
}

// UpdateReportStatusRequest represents a request to update report status
type UpdateReportStatusRequest struct {
	Status ReportStatus `json:"status" validate:"required,oneof=PENDING REVIEWING RESOLVED REJECTED"`
}

// ReportListResponse represents a paginated list of reports
type ReportListResponse struct {
	Reports    interface{} `json:"reports"`
	TotalCount int         `json:"total_count"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
}
