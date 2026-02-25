package models

import "time"

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email      string   `json:"email" validate:"required,email"`
	Password   string   `json:"password" validate:"required,min=8"`
	FirstName  string   `json:"first_name" validate:"required,min=2,max=100"`
	LastName   string   `json:"last_name" validate:"required,min=2,max=100"`
	Latitude   float64  `json:"latitude" validate:"required,latitude"`
	Longitude  float64  `json:"longitude" validate:"required,longitude"`
	DeviceInfo *string  `json:"device_info,omitempty"`
	IPAddress  *string  `json:"-"` // Set from request context
	UserAgent  *string  `json:"-"` // Set from request context
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email      string  `json:"email" validate:"required,email"`
	Password   string  `json:"password" validate:"required"`
	DeviceInfo *string `json:"device_info,omitempty"`
	IPAddress  *string `json:"-"` // Set from request context
	UserAgent  *string `json:"-"` // Set from request context
}

// UnifiedAuthRequest represents a unified authentication request (login or register)
// If user exists, it logs them in. If not, it registers them.
type UnifiedAuthRequest struct {
	Email      string   `json:"email" validate:"required,email"`
	Password   string   `json:"password" validate:"required,min=8"`
	FirstName  *string  `json:"first_name,omitempty" validate:"omitempty,min=2,max=100"`
	LastName   *string  `json:"last_name,omitempty" validate:"omitempty,min=2,max=100"`
	Latitude   *float64 `json:"latitude,omitempty" validate:"omitempty,latitude"`
	Longitude  *float64 `json:"longitude,omitempty" validate:"omitempty,longitude"`
	DeviceInfo *string  `json:"device_info,omitempty"`
	IPAddress  *string  `json:"-"` // Set from request context
	UserAgent  *string  `json:"-"` // Set from request context
}

// RefreshTokenRequest represents a refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ForgotPasswordRequest represents a forgot password request
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents a reset password request
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ChangePasswordRequest represents a change password request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// VerifyEmailRequest represents an email verification request
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// OAuthRequest represents an OAuth login request
type OAuthRequest struct {
	IDToken  string `json:"id_token" validate:"required"`
	Provider string `json:"provider" validate:"required,oneof=google apple facebook"`
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// UserResponse represents a sanitized user for API responses
type UserResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Role          UserRole  `json:"role,omitempty"`
	FirstName     *string   `json:"first_name,omitempty"`
	LastName      *string   `json:"last_name,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	MFAEnabled    bool      `json:"mfa_enabled"`
	CreatedAt     time.Time `json:"created_at"`
}

// ProfileResponse represents a profile for API responses
type ProfileResponse struct {
	ID           string  `json:"id"`
	FirstName    *string `json:"first_name,omitempty"`
	LastName     *string `json:"last_name,omitempty"`
	Avatar       *Photo  `json:"avatar,omitempty"`
	AvatarColor  *string `json:"avatar_color,omitempty"`
	Province     *string `json:"province,omitempty"`
	District     *string `json:"district,omitempty"`
	Neighborhood *string `json:"neighborhood,omitempty"`
	Country      *string `json:"country,omitempty"`
	IsComplete   bool    `json:"is_complete"`
}

// AuthResponse represents the response after successful authentication
type AuthResponse struct {
	User           *UserResponse `json:"user,omitempty"`
	Profile        *ProfileResponse `json:"profile,omitempty"`
	Tokens         *TokenPair    `json:"tokens,omitempty"`
	RequiresMFA    bool          `json:"requires_mfa"`
	MFAChallengeID *string       `json:"mfa_challenge_id,omitempty"`
}

// MFAChallenge represents an MFA challenge that needs to be verified
type MFAChallenge struct {
	ChallengeID string    `json:"challenge_id"`
	FactorType  string    `json:"factor_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// LoginResponse can return either auth response or MFA challenge
type LoginResponse struct {
	RequiresMFA  bool          `json:"requires_mfa"`
	Auth         *AuthResponse `json:"auth,omitempty"`
	MFAChallenge *MFAChallenge `json:"mfa_challenge,omitempty"`
}

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	AAL       int    `json:"aal"` // Authentication Assurance Level (1 or 2)
	SessionID string `json:"session_id"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Issuer    string `json:"iss"`
}

// AAL (Authentication Assurance Level)
const (
	AAL1 = 1 // Basic authentication (email/password or OAuth)
	AAL2 = 2 // MFA verified
)
