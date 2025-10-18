package models

import "time"

// MFAFactor represents a multi-factor authentication method
type MFAFactor struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Type      string     `json:"type"` // TOTP, SMS, EMAIL
	SecretKey *string    `json:"-"`    // Never expose
	FactorID  *string    `json:"factor_id,omitempty"`
	Status    string     `json:"status"` // verified, unverified
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// MFAEnrollRequest represents an MFA enrollment request
type MFAEnrollRequest struct {
	Type string `json:"type" validate:"required,oneof=TOTP SMS EMAIL"`
}

// MFAEnrollResponse represents the response after MFA enrollment
type MFAEnrollResponse struct {
	FactorID    string   `json:"factor_id"`
	Type        string   `json:"type"`
	QRCodeURL   string   `json:"qr_code_url,omitempty"`   // For TOTP
	SecretKey   string   `json:"secret_key,omitempty"`    // For TOTP (backup)
	BackupCodes []string `json:"backup_codes"`
}

// MFAVerifyRequest represents an MFA verification request
type MFAVerifyRequest struct {
	FactorID string `json:"factor_id" validate:"required"`
	Code     string `json:"code" validate:"required,len=6"`
}

// MFAVerifyChallengeRequest represents verifying an MFA challenge during login
type MFAVerifyChallengeRequest struct {
	ChallengeID string `json:"challenge_id" validate:"required"`
	Code        string `json:"code" validate:"required,len=6"`
}

// MFABackupCodeRequest represents using a backup code
type MFABackupCodeRequest struct {
	ChallengeID string `json:"challenge_id" validate:"required"`
	BackupCode  string `json:"backup_code" validate:"required"`
}

// MFADisableRequest represents disabling MFA
type MFADisableRequest struct {
	Password string `json:"password" validate:"required"`
}

// BackupCode represents an MFA backup code
type BackupCode struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Code      string     `json:"code"`
	Used      bool       `json:"used"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
