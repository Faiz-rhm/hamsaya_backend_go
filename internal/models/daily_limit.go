package models

import "time"

// DailyPostLimit holds the per-post-type daily creation limit. Rows are
// admin-editable via /api/v1/admin/daily-limits. Counters live in Redis
// (see services.DailyLimitService); this struct only describes the limit.
type DailyPostLimit struct {
	PostType           string    `json:"post_type"`
	UserLimit          int       `json:"user_limit"`
	BusinessMultiplier float64   `json:"business_multiplier"`
	Unlimited          bool      `json:"unlimited"`
	Description        *string   `json:"description,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
	UpdatedBy          *string   `json:"updated_by,omitempty"`
}

// UpdateDailyPostLimitRequest is the admin payload to tune a limit row.
// Both fields are optional so the admin can change just one without a
// full overwrite. Validation happens in the service.
type UpdateDailyPostLimitRequest struct {
	UserLimit          *int     `json:"user_limit,omitempty" validate:"omitempty,min=0,max=10000"`
	BusinessMultiplier *float64 `json:"business_multiplier,omitempty" validate:"omitempty,min=0,max=100"`
	Unlimited          *bool    `json:"unlimited,omitempty"`
	Description        *string  `json:"description,omitempty" validate:"omitempty,max=500"`
}

// DailyLimitUsage is the per-post-type status returned to the user-facing
// endpoint. Used by the mobile app to show remaining counts and reset time.
type DailyLimitUsage struct {
	PostType  string    `json:"post_type"`
	Used      int       `json:"used"`
	Limit     int       `json:"limit"`     // Effective limit (after business multiplier / admin bypass)
	Remaining int       `json:"remaining"` // max(0, limit - used). -1 = unlimited (admin)
	Unlimited bool      `json:"unlimited"`
	ResetsAt  time.Time `json:"resets_at"` // Next UTC midnight
}
