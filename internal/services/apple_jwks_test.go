package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// jwksTestKey holds a generated RSA key + its kid for use across tests.
type jwksTestKey struct {
	kid  string
	key  *rsa.PrivateKey
	n, e string
}

func generateJWKSTestKey(t *testing.T, kid string) *jwksTestKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	nB64 := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
	eB64 := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes())
	return &jwksTestKey{kid: kid, key: priv, n: nB64, e: eB64}
}

// newJWKSServer spins up an httptest.Server that responds to GET requests
// with a JWKS document containing the supplied keys.
func newJWKSServer(t *testing.T, keys ...*jwksTestKey) *httptest.Server {
	t.Helper()
	jwks := appleJWKS{Keys: make([]appleJWK, 0, len(keys))}
	for _, k := range keys {
		jwks.Keys = append(jwks.Keys, appleJWK{
			Kty: "RSA",
			Kid: k.kid,
			Use: "sig",
			Alg: "RS256",
			N:   k.n,
			E:   k.e,
		})
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

// signAppleToken builds a signed JWT mimicking Apple's identity_token.
func signAppleToken(t *testing.T, k *jwksTestKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = k.kid
	signed, err := tok.SignedString(k.key)
	require.NoError(t, err)
	return signed
}

func newAppleTestService(t *testing.T, jwksURL string) *OAuthService {
	t.Helper()
	cfg := &config.Config{
		OAuth: config.OAuthConfig{
			Apple: config.AppleOAuthConfig{ClientID: "af.hamsaya"},
		},
	}
	svc := NewOAuthService(cfg, new(mocks.MockUserRepository), zap.NewNop())
	svc.appleKeys.url = jwksURL
	return svc
}

func TestOAuthService_VerifyAppleToken_Success(t *testing.T) {
	k := generateJWKSTestKey(t, "kid-1")
	srv := newJWKSServer(t, k)
	defer srv.Close()
	svc := newAppleTestService(t, srv.URL)

	tokenStr := signAppleToken(t, k, jwt.MapClaims{
		"iss":            "https://appleid.apple.com",
		"aud":            "af.hamsaya",
		"sub":            "001234.abcd5678",
		"email":          "user@example.com",
		"email_verified": "true",
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
		"iat":            time.Now().Unix(),
	})

	info, err := svc.VerifyAppleToken(context.Background(), tokenStr)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "001234.abcd5678", info.ProviderUserID)
	assert.Equal(t, "user@example.com", info.Email)
	assert.True(t, info.EmailVerified)
	assert.Equal(t, "apple", info.Provider)
}

func TestOAuthService_VerifyAppleToken_WrongAudience(t *testing.T) {
	k := generateJWKSTestKey(t, "kid-1")
	srv := newJWKSServer(t, k)
	defer srv.Close()
	svc := newAppleTestService(t, srv.URL)

	tokenStr := signAppleToken(t, k, jwt.MapClaims{
		"iss": "https://appleid.apple.com",
		"aud": "com.someone.else",
		"sub": "001234.abcd5678",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	_, err := svc.VerifyAppleToken(context.Background(), tokenStr)
	require.Error(t, err)
}

func TestOAuthService_VerifyAppleToken_Expired(t *testing.T) {
	k := generateJWKSTestKey(t, "kid-1")
	srv := newJWKSServer(t, k)
	defer srv.Close()
	svc := newAppleTestService(t, srv.URL)

	tokenStr := signAppleToken(t, k, jwt.MapClaims{
		"iss": "https://appleid.apple.com",
		"aud": "af.hamsaya",
		"sub": "001234.abcd5678",
		"exp": time.Now().Add(-5 * time.Minute).Unix(),
	})

	_, err := svc.VerifyAppleToken(context.Background(), tokenStr)
	require.Error(t, err)
}

func TestOAuthService_VerifyAppleToken_UnknownKid(t *testing.T) {
	serverKey := generateJWKSTestKey(t, "kid-server")
	signerKey := generateJWKSTestKey(t, "kid-rotated") // different from what server publishes
	srv := newJWKSServer(t, serverKey)
	defer srv.Close()
	svc := newAppleTestService(t, srv.URL)

	tokenStr := signAppleToken(t, signerKey, jwt.MapClaims{
		"iss": "https://appleid.apple.com",
		"aud": "af.hamsaya",
		"sub": "001234.abcd5678",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	_, err := svc.VerifyAppleToken(context.Background(), tokenStr)
	require.Error(t, err)
}

func TestAppleKeyCache_RefreshAndCache(t *testing.T) {
	k := generateJWKSTestKey(t, "kid-1")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		jwks := appleJWKS{Keys: []appleJWK{{Kty: "RSA", Kid: k.kid, N: k.n, E: k.e, Alg: "RS256"}}}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	cache := &appleKeyCache{
		httpc: srv.Client(),
		url:   srv.URL,
		ttl:   time.Hour,
	}

	// First call hits the network.
	_, err := cache.publicKey(context.Background(), "kid-1")
	require.NoError(t, err)
	assert.Equal(t, 1, hits)

	// Second call within TTL is served from cache.
	_, err = cache.publicKey(context.Background(), "kid-1")
	require.NoError(t, err)
	assert.Equal(t, 1, hits, "cache hit expected")

	// Force expiry → next call refreshes.
	cache.mu.Lock()
	cache.fetchedAt = time.Now().Add(-2 * time.Hour)
	cache.mu.Unlock()
	_, err = cache.publicKey(context.Background(), "kid-1")
	require.NoError(t, err)
	assert.Equal(t, 2, hits)
}
