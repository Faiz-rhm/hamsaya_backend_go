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
	ID         string     `json:"id"`
	BusinessID string     `json:"business_id"`
	FollowerID string     `json:"follower_id"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CreateBusinessRequest represents a request to create a business profile
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
	CategoryIDs    []string `json:"category_ids,omitempty" validate:"omitempty,dive,uuid"`
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
	ID             string                    `json:"id"`
	UserID         string                    `json:"user_id"`
	Name           string                    `json:"name"`
	LicenseNo      *string                   `json:"license_no,omitempty"`
	Description    *string                   `json:"description,omitempty"`
	Address        *string                   `json:"address,omitempty"`
	PhoneNumber    *string                   `json:"phone_number,omitempty"`
	Email          *string                   `json:"email,omitempty"`
	Website        *string                   `json:"website,omitempty"`
	Avatar         *Photo                    `json:"avatar,omitempty"`
	Cover          *Photo                    `json:"cover,omitempty"`
	Status         bool                      `json:"status"`
	AdditionalInfo *string                   `json:"additional_info,omitempty"`
	Location       *LocationInfo             `json:"location,omitempty"`
	ShowLocation   bool                      `json:"show_location"`
	TotalViews     int                       `json:"total_views"`
	TotalFollow    int                       `json:"total_follow"`
	Categories     []BusinessCategory        `json:"categories"`
	Hours          []BusinessHoursResponse   `json:"hours,omitempty"`
	Gallery        []Photo                   `json:"gallery,omitempty"`
	IsFollowing    bool                      `json:"is_following"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
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
