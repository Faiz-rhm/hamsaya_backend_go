package models

import "time"

// BusinessReview is a single user-authored review for a business profile.
// Aggregates (avg + count) live on business_profiles and are kept in sync
// by a database trigger; this struct only carries the row state.
type BusinessReview struct {
	ID                string    `json:"id"`
	BusinessProfileID string    `json:"business_profile_id"`
	UserID            string    `json:"user_id"`
	Rating            int       `json:"rating"`
	Comment           *string   `json:"comment,omitempty"`
	IsHidden          bool      `json:"is_hidden"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// BusinessReviewWithAuthor enriches a review with the author's display data
// for the list endpoint. Avoids a second round-trip to fetch profiles.
type BusinessReviewWithAuthor struct {
	BusinessReview
	AuthorFirstName *string `json:"author_first_name,omitempty"`
	AuthorLastName  *string `json:"author_last_name,omitempty"`
	AuthorAvatar    *Photo  `json:"author_avatar,omitempty"`
	AuthorAvatarHex *string `json:"author_avatar_color,omitempty"`
}

// CreateBusinessReviewRequest is the body for creating or upserting a review.
// Editing an existing review hits the same endpoint — the unique constraint
// on (business_profile_id, user_id) makes this naturally idempotent.
type CreateBusinessReviewRequest struct {
	Rating  int     `json:"rating" validate:"required,min=1,max=5"`
	Comment *string `json:"comment,omitempty" validate:"omitempty,max=2000"`
}

// UpdateBusinessReviewRequest is for editing your own review later.
type UpdateBusinessReviewRequest struct {
	Rating  *int    `json:"rating,omitempty" validate:"omitempty,min=1,max=5"`
	Comment *string `json:"comment,omitempty" validate:"omitempty,max=2000"`
}

// BusinessReviewStats holds aggregate stats for the summary card.
// Distribution is the count per star (index 0 = 1-star, ..., index 4 = 5-star).
type BusinessReviewStats struct {
	BusinessProfileID string  `json:"business_profile_id"`
	AvgRating         float64 `json:"avg_rating"`
	ReviewCount       int     `json:"review_count"`
	Distribution      [5]int  `json:"distribution"`
}
