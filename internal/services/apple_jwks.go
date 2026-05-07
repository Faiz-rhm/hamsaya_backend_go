package services

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// appleJWKSURL is the public endpoint exposing the rotating set of public
// keys Apple uses to sign Sign-In-with-Apple identity tokens.
const appleJWKSURL = "https://appleid.apple.com/auth/keys"

// appleJWKSCacheTTL is how long fetched keys stay in memory before a refresh
// is forced. Apple rotates keys infrequently; 1 hour is a safe balance
// between freshness and avoiding the keys endpoint on every login.
const appleJWKSCacheTTL = time.Hour

// appleJWK is one entry from Apple's JWKS endpoint.
type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type appleJWKS struct {
	Keys []appleJWK `json:"keys"`
}

// appleKeyCache fetches and caches Apple's public keys. Safe for concurrent
// use; refreshes on demand once the TTL elapses.
type appleKeyCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	httpc     *http.Client
}

// global singleton — there's no per-request state worth duplicating.
var appleKeys = &appleKeyCache{
	httpc: &http.Client{Timeout: 5 * time.Second},
}

// publicKey returns the RSA public key for the given kid, refreshing the
// cache from Apple if needed.
func (c *appleKeyCache) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if k, ok := c.keys[kid]; ok && time.Since(c.fetchedAt) < appleJWKSCacheTTL {
		c.mu.RUnlock()
		return k, nil
	}
	c.mu.RUnlock()

	if err := c.refresh(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	k, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("apple jwks: kid %q not found", kid)
	}
	return k, nil
}

func (c *appleKeyCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appleJWKSURL, nil)
	if err != nil {
		return fmt.Errorf("apple jwks request: %w", err)
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("apple jwks fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apple jwks status %d", resp.StatusCode)
	}

	var jwks appleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("apple jwks decode: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("apple jwks: no usable keys")
	}

	c.mu.Lock()
	c.keys = keys
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return nil
}

// rsaPublicKeyFromJWK reconstructs an RSA public key from the base64url-
// encoded modulus (n) and exponent (e) supplied by a JWK.
func rsaPublicKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	// Exponent fits in an int — Apple uses 65537.
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}
