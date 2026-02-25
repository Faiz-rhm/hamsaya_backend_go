package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// UserRole represents the role of a user
type UserRole string

const (
	RoleUser      UserRole = "user"
	RoleAdmin     UserRole = "admin"
	RoleModerator UserRole = "moderator"
)

// User represents a user account
type User struct {
	ID                   string             `json:"id"`
	Email                string             `json:"email"`
	Phone                *string            `json:"phone,omitempty"`
	PasswordHash         *string            `json:"-"` // Never expose password hash
	EmailVerified        bool               `json:"email_verified"`
	PhoneVerified        bool               `json:"phone_verified"`
	MFAEnabled           bool               `json:"mfa_enabled"`
	Role                 UserRole           `json:"role"`
	OAuthProvider        *string            `json:"oauth_provider,omitempty"`
	OAuthProviderID      *string            `json:"-"`
	LastLoginAt          *time.Time         `json:"last_login_at,omitempty"`
	FailedLoginAttempts  int                `json:"-"`
	LockedUntil          *time.Time         `json:"-"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
	DeletedAt            *time.Time         `json:"-"`
}

// AvatarColors are predefined hex colors for avatar placeholders (no photo).
var AvatarColors = []string{
	"#E57373", "#F06292", "#BA68C8", "#9575CD", "#7986CB",
	"#64B5F6", "#4FC3F7", "#4DD0E1", "#4DB6AC", "#81C784",
	"#AED581", "#DCE775", "#FFF176", "#FFD54F", "#FFB74D",
	"#FF8A65", "#A1887F",
}

// RandomAvatarColor returns a random hex color from AvatarColors for new profiles.
func RandomAvatarColor() string {
	return AvatarColors[rand.Intn(len(AvatarColors))]
}

// Profile represents extended user profile information
type Profile struct {
	ID           string                 `json:"id"`
	FirstName    *string                `json:"first_name,omitempty"`
	LastName     *string                `json:"last_name,omitempty"`
	Avatar       *Photo                 `json:"avatar,omitempty"`
	AvatarColor  *string                `json:"avatar_color,omitempty"` // Hex for placeholder when no avatar
	Cover        *Photo                 `json:"cover,omitempty"`
	About        *string                `json:"about,omitempty"`
	Gender       *string                `json:"gender,omitempty"`
	DOB          *time.Time             `json:"dob,omitempty"`
	Website      *string                `json:"website,omitempty"`
	Location     *pgtype.Point          `json:"location,omitempty"`
	Country      *string                `json:"country,omitempty"`
	Province     *string                `json:"province,omitempty"`
	District     *string                `json:"district,omitempty"`
	Neighborhood *string                `json:"neighborhood,omitempty"`
	IsComplete   bool                   `json:"is_complete"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	DeletedAt    *time.Time             `json:"-"`
}

// Photo represents an image with metadata
type Photo struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MimeType string `json:"mime_type"`
}

// Scan implements the sql.Scanner interface for Photo to handle JSONB from PostgreSQL
func (p *Photo) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	var source []byte
	switch v := src.(type) {
	case []byte:
		source = v
	case string:
		source = []byte(v)
	default:
		return fmt.Errorf("unsupported type for Photo: %T", src)
	}

	return json.Unmarshal(source, p)
}

// Value implements the driver.Valuer interface for Photo to handle JSONB to PostgreSQL
func (p Photo) Value() (driver.Value, error) {
	if p.URL == "" {
		return nil, nil
	}
	return json.Marshal(p)
}

// UserSession represents an active user session
type UserSession struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	RefreshToken     string     `json:"-"` // Never expose
	RefreshTokenHash string     `json:"-"` // SHA-256 hash of refresh token for secure lookup
	AccessTokenHash  string     `json:"-"` // Never expose
	DeviceInfo       *string    `json:"device_info,omitempty"`
	IPAddress        *string    `json:"ip_address,omitempty"`
	UserAgent        *string    `json:"user_agent,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	Revoked          bool       `json:"revoked"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// IsLocked checks if the user account is currently locked
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return u.LockedUntil.After(time.Now())
}

// IsAdmin checks if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsModerator checks if the user has moderator role
func (u *User) IsModerator() bool {
	return u.Role == RoleModerator
}

// IsAdminOrModerator checks if the user has admin or moderator role
func (u *User) IsAdminOrModerator() bool {
	return u.Role == RoleAdmin || u.Role == RoleModerator
}

// FullName returns the user's full name
func (p *Profile) FullName() string {
	if p.FirstName == nil && p.LastName == nil {
		return ""
	}
	if p.FirstName == nil {
		return *p.LastName
	}
	if p.LastName == nil {
		return *p.FirstName
	}
	return *p.FirstName + " " + *p.LastName
}
