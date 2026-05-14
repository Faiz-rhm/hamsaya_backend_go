package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordService handles password hashing and verification
type PasswordService struct {
	cost int
}

// NewPasswordService creates a new password service
func NewPasswordService() *PasswordService {
	return &PasswordService{
		// 13 ≈ 350-500ms per hash on modern hardware — slow enough that
		// online brute-force is hopeless yet still acceptable login latency.
		// Existing cost-12 hashes verify fine; only newly-hashed passwords
		// pick up cost-13. Bump again in 2-3 years.
		cost: 13,
	}
}

// NewPasswordServiceWithCost creates a password service with a specific bcrypt cost.
// Use bcrypt.MinCost (4) in tests to avoid multi-second hashes per register call.
func NewPasswordServiceWithCost(cost int) *PasswordService {
	return &PasswordService{cost: cost}
}

// Hash hashes a password using bcrypt
func (s *PasswordService) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// Verify checks if a password matches the hash
func (s *PasswordService) Verify(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateSecureToken generates a cryptographically secure random token
func (s *PasswordService) GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidatePasswordStrength validates password strength.
// Operator preference: minimum 8 characters, no character-class
// requirements (no forced upper/lower/number/special). Length + bcrypt
// cost-13 hashing carries the security weight; complexity rules push
// users toward predictable patterns (`Password1!`) without adding real
// entropy.
func (s *PasswordService) ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	return nil
}
