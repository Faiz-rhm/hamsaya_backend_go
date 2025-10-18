package models

import "time"

// CategoryStatus represents the status of a category
type CategoryStatus string

const (
	CategoryStatusActive   CategoryStatus = "ACTIVE"
	CategoryStatusInactive CategoryStatus = "INACTIVE"
)

// CategoryIcon represents the icon information stored in JSONB
type CategoryIcon struct {
	Name    string `json:"name"`
	Library string `json:"library"`
}

// SellCategory represents a marketplace category
type SellCategory struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Icon      CategoryIcon   `json:"icon"`
	Color     string         `json:"color"`
	Status    CategoryStatus `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
}

// CategoryResponse is the API response for a single category
type CategoryResponse struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Icon      CategoryIcon `json:"icon"`
	Color     string       `json:"color"`
	Status    string       `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
}

// CreateCategoryRequest represents the request to create a category
type CreateCategoryRequest struct {
	Name   string       `json:"name" validate:"required,min=2,max=100"`
	Icon   CategoryIcon `json:"icon" validate:"required"`
	Color  string       `json:"color" validate:"required,hexcolor|rgb|rgba"`
	Status string       `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE INACTIVE"`
}

// UpdateCategoryRequest represents the request to update a category
type UpdateCategoryRequest struct {
	Name   *string       `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Icon   *CategoryIcon `json:"icon,omitempty"`
	Color  *string       `json:"color,omitempty" validate:"omitempty,hexcolor|rgb|rgba"`
	Status *string       `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE INACTIVE"`
}

// CategoryListFilter represents filters for listing categories
type CategoryListFilter struct {
	Status *CategoryStatus
	Limit  int
	Offset int
}

// ToCategoryResponse converts a SellCategory to CategoryResponse
func (c *SellCategory) ToCategoryResponse() *CategoryResponse {
	return &CategoryResponse{
		ID:        c.ID,
		Name:      c.Name,
		Icon:      c.Icon,
		Color:     c.Color,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt,
	}
}
