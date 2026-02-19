package models

import "time"

// SearchType represents the type of search
type SearchType string

const (
	SearchTypeAll       SearchType = "all"
	SearchTypePosts     SearchType = "posts"
	SearchTypeUsers     SearchType = "users"
	SearchTypeBusinesses SearchType = "businesses"
)

// SearchRequest represents a search request
type SearchRequest struct {
	Query     string     `json:"query" validate:"required,min=2"`
	Type      SearchType `json:"type" validate:"omitempty,oneof=all posts users businesses"`
	Limit     int        `json:"limit" validate:"omitempty,min=1,max=100"`
	Offset    int        `json:"offset" validate:"omitempty,min=0"`
	Latitude  *float64   `json:"latitude" validate:"omitempty,latitude"`
	Longitude *float64   `json:"longitude" validate:"omitempty,longitude"`
	RadiusKm  *float64   `json:"radius_km" validate:"omitempty,min=0,max=1000"`
}

// SearchResponse represents aggregated search results
type SearchResponse struct {
	Posts      []*PostResponse         `json:"posts"`
	Users      []*UserSearchResult     `json:"users"`
	Businesses []*BusinessSearchResult `json:"businesses"`
	Total      int                     `json:"total"`
}

// BusinessSearchResult represents a business in search results
type BusinessSearchResult struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    *string   `json:"description,omitempty"`
	Avatar         *Photo    `json:"avatar,omitempty"`
	Cover          *Photo    `json:"cover,omitempty"`
	Address        *string   `json:"address,omitempty"`
	PhoneNumber    *string   `json:"phone_number,omitempty"`
	Website        *string   `json:"website,omitempty"`
	Categories     []string  `json:"categories,omitempty"`
	Location       *Location `json:"location,omitempty"`
	Distance       *float64  `json:"distance,omitempty"` // Distance in km from search point
	TotalFollow    int       `json:"total_follow"`
	TotalViews     int       `json:"total_views"`
	IsFollowing    bool      `json:"is_following,omitempty"`
}

// DiscoverFilter is the tab/filter for discover: all, business, event, sell
type DiscoverFilter string

const (
	DiscoverFilterAll      DiscoverFilter = "all"
	DiscoverFilterBusiness DiscoverFilter = "business"
	DiscoverFilterEvent    DiscoverFilter = "event"
	DiscoverFilterSell     DiscoverFilter = "sell"
)

// DiscoverRequest represents a discovery/map request
type DiscoverRequest struct {
	Latitude  float64        `json:"latitude" validate:"required,latitude"`
	Longitude float64        `json:"longitude" validate:"required,longitude"`
	RadiusKm  float64        `json:"radius_km" validate:"required,min=0.1,max=100"`
	Filter    DiscoverFilter `json:"filter" validate:"omitempty,oneof=all business event sell"`
	Type      *PostType      `json:"type" validate:"omitempty,oneof=FEED EVENT SELL PULL"`
	Limit     int            `json:"limit" validate:"omitempty,min=1,max=500"`
}

// DiscoverResponse represents discovery results
type DiscoverResponse struct {
	Posts      []*DiscoverPost     `json:"posts"`
	Businesses []*DiscoverBusiness `json:"businesses"`
	Total      int                 `json:"total"`
}

// DiscoverPost represents a post marker on the map
type DiscoverPost struct {
	ID          string    `json:"id"`
	Type        PostType  `json:"type"`
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Thumbnail   *Photo    `json:"thumbnail,omitempty"` // First attachment
	Location    *Location `json:"location"`
	Distance    float64   `json:"distance"` // Distance in km from search point
	Price       *float64  `json:"price,omitempty"`
	CreatedAt   time.Time `json:"created_at"`

	// For EVENT type
	StartDate *string `json:"start_date,omitempty"`
	StartTime *string `json:"start_time,omitempty"`
}

// DiscoverBusiness represents a business marker on the map
type DiscoverBusiness struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Avatar      *Photo    `json:"avatar,omitempty"`
	Location    *Location `json:"location"`
	Distance    float64   `json:"distance"` // Distance in km from search point
	Categories  []string  `json:"categories,omitempty"`
	TotalFollow int       `json:"total_follow"`
}

// Location represents geographic coordinates
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   *string `json:"country,omitempty"`
	Province  *string `json:"province,omitempty"`
	District  *string `json:"district,omitempty"`
}

// SearchFilter represents advanced search filters
type SearchFilter struct {
	Query      string
	Type       SearchType
	Limit      int
	Offset     int
	UserID     *string // For personalized results
	Latitude   *float64
	Longitude  *float64
	RadiusKm   *float64
}
