package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// BusinessProfile represents a business profile
type BusinessProfile struct {
	ID              string        `json:"id"`
	UserID          string        `json:"user_id"`
	Name            string        `json:"name"`
	LicenseNo       *string       `json:"license_no,omitempty"`
	Description     *string       `json:"description,omitempty"`
	Address         *string       `json:"address,omitempty"`
	PhoneNumber     *string       `json:"phone_number,omitempty"`
	Email           *string       `json:"email,omitempty"`
	Website         *string       `json:"website,omitempty"`
	Avatar          *Photo        `json:"avatar,omitempty"`
	AvatarColor     *string       `json:"avatar_color,omitempty"`
	Cover           *Photo        `json:"cover,omitempty"`
	Status          bool          `json:"status"`
	AdditionalInfo  *string       `json:"additional_info,omitempty"`
	AddressLocation *pgtype.Point `json:"-"`
	Country         *string       `json:"country,omitempty"`
	Province        *string       `json:"province,omitempty"`
	District        *string       `json:"district,omitempty"`
	Neighborhood    *string       `json:"neighborhood,omitempty"`
	ShowLocation    bool          `json:"show_location"`
	TotalViews      int           `json:"total_views"`
	TotalFollow     int           `json:"total_follow"`
	AvgRating       float64       `json:"avg_rating"`
	ReviewCount     int           `json:"review_count"`
	IsVerified      bool          `json:"is_verified"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `json:"-"`
}

// BusinessCategory represents a business category
type BusinessCategory struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// BusinessProfileCategory represents the many-to-many relationship
type BusinessProfileCategory struct {
	ID                 string    `json:"id"`
	BusinessProfileID  string    `json:"business_profile_id"`
	BusinessCategoryID string    `json:"business_category_id"`
	CreatedAt          time.Time `json:"created_at"`
}

// BusinessHours represents business operating hours
type BusinessHours struct {
	ID                string     `json:"id"`
	BusinessProfileID string     `json:"business_profile_id"`
	Day               string     `json:"day"`
	OpenTime          *time.Time `json:"open_time,omitempty"`
	CloseTime         *time.Time `json:"close_time,omitempty"`
	IsClosed          bool       `json:"is_closed"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// GalleryItem is a single gallery image with id (for client delete)
type GalleryItem struct {
	ID    string `json:"id"`
	Photo Photo  `json:"photo"`
}

// BusinessAttachment represents a business gallery image
type BusinessAttachment struct {
	ID                string     `json:"id"`
	BusinessProfileID string     `json:"business_profile_id"`
	Photo             Photo      `json:"photo"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"-"`
}

// BusinessFollower represents a user following a business
type BusinessFollower struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"business_id"`
	FollowerID string    `json:"follower_id"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateBusinessRequest represents a request to create a business profile
// AdminCreateBusinessRequest lets an admin create a business on behalf of
// another user. OwnerID is the user the business is assigned to; the rest of
// the fields are identical to a normal create.
type AdminCreateBusinessRequest struct {
	OwnerID string `json:"owner_id" validate:"required,uuid"`
	CreateBusinessRequest
}

type CreateBusinessRequest struct {
	Name           string   `json:"name" validate:"required,min=2,max=255"`
	LicenseNo      *string  `json:"license_no,omitempty" validate:"omitempty,max=100"`
	Description    *string  `json:"description,omitempty" validate:"omitempty,max=5000"`
	Address        *string  `json:"address,omitempty" validate:"omitempty,max=500"`
	PhoneNumber    *string  `json:"phone_number,omitempty" validate:"omitempty,max=20"`
	Email          *string  `json:"email,omitempty" validate:"omitempty,email"`
	Website        *string  `json:"website,omitempty" validate:"omitempty,url"`
	AdditionalInfo *string  `json:"additional_info,omitempty"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	Country        *string  `json:"country,omitempty" validate:"omitempty,max=100"`
	Province       *string  `json:"province,omitempty" validate:"omitempty,max=100"`
	District       *string  `json:"district,omitempty" validate:"omitempty,max=100"`
	Neighborhood   *string  `json:"neighborhood,omitempty" validate:"omitempty,max=100"`
	ShowLocation   *bool    `json:"show_location,omitempty"`
	AvatarColor    *string  `json:"avatar_color,omitempty" validate:"omitempty,len=7"`
	CategoryIDs    []string `json:"category_ids,omitempty" validate:"omitempty,dive,uuid"`
	// CategoryNames are created if they don't exist, then linked (with category_ids).
	CategoryNames []string `json:"category_names,omitempty" validate:"omitempty,dive,max=100"`
}

// UpdateBusinessRequest represents a request to update a business profile
type UpdateBusinessRequest struct {
	Name           *string  `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	LicenseNo      *string  `json:"license_no,omitempty" validate:"omitempty,max=100"`
	Description    *string  `json:"description,omitempty" validate:"omitempty,max=5000"`
	Address        *string  `json:"address,omitempty" validate:"omitempty,max=500"`
	PhoneNumber    *string  `json:"phone_number,omitempty" validate:"omitempty,max=20"`
	Email          *string  `json:"email,omitempty" validate:"omitempty,email"`
	Website        *string  `json:"website,omitempty" validate:"omitempty,url"`
	AdditionalInfo *string  `json:"additional_info,omitempty"`
	Status         *bool    `json:"status,omitempty"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	Country        *string  `json:"country,omitempty" validate:"omitempty,max=100"`
	Province       *string  `json:"province,omitempty" validate:"omitempty,max=100"`
	District       *string  `json:"district,omitempty" validate:"omitempty,max=100"`
	Neighborhood   *string  `json:"neighborhood,omitempty" validate:"omitempty,max=100"`
	ShowLocation   *bool    `json:"show_location,omitempty"`
	AvatarColor    *string  `json:"avatar_color,omitempty" validate:"omitempty,len=7"`
	CategoryIDs    []string `json:"category_ids,omitempty" validate:"omitempty,dive,uuid"`
	// CategoryNames are created if they don't exist, then linked (with category_ids).
	CategoryNames []string `json:"category_names,omitempty" validate:"omitempty,dive,max=100"`
}

// BusinessHoursRequest represents operating hours for a day
type BusinessHoursRequest struct {
	Day       string `json:"day" validate:"required,oneof=Monday Tuesday Wednesday Thursday Friday Saturday Sunday"`
	OpenTime  string `json:"open_time,omitempty" validate:"omitempty"`
	CloseTime string `json:"close_time,omitempty" validate:"omitempty"`
	IsClosed  bool   `json:"is_closed"`
}

// SetBusinessHoursRequest represents a request to set business hours
type SetBusinessHoursRequest struct {
	Hours []BusinessHoursRequest `json:"hours" validate:"required,min=1,max=7"`
}

// BusinessResponse represents a business profile in API responses
type BusinessResponse struct {
	ID              string                  `json:"id"`
	UserID          string                  `json:"user_id"`
	Name            string                  `json:"name"`
	LicenseNo       *string                 `json:"license_no,omitempty"`
	Description     *string                 `json:"description,omitempty"`
	Address         *string                 `json:"address,omitempty"`
	PhoneNumber     *string                 `json:"phone_number,omitempty"`
	Email           *string                 `json:"email,omitempty"`
	Website         *string                 `json:"website,omitempty"`
	Avatar          *Photo                  `json:"avatar,omitempty"`
	AvatarColor     *string                 `json:"avatar_color,omitempty"`
	Cover           *Photo                  `json:"cover,omitempty"`
	Status          bool                    `json:"status"`
	AdditionalInfo  *string                 `json:"additional_info,omitempty"`
	Location        *LocationInfo           `json:"location"`         // always present (null if no coordinates)
	AddressLocation *string                 `json:"address_location"` // "(lat,lng)" for mobile; null if not set
	Country         *string                 `json:"country"`
	Province        *string                 `json:"province"`
	District        *string                 `json:"district"`
	Neighborhood    *string                 `json:"neighborhood"`
	ShowLocation    bool                    `json:"show_location"`
	TotalViews      int                     `json:"total_views"`
	TotalFollow     int                     `json:"total_follow"`
	Categories      []BusinessCategory      `json:"categories"`
	Hours           []BusinessHoursResponse `json:"hours,omitempty"`
	Gallery         []GalleryItem           `json:"gallery,omitempty"`
	IsFollowing     bool                    `json:"is_following"`
	IsVerified      bool                    `json:"is_verified"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// BusinessHoursResponse represents business hours in API responses
type BusinessHoursResponse struct {
	Day       string  `json:"day"`
	OpenTime  *string `json:"open_time,omitempty"`
	CloseTime *string `json:"close_time,omitempty"`
	IsClosed  bool    `json:"is_closed"`
}

// BusinessListFilter represents filters for listing businesses
type BusinessListFilter struct {
	UserID     *string  `json:"user_id,omitempty"`
	CategoryID *string  `json:"category_id,omitempty"`
	Province   *string  `json:"province,omitempty"`
	Search     *string  `json:"search,omitempty"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	RadiusKm   *float64 `json:"radius_km,omitempty"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
}

// DailyCount is one point in an insights time-series.
type DailyCount struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// BusinessInsightsResponse is the owner-only analytics payload: per-day
// series (zero-filled, oldest first) plus all-time totals for the header
// numbers on the insight cards.
type BusinessInsightsResponse struct {
	Days       int          `json:"days"`
	Views      []DailyCount `json:"views"`
	Followers  []DailyCount `json:"followers"`
	Reviews    []DailyCount `json:"reviews"`
	Likes      []DailyCount `json:"likes"`       // likes on the business's posts
	Comments   []DailyCount `json:"comments"`    // comments on the business's posts
	PostViews  []DailyCount `json:"post_views"`  // unique post views ("reach")
	Sold       []DailyCount `json:"sold"`        // owner's SELL listings marked sold
	EventRSVPs []DailyCount `json:"event_rsvps"` // "going" RSVPs on the business's events
	// Visible-review counts keyed by star ("1".."5"), zero-filled.
	RatingDistribution map[string]int `json:"rating_distribution"`
	AvgRating          float64        `json:"avg_rating"`
	TotalViews         int            `json:"total_views"`
	TotalFollowers     int            `json:"total_followers"`
	TotalReviews       int            `json:"total_reviews"`
	// Distinct users going to any of the business's events (all-time).
	TotalEventAttendees int `json:"total_event_attendees"`
	// Content counts for the dashboard (business posts + owner's listings).
	PostCounts *BusinessOwnerPostCounts `json:"post_counts,omitempty"`
}

// BusinessOwnerPostCounts summarizes the owner's content for the dashboard:
// the business's updates/events/polls plus the owner's marketplace listings
// (SELL posts are user-authored under the business-updates rule).
type BusinessOwnerPostCounts struct {
	UpcomingEvents int `json:"upcoming_events"`
	Updates        int `json:"updates"`
	Polls          int `json:"polls"`
	ActiveSells    int `json:"active_sells"`
	SoldSells      int `json:"sold_sells"`
}

// Business verification -------------------------------------------------------

// VerificationStatus values for business_verification_requests.status.
const (
	VerificationStatusPending  = "PENDING"
	VerificationStatusApproved = "APPROVED"
	VerificationStatusRejected = "REJECTED"
)

// BusinessVerificationRequest is one owner-submitted verification attempt.
type BusinessVerificationRequest struct {
	ID              string     `json:"id"`
	BusinessID      string     `json:"business_id"`
	UserID          string     `json:"user_id"`
	LicenseNo       *string    `json:"license_no,omitempty"`
	Note            *string    `json:"note,omitempty"`
	Documents       []Photo    `json:"documents"`
	Status          string     `json:"status"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	ReviewedBy      *string    `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// BusinessVerificationListItem is the admin queue row: request + business
// context for review without extra fetches.
type BusinessVerificationListItem struct {
	BusinessVerificationRequest
	BusinessName   string  `json:"business_name"`
	BusinessAvatar *Photo  `json:"business_avatar,omitempty"`
	OwnerEmail     *string `json:"owner_email,omitempty"`
}

// ReviewBusinessVerificationRequest is the admin approve/reject payload.
type ReviewBusinessVerificationRequest struct {
	Action string  `json:"action" validate:"required,oneof=approve reject"`
	Reason *string `json:"reason,omitempty" validate:"omitempty,max=1000"`
}
