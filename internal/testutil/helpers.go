package testutil

import (
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/utils"
)

// CreateTestUser creates a test user with default values
func CreateTestUser(id, email string) *models.User {
	now := time.Now()
	passwordHash := "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5d8dBx7gBZ.v2" // "password"
	return &models.User{
		ID:            id,
		Email:         email,
		PasswordHash:  &passwordHash,
		EmailVerified: true,
		Role:          models.RoleUser,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CreateTestAdminUser creates a test admin user
func CreateTestAdminUser(id, email string) *models.User {
	user := CreateTestUser(id, email)
	user.Role = models.RoleAdmin
	return user
}

// CreateTestProfile creates a test profile
func CreateTestProfile(id, firstName, lastName string) *models.Profile {
	now := time.Now()
	return &models.Profile{
		ID:        id,
		FirstName: &firstName,
		LastName:  &lastName,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// CreateTestPost creates a test post
func CreateTestPost(id, userID string, postType models.PostType) *models.Post {
	now := time.Now()
	description := "Test post content"
	return &models.Post{
		ID:          id,
		UserID:      &userID,
		Type:        postType,
		Description: &description,
		Status:      true,
		Visibility:  models.VisibilityPublic,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// CreateTestBusiness creates a test business
func CreateTestBusiness(id, userID, name string) *models.BusinessProfile {
	now := time.Now()
	return &models.BusinessProfile{
		ID:        id,
		UserID:    userID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// CreateTestValidator creates a test validator
func CreateTestValidator() *utils.Validator {
	return utils.NewValidator()
}

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to the given int
func IntPtr(i int) *int {
	return &i
}

// BoolPtr returns a pointer to the given bool
func BoolPtr(b bool) *bool {
	return &b
}

// TimePtr returns a pointer to the given time
func TimePtr(t time.Time) *time.Time {
	return &t
}
