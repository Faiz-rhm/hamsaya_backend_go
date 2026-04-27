package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
)

// validKey is a deterministic 32-byte hex string for tests.
const validKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestNewSecretCipher_Validation(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "empty", key: "", wantErr: "empty"},
		{name: "not hex", key: "zzzz", wantErr: "valid hex"},
		{name: "wrong length", key: "deadbeef", wantErr: "32 bytes"},
		{name: "valid", key: validKey, wantErr: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSecretCipher(tc.key)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := NewSecretCipher(validKey)
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{
		"",
		"a",
		"JBSWY3DPEHPK3PXP", // typical TOTP secret
		strings.Repeat("z", 1024),
	}
	for _, plaintext := range cases {
		envelope, err := c.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plaintext, err)
		}
		if !strings.HasPrefix(envelope, magicPrefix) {
			t.Fatalf("envelope missing magic prefix: %q", envelope)
		}
		got, err := c.Decrypt(envelope)
		if err != nil {
			t.Fatalf("decrypt %q: %v", envelope, err)
		}
		if got != plaintext {
			t.Fatalf("round-trip mismatch: got %q, want %q", got, plaintext)
		}
	}
}

func TestEncrypt_NonceUniqueness(t *testing.T) {
	c, err := NewSecretCipher(validKey)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.Encrypt("same plaintext")
	b, _ := c.Encrypt("same plaintext")
	if a == b {
		t.Fatal("two encryptions of the same plaintext produced identical envelopes — nonce isn't random")
	}
}

func TestDecrypt_LegacyPlaintextPassthrough(t *testing.T) {
	c, err := NewSecretCipher(validKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Decrypt("legacy-totp-secret")
	if err != nil {
		t.Fatalf("legacy passthrough: %v", err)
	}
	if got != "legacy-totp-secret" {
		t.Fatalf("legacy passthrough mutated value: %q", got)
	}
}

func TestDecrypt_MalformedEnvelope(t *testing.T) {
	c, err := NewSecretCipher(validKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decrypt(magicPrefix + "not-base64!!!"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if _, err := c.Decrypt(magicPrefix + "AAAA"); err == nil {
		t.Fatal("expected error for too-short envelope")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	c1, _ := NewSecretCipher(validKey)
	envelope, _ := c1.Encrypt("secret")

	otherKey := make([]byte, 32)
	_, _ = rand.Read(otherKey)
	c2, _ := NewSecretCipher(hex.EncodeToString(otherKey))
	if _, err := c2.Decrypt(envelope); err == nil {
		t.Fatal("expected GCM auth failure with mismatched key")
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("plaintext") {
		t.Fatal("plaintext should not be reported as encrypted")
	}
	if !IsEncrypted(magicPrefix + "anything") {
		t.Fatal("magic-prefixed value should be reported as encrypted")
	}
}
