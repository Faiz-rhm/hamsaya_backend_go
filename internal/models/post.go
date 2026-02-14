package models

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// PostType represents the type of post
type PostType string

const (
	PostTypeFeed PostType = "FEED"
	PostTypeEvent PostType = "EVENT"
	PostTypeSell PostType = "SELL"
	PostTypePull PostType = "PULL"
)

// PostVisibility represents the visibility of a post
type PostVisibility string

const (
	VisibilityPublic  PostVisibility = "PUBLIC"
	VisibilityFriends PostVisibility = "FRIENDS"
	VisibilityPrivate PostVisibility = "PRIVATE"
)

// EventState represents the state of an event
type EventState string

const (
	EventStateUpcoming EventState = "upcoming"
	EventStateOngoing  EventState = "ongoing"
	EventStateEnded    EventState = "ended"
)

// Post represents a post in the system
type Post struct {
	ID               string          `json:"id"`
	UserID           *string         `json:"user_id,omitempty"`
	BusinessID       *string         `json:"business_id,omitempty"`
	OriginalPostID   *string         `json:"original_post_id,omitempty"`
	CategoryID       *string         `json:"category_id,omitempty"`

	// Content fields
	Title            *string         `json:"title,omitempty"`
	Description      *string         `json:"description,omitempty"`
	Type             PostType        `json:"type"`
	Status           bool            `json:"status"`
	Visibility       PostVisibility  `json:"visibility"`

	// Sell-specific fields
	Currency         *string         `json:"currency,omitempty"`
	Price            *float64        `json:"price,omitempty"`
	Discount         *float64        `json:"discount,omitempty"`
	Free             bool            `json:"free"`
	Sold             bool            `json:"sold"`
	IsPromoted       bool            `json:"is_promoted"`
	CountryCode      *string         `json:"country_code,omitempty"`
	ContactNo        *string         `json:"contact_no,omitempty"`
	IsLocation       bool            `json:"is_location"`

	// Event-specific fields
	StartDate        *time.Time      `json:"start_date,omitempty"`
	StartTime        *time.Time      `json:"start_time,omitempty"`
	EndDate          *time.Time      `json:"end_date,omitempty"`
	EndTime          *time.Time      `json:"end_time,omitempty"`
	EventState       *EventState     `json:"event_state,omitempty"`
	InterestedCount  int             `json:"interested_count"`
	GoingCount       int             `json:"going_count"`
	ExpiredAt        *time.Time      `json:"expired_at,omitempty"`

	// Location fields
	AddressLocation  *pgtype.Point   `json:"address_location,omitempty"`
	UserLocation     *pgtype.Point   `json:"user_location,omitempty"`
	Country          *string         `json:"country,omitempty"`
	Province         *string         `json:"province,omitempty"`
	District         *string         `json:"district,omitempty"`
	Neighborhood     *string         `json:"neighborhood,omitempty"`

	// Engagement counters
	TotalComments    int             `json:"total_comments"`
	TotalLikes       int             `json:"total_likes"`
	TotalShares      int             `json:"total_shares"`

	// Timestamps
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        *time.Time      `json:"-"`
}

// Attachment represents an attachment on a post
type Attachment struct {
	ID        string     `json:"id"`
	PostID    string     `json:"post_id"`
	Photo     Photo      `json:"photo"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// AttachmentResponse is the API‐facing attachment that includes the database ID
// so clients can reference specific attachments (e.g. for deletion on update).
type AttachmentResponse struct {
	ID    string `json:"id"`
	Photo Photo  `json:"photo"`
}

// PollRequestData represents poll data from mobile app
type PollRequestData struct {
	Question string   `json:"question"`
	Options  []string `json:"options" validate:"required,min=2,max=10,dive,required,min=1,max=100"`
}

// CreatePostRequest represents a request to create a post
type CreatePostRequest struct {
	// Content
	Title       *string        `json:"title,omitempty" validate:"omitempty,max=255"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=5000"`
	Type        PostType       `json:"type" validate:"required,oneof=FEED EVENT SELL PULL"`
	Visibility  PostVisibility `json:"visibility,omitempty" validate:"omitempty,oneof=PUBLIC FRIENDS PRIVATE"`

	// Sell-specific
	Currency    *string  `json:"currency,omitempty" validate:"omitempty,len=3"`
	Price       *float64 `json:"price,omitempty" validate:"omitempty,min=0"`
	Discount    *float64 `json:"discount,omitempty" validate:"omitempty,min=0"`
	Free        *bool    `json:"free,omitempty"`
	CategoryID  *string  `json:"category_id,omitempty" validate:"omitempty,uuid"`
	CountryCode *string  `json:"country_code,omitempty"`
	ContactNo   *string  `json:"contact_no,omitempty"`

	// Event-specific
	StartDate *time.Time `json:"start_date,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// Poll-specific (for PULL posts)
	PollOptions []string          `json:"poll_options,omitempty" validate:"omitempty,min=2,max=10,dive,required,min=1,max=100"`
	Poll        *PollRequestData  `json:"poll,omitempty"`

	// Location (accept top-level latitude/longitude or nested location object from app)
	Latitude     *float64             `json:"latitude,omitempty"`
	Longitude    *float64             `json:"longitude,omitempty"`
	Location     *CreatePostLocation  `json:"location,omitempty"`
	IsLocation   *bool    `json:"is_location,omitempty"` // When true, show item on map (SELL)
	Country      *string  `json:"country,omitempty" validate:"omitempty,max=100"`
	Province     *string  `json:"province,omitempty" validate:"omitempty,max=100"`
	District     *string  `json:"district,omitempty" validate:"omitempty,max=100"`
	Neighborhood *string  `json:"neighborhood,omitempty" validate:"omitempty,max=100"`

	// Attachments: already uploaded. Accepts []string (URLs only) or []Photo (full metadata).
	// Use json.RawMessage so we can unmarshal flexibly in the service and avoid binding issues.
	Attachments []json.RawMessage `json:"attachments,omitempty"`

	// For shared posts
	OriginalPostID *string `json:"original_post_id,omitempty" validate:"omitempty,uuid"`
}

// CreatePostLocation is the nested location format sent by the app.
type CreatePostLocation struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// ParseAttachmentPhoto parses a single attachment value (JSON string or object) into a Photo.
// Handles both "url" (string) and {"url":"...","name":"...","size":0,...} from the app.
func ParseAttachmentPhoto(data json.RawMessage) (Photo, error) {
	if len(data) == 0 {
		return Photo{}, nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return Photo{URL: s}, nil
	}
	var p Photo
	if err := json.Unmarshal(data, &p); err != nil {
		return Photo{}, err
	}
	return p, nil
}

// UpdatePostRequest represents a request to update a post
type UpdatePostRequest struct {
	Title       *string        `json:"title,omitempty" validate:"omitempty,max=255"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=5000"`
	Visibility  *PostVisibility `json:"visibility,omitempty" validate:"omitempty,oneof=PUBLIC FRIENDS PRIVATE"`

	// Sell-specific
	Price       *float64 `json:"price,omitempty" validate:"omitempty,min=0"`
	Discount    *float64 `json:"discount,omitempty" validate:"omitempty,min=0"`
	Free        *bool    `json:"free,omitempty"`
	Sold        *bool    `json:"sold,omitempty"`
	Currency    *string  `json:"currency,omitempty" validate:"omitempty,len=3"`
	CategoryID  *string  `json:"category_id,omitempty" validate:"omitempty,uuid"`
	CountryCode *string  `json:"country_code,omitempty"`
	ContactNo   *string  `json:"contact_no,omitempty"`
	IsLocation  *bool    `json:"is_location,omitempty"`

	// Event-specific
	StartDate *time.Time `json:"start_date,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// Attachment changes: newly uploaded photo objects / URLs, and IDs of attachments to remove.
	Attachments        []json.RawMessage `json:"attachments,omitempty"`
	DeletedAttachments []string          `json:"deleted_attachments,omitempty"`

	// PULL-specific: updated poll options (replaces existing options when present).
	PollOptions []string `json:"poll_options,omitempty" validate:"omitempty,min=2,max=10,dive,required,min=1,max=100"`
}

// PostResponse represents a post in API responses
type PostResponse struct {
	ID          string          `json:"id"`
	Type        PostType        `json:"type"`
	Title       *string         `json:"title,omitempty"`
	Description *string         `json:"description,omitempty"`
	Visibility  PostVisibility  `json:"visibility"`
	Status      bool            `json:"status"`

	// Author info
	Author  *AuthorInfo  `json:"author,omitempty"`
	Business *BusinessInfo `json:"business,omitempty"`

	// Attachments (full objects with id so the client can reference them for deletion)
	Attachments []AttachmentResponse `json:"attachments,omitempty"`

	// Sell-specific
	Currency    *string         `json:"currency,omitempty"`
	Price       *float64        `json:"price,omitempty"`
	Discount    *float64        `json:"discount,omitempty"`
	Free        *bool           `json:"free,omitempty"`
	Sold        *bool           `json:"sold,omitempty"`
	IsPromoted  *bool           `json:"is_promoted,omitempty"`
	CategoryID  *string         `json:"category_id,omitempty"` // so clients get ID for edit without parsing category.id
	Category    *CategoryInfo   `json:"category,omitempty"`
	ContactNo   *string         `json:"contact_no,omitempty"`
	IsLocation  *bool           `json:"is_location"` // when true, show item on map (SELL)

	// Event-specific
	StartDate       *time.Time  `json:"start_date,omitempty"`
	StartTime       *time.Time  `json:"start_time,omitempty"`
	EndDate         *time.Time  `json:"end_date,omitempty"`
	EndTime         *time.Time  `json:"end_time,omitempty"`
	EventState      *EventState `json:"event_state,omitempty"`
	InterestedCount *int        `json:"interested_count,omitempty"`
	GoingCount      *int        `json:"going_count,omitempty"`

	// Location
	Location     *LocationInfo `json:"location,omitempty"`

	// Engagement
	TotalComments  int  `json:"total_comments"`
	TotalLikes     int  `json:"total_likes"`
	TotalShares    int  `json:"total_shares"`
	LikedByMe      bool `json:"liked_by_me"`
	BookmarkedByMe bool `json:"bookmarked_by_me"`
	IsMine         bool `json:"is_mine"`

	// Original post (for shares)
	OriginalPost *PostResponse `json:"original_post,omitempty"`

	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AuthorInfo represents post author information
type AuthorInfo struct {
	UserID       string  `json:"user_id"`
	FirstName    *string `json:"first_name,omitempty"`
	LastName     *string `json:"last_name,omitempty"`
	FullName     string  `json:"full_name"`
	Avatar       *Photo  `json:"avatar"`
	Province     *string `json:"province"`
	District     *string `json:"district"`
	Neighborhood *string `json:"neighborhood"`
}

// BusinessInfo represents business information for business posts
type BusinessInfo struct {
	BusinessID string  `json:"business_id"`
	Name       string  `json:"name"`
	Avatar     *Photo  `json:"avatar,omitempty"`
}

// LocationInfo represents location information
type LocationInfo struct {
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	Country      *string  `json:"country,omitempty"`
	Province     *string  `json:"province,omitempty"`
	District     *string  `json:"district,omitempty"`
	Neighborhood *string  `json:"neighborhood,omitempty"`
}

// CategoryInfo represents category information
type CategoryInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Icon   Icon   `json:"icon"`
	Color  string `json:"color"`
}

// Icon represents an icon with library reference
type Icon struct {
	Name    string `json:"name"`
	Library string `json:"library"`
}

// PostLike represents a like on a post
type PostLike struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	PostID    string    `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}

// PostBookmark represents a bookmark on a post
type PostBookmark struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	PostID    string    `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}

// PostShare represents a share of a post
type PostShare struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	OriginalPostID string     `json:"original_post_id"`
	SharedPostID   *string    `json:"shared_post_id,omitempty"`
	ShareText      *string    `json:"share_text,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// FeedFilter represents filters for fetching posts
type FeedFilter struct {
	Type         *PostType  `json:"type,omitempty"`
	UserID       *string    `json:"user_id,omitempty"`
	BusinessID   *string    `json:"business_id,omitempty"`
	CategoryID   *string    `json:"category_id,omitempty"`
	Province     *string    `json:"province,omitempty"`
	SortBy       string     `json:"sort_by"` // recent, trending, nearby
	Limit        int        `json:"limit"`
	Offset       int        `json:"offset"`
	Latitude     *float64   `json:"latitude,omitempty"`
	Longitude    *float64   `json:"longitude,omitempty"`
	RadiusKm     *float64   `json:"radius_km,omitempty"`

	// Cursor-based pagination (preferred over offset for performance at scale).
	// When Cursor is set, Offset is ignored. Cursor is the created_at timestamp
	// of the last item from the previous page.
	Cursor       *time.Time `json:"cursor,omitempty"`
}
