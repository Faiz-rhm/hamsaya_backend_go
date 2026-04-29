package models

import "time"

// CustomRole is an admin-defined permission bundle assignable to users.
type CustomRole struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Permissions []string  `json:"permissions"`
	CreatedBy   *string   `json:"created_by,omitempty"`
	UpdatedBy   *string   `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// UserCount is populated on list responses.
	UserCount int `json:"user_count,omitempty"`
}

// CreateCustomRoleRequest is the payload for creating a new role.
type CreateCustomRoleRequest struct {
	Name        string   `json:"name"        validate:"required,min=2,max=64"`
	Description *string  `json:"description" validate:"omitempty,max=500"`
	Permissions []string `json:"permissions" validate:"required"`
}

// UpdateCustomRoleRequest is the partial-update payload.
type UpdateCustomRoleRequest struct {
	Name        *string  `json:"name"        validate:"omitempty,min=2,max=64"`
	Description *string  `json:"description" validate:"omitempty,max=500"`
	Permissions []string `json:"permissions"`
}

// AssignCustomRoleRequest assigns or clears a custom role for a user.
type AssignCustomRoleRequest struct {
	// CustomRoleID nil or "" = clear the user's custom role.
	CustomRoleID *string `json:"custom_role_id"`
}

// CustomRoleUser is a lightweight user summary returned when listing users
// assigned to a custom role.
type CustomRoleUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}
