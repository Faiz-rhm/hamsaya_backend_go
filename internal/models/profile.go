package models

import "time"

// UpdateProfileRequest represents a request to update user profile
type UpdateProfileRequest struct {
	FirstName    *string    `json:"first_name,omitempty" validate:"omitempty,min=2,max=100"`
	LastName     *string    `json:"last_name,omitempty" validate:"omitempty,min=2,max=100"`
	About        *string    `json:"about,omitempty" validate:"omitempty,max=500"`
	Gender       *string    `json:"gender,omitempty" validate:"omitempty,oneof=male female other prefer_not_to_say"`
	DOB          *time.Time `json:"dob,omitempty"`
	Website      *string    `json:"website,omitempty" validate:"omitempty,url"`
	Country      *string    `json:"country,omitempty" validate:"omitempty,max=100"`
	Province     *string    `json:"province,omitempty" validate:"omitempty,max=100"`
	District     *string    `json:"district,omitempty" validate:"omitempty,max=100"`
	Neighborhood *string    `json:"neighborhood,omitempty" validate:"omitempty,max=100"`
	Latitude     *float64   `json:"latitude,omitempty" validate:"omitempty,latitude"`
	Longitude    *float64   `json:"longitude,omitempty" validate:"omitempty,longitude"`
}

// FullProfileResponse represents complete profile information
type FullProfileResponse struct {
	ID           string     `json:"id"`
	FirstName    *string    `json:"first_name,omitempty"`
	LastName     *string    `json:"last_name,omitempty"`
	FullName     string     `json:"full_name"`
	Avatar       *Photo     `json:"avatar,omitempty"`
	Cover        *Photo     `json:"cover,omitempty"`
	About        *string    `json:"about,omitempty"`
	Gender       *string    `json:"gender,omitempty"`
	DOB          *time.Time `json:"dob,omitempty"`
	Website      *string    `json:"website,omitempty"`
	Country      *string    `json:"country,omitempty"`
	Province     *string    `json:"province,omitempty"`
	District     *string    `json:"district,omitempty"`
	Neighborhood *string    `json:"neighborhood,omitempty"`
	IsComplete   bool       `json:"is_complete"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// User info
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	MFAEnabled    bool      `json:"mfa_enabled"`

	// Stats (will be populated later)
	FollowersCount  int `json:"followers_count"`
	FollowingCount  int `json:"following_count"`
	PostsCount      int `json:"posts_count"`

	// Relationship status (relative to authenticated user)
	IsFollowing     bool `json:"is_following,omitempty"`
	IsFollowedBy    bool `json:"is_followed_by,omitempty"`
	IsBlocked       bool `json:"is_blocked,omitempty"`
	HasBlockedMe    bool `json:"has_blocked_me,omitempty"`
}

// UserSearchResult represents a user in search results
type UserSearchResult struct {
	ID        string  `json:"id"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	FullName  string  `json:"full_name"`
	Avatar    *Photo  `json:"avatar,omitempty"`
	About     *string `json:"about,omitempty"`
	Province  *string `json:"province,omitempty"`

	// Relationship status
	IsFollowing  bool `json:"is_following"`
	IsFollowedBy bool `json:"is_followed_by"`
}

// UploadImageRequest represents an image upload request
type UploadImageRequest struct {
	ImageType string `json:"image_type" validate:"required,oneof=avatar cover"`
}

// UploadImageResponse represents an image upload response
type UploadImageResponse struct {
	Photo *Photo `json:"photo"`
}

// ToFullProfileResponse converts Profile and User to FullProfileResponse
func ToFullProfileResponse(user *User, profile *Profile) *FullProfileResponse {
	resp := &FullProfileResponse{
		ID:            profile.ID,
		FirstName:     profile.FirstName,
		LastName:      profile.LastName,
		FullName:      profile.FullName(),
		Avatar:        profile.Avatar,
		Cover:         profile.Cover,
		About:         profile.About,
		Gender:        profile.Gender,
		DOB:           profile.DOB,
		Website:       profile.Website,
		Country:       profile.Country,
		Province:      profile.Province,
		District:      profile.District,
		Neighborhood:  profile.Neighborhood,
		IsComplete:    profile.IsComplete,
		CreatedAt:     profile.CreatedAt,
		UpdatedAt:     profile.UpdatedAt,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		PhoneVerified: user.PhoneVerified,
		MFAEnabled:    user.MFAEnabled,
		// Stats will be populated by service layer
		FollowersCount: 0,
		FollowingCount: 0,
		PostsCount:     0,
	}
	return resp
}

// ToUserSearchResult converts Profile to UserSearchResult
func ToUserSearchResult(profile *Profile) *UserSearchResult {
	return &UserSearchResult{
		ID:        profile.ID,
		FirstName: profile.FirstName,
		LastName:  profile.LastName,
		FullName:  profile.FullName(),
		Avatar:    profile.Avatar,
		About:     profile.About,
		Province:  profile.Province,
	}
}
