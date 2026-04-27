// Package secrets is a thin abstraction over secret-source backends.
//
// Two backends ship today:
//   - EnvSource: reads from process environment (default; current behavior).
//   - SSMSource: AWS Systems Manager Parameter Store. Lazy-resolves on
//     first Get; caches in-process for cacheTTL to avoid per-request
//     round trips.
//
// Selection: SECRETS_BACKEND=env|ssm. SSM additionally requires
// AWS_REGION + standard AWS credential resolution (IRSA on EKS, instance
// profile on EC2, ~/.aws/credentials on dev).
//
// Vault was considered and deferred — requires running Vault infra
// (multi-day deploy task). The interface is designed so a VaultSource
// can drop in later.

package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Source resolves secret values by key.
type Source interface {
	Get(ctx context.Context, key string) (string, error)
}

// EnvSource reads from process environment. The default backend.
type EnvSource struct{}

// Get returns os.Getenv(key). Empty string is allowed (caller decides).
func (EnvSource) Get(_ context.Context, key string) (string, error) {
	return os.Getenv(key), nil
}

// CachingSource decorates another Source with a TTL cache. Useful when
// the backing store charges per request (SSM) or has noticeable latency.
type CachingSource struct {
	upstream Source
	ttl      time.Duration
	mu       sync.RWMutex
	entries  map[string]cacheEntry
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// NewCaching wraps src with a TTL cache.
func NewCaching(src Source, ttl time.Duration) *CachingSource {
	return &CachingSource{
		upstream: src,
		ttl:      ttl,
		entries:  make(map[string]cacheEntry),
	}
}

// Get returns the cached value when fresh, otherwise refreshes from upstream.
func (c *CachingSource) Get(ctx context.Context, key string) (string, error) {
	c.mu.RLock()
	if e, ok := c.entries[key]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.value, nil
	}
	c.mu.RUnlock()

	value, err := c.upstream.Get(ctx, key)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.entries[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return value, nil
}

// FromEnvOrBackend chooses a Source based on SECRETS_BACKEND. Falls back
// to EnvSource when the env var is unset, "env", or unrecognized so a
// missing/typo'd value never silently disables secret resolution.
//
// Returns the chosen source and a human-readable label for boot logs.
func FromEnvOrBackend(ctx context.Context) (Source, string, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("SECRETS_BACKEND")))
	switch backend {
	case "", "env":
		return EnvSource{}, "env", nil
	case "ssm":
		src, err := NewSSMSource(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("secrets: ssm: %w", err)
		}
		return NewCaching(src, 5*time.Minute), "ssm (5m cache)", nil
	default:
		return nil, "", fmt.Errorf("secrets: unknown backend %q (env|ssm)", backend)
	}
}

// MustGet wraps Source.Get and panics on lookup error. For boot-time
// resolution where a missing secret is fatal anyway.
func MustGet(ctx context.Context, src Source, key string) string {
	v, err := src.Get(ctx, key)
	if err != nil {
		panic(fmt.Errorf("secrets: required key %q lookup failed: %w", key, err))
	}
	return v
}

// ErrSourceNotConfigured is returned when a backend impl is referenced
// but its dependencies are missing (e.g. SSM without AWS creds in env).
var ErrSourceNotConfigured = errors.New("secrets: backend not configured")
