// Package crypto provides at-rest encryption for sensitive secrets stored
// in the database (currently MFA TOTP secrets, future: OAuth tokens).
//
// Algorithm: AES-256-GCM with a random 12-byte nonce. The on-wire format
// is base64( nonce || ciphertext || authtag ) prefixed with a versioned
// magic so we can rotate algorithms or keys without breaking older rows:
//
//   "mfaenc:v1:" + base64(nonce || ciphertext+tag)
//
// Backwards compatibility: rows written before this layer have no magic
// prefix. Decrypt detects that case and returns the raw value unchanged
// so existing TOTP factors keep verifying. A subsequent write through
// the repository re-encrypts the value into the new format.
//
// Key management: 32-byte hex key sourced from env MFA_SECRET_ENCRYPTION_KEY.
// Generate with: `openssl rand -hex 32`. Rotate by adding a v2 magic and
// supporting both v1 and v2 read paths.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	magicPrefix = "mfaenc:v1:"
	keyLenBytes = 32
)

// SecretCipher encrypts/decrypts secrets at rest using AES-256-GCM.
type SecretCipher struct {
	gcm cipher.AEAD
}

// NewSecretCipher constructs a cipher from a 32-byte hex key. Returns an
// error if the key is missing, malformed, or wrong size.
func NewSecretCipher(hexKey string) (*SecretCipher, error) {
	if hexKey == "" {
		return nil, errors.New("crypto: encryption key is empty")
	}
	key, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil {
		return nil, fmt.Errorf("crypto: key is not valid hex: %w", err)
	}
	if len(key) != keyLenBytes {
		return nil, fmt.Errorf("crypto: key must be %d bytes (got %d)", keyLenBytes, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: cipher.NewGCM: %w", err)
	}
	return &SecretCipher{gcm: gcm}, nil
}

// Encrypt wraps plaintext into the magic-prefixed base64 envelope.
func (c *SecretCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	ct := c.gcm.Seal(nil, nonce, []byte(plaintext), nil)
	body := append(nonce, ct...)
	return magicPrefix + base64.StdEncoding.EncodeToString(body), nil
}

// Decrypt unwraps a magic-prefixed envelope. If the input lacks the magic
// prefix it is treated as legacy plaintext and returned unchanged — see
// the backwards-compat note in the package comment.
func (c *SecretCipher) Decrypt(envelope string) (string, error) {
	if !strings.HasPrefix(envelope, magicPrefix) {
		// Legacy plaintext — return as-is so existing rows keep working.
		return envelope, nil
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(envelope, magicPrefix))
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}
	nonceSize := c.gcm.NonceSize()
	if len(body) < nonceSize {
		return "", errors.New("crypto: envelope too short")
	}
	nonce, ct := body[:nonceSize], body[nonceSize:]
	pt, err := c.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: open: %w", err)
	}
	return string(pt), nil
}

// IsEncrypted returns true if envelope was produced by [Encrypt].
func IsEncrypted(envelope string) bool {
	return strings.HasPrefix(envelope, magicPrefix)
}
