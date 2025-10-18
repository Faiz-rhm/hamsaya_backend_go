package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordService_Hash(t *testing.T) {
	service := NewPasswordService()

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "MySecurePass123!",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false, // bcrypt allows empty passwords
		},
		{
			name:     "long password",
			password: "ThisIsAVeryLongPasswordThatShouldStillWorkCorrectly123!@#$%^&*()",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := service.Hash(tt.password)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, hash)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, hash)
				// Hash should be different from original password
				assert.NotEqual(t, tt.password, hash)
				// Hash should start with bcrypt prefix
				assert.Contains(t, hash, "$2a$")
			}
		})
	}
}

func TestPasswordService_Verify(t *testing.T) {
	service := NewPasswordService()
	password := "MySecurePass123!"
	hash, err := service.Hash(password)
	require.NoError(t, err)

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			want:     true,
		},
		{
			name:     "incorrect password",
			password: "WrongPassword",
			hash:     hash,
			want:     false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			want:     false,
		},
		{
			name:     "case sensitive - uppercase",
			password: "MYSECUREPASS123!",
			hash:     hash,
			want:     false,
		},
		{
			name:     "case sensitive - lowercase",
			password: "mysecurepass123!",
			hash:     hash,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.Verify(tt.password, tt.hash)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestPasswordService_ValidatePasswordStrength(t *testing.T) {
	service := NewPasswordService()

	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid password",
			password: "MySecure123!",
			wantErr:  false,
		},
		{
			name:     "another valid password",
			password: "Passw0rd@2024",
			wantErr:  false,
		},
		{
			name:     "too short",
			password: "Ab1!",
			wantErr:  true,
			errMsg:   "at least 8 characters long",
		},
		{
			name:     "no uppercase",
			password: "mysecure123!",
			wantErr:  true,
			errMsg:   "uppercase letter",
		},
		{
			name:     "no lowercase",
			password: "MYSECURE123!",
			wantErr:  true,
			errMsg:   "lowercase letter",
		},
		{
			name:     "no number",
			password: "MySecurePass!",
			wantErr:  true,
			errMsg:   "number",
		},
		{
			name:     "no special character",
			password: "MySecure123",
			wantErr:  true,
			errMsg:   "special character",
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
			errMsg:   "at least 8 characters long",
		},
		{
			name:     "only spaces",
			password: "        ",
			wantErr:  true,
			errMsg:   "uppercase letter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidatePasswordStrength(tt.password)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPasswordService_GenerateSecureToken(t *testing.T) {
	service := NewPasswordService()

	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			name:    "generate 16 bytes",
			length:  16,
			wantErr: false,
		},
		{
			name:    "generate 32 bytes",
			length:  32,
			wantErr: false,
		},
		{
			name:    "generate 64 bytes",
			length:  64,
			wantErr: false,
		},
		{
			name:    "generate 0 bytes",
			length:  0,
			wantErr: false, // Should generate empty token
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := service.GenerateSecureToken(tt.length)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				if tt.length > 0 {
					assert.NotEmpty(t, token)
				}
			}
		})
	}

	// Test uniqueness
	t.Run("tokens are unique", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := service.GenerateSecureToken(32)
			require.NoError(t, err)
			assert.False(t, tokens[token], "generated duplicate token")
			tokens[token] = true
		}
	})
}

func TestPasswordService_HashAndVerify_Integration(t *testing.T) {
	service := NewPasswordService()

	// Test that we can hash and verify multiple passwords
	passwords := []string{
		"Password123!",
		"AnotherSecure456@",
		"Testing789#",
	}

	hashes := make([]string, len(passwords))
	for i, password := range passwords {
		hash, err := service.Hash(password)
		require.NoError(t, err)
		hashes[i] = hash
	}

	// Verify each password with its own hash
	for i, password := range passwords {
		assert.True(t, service.Verify(password, hashes[i]))
	}

	// Verify passwords don't match other hashes
	for i, password := range passwords {
		for j, hash := range hashes {
			if i != j {
				assert.False(t, service.Verify(password, hash))
			}
		}
	}
}

func TestPasswordService_SamePasswordDifferentHashes(t *testing.T) {
	service := NewPasswordService()
	password := "MyPassword123!"

	// Generate multiple hashes for the same password
	hash1, err1 := service.Hash(password)
	hash2, err2 := service.Hash(password)

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Hashes should be different due to salt
	assert.NotEqual(t, hash1, hash2)

	// But both should verify against the same password
	assert.True(t, service.Verify(password, hash1))
	assert.True(t, service.Verify(password, hash2))
}
