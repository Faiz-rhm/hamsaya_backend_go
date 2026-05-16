// Package cache wraps Redis with a tiny, JSON-serialised key/value layer
// for read-through caching of expensive Postgres queries.
//
// Design goals:
//
//   - No-op safe: when Redis is unavailable or the client is nil, every
//     operation degrades to a cache miss. Callers always run the loader
//     so cache outages never block requests.
//   - Namespace by service: each consumer passes a prefix (e.g.
//     "categories:") so flushing one namespace doesn't nuke unrelated keys.
//   - JSON values: keep the wire format dumb. Don't reach for gob / protobuf
//     until you actually have a perf reason — Redis is rarely the bottleneck.
//   - Bounded TTLs: callers MUST pass a TTL. No infinite caching — that
//     way a forgotten bust still self-heals on the next clock tick.
//
// Use the helpers via the constructor and pass the resulting *Cache into
// service constructors (e.g. CategoryService.WithCache(c)). Services keep
// the *Cache optional so unit tests don't need Redis.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache is a thin wrapper around *redis.Client scoped to one namespace.
//
// Operations log on error but never propagate — a Redis outage becomes a
// cache miss, not a 500.
type Cache struct {
	rdb       *redis.Client
	namespace string
	logger    *zap.Logger
}

// New returns a Cache that prefixes every key with namespace + ":" (e.g.
// New(rdb, "categories", log) and Set(ctx, "active:en") writes to key
// "categories:active:en"). namespace must be non-empty in production —
// the empty string is allowed for tests / migration scripts but logs a
// warning.
func New(rdb *redis.Client, namespace string, logger *zap.Logger) *Cache {
	if logger == nil {
		logger = zap.NewNop()
	}
	if namespace == "" {
		logger.Warn("cache: empty namespace — keys will collide across services")
	}
	return &Cache{rdb: rdb, namespace: namespace, logger: logger}
}

// keyFor builds the namespaced redis key.
func (c *Cache) keyFor(key string) string {
	if c.namespace == "" {
		return key
	}
	return c.namespace + ":" + key
}

// disabled returns true when the cache is a no-op (nil client or nil receiver).
func (c *Cache) disabled() bool {
	return c == nil || c.rdb == nil
}

// Get looks up a value and JSON-decodes it into out. Returns (found, err).
// A clean cache miss is (false, nil) — callers should run the loader and
// store the fresh value via Set.
func (c *Cache) Get(ctx context.Context, key string, out interface{}) (bool, error) {
	if c.disabled() {
		return false, nil
	}
	raw, err := c.rdb.Get(ctx, c.keyFor(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		c.logger.Warn("cache: get failed", zap.String("key", key), zap.Error(err))
		return false, nil // soft-fail: treat as miss
	}
	if err := json.Unmarshal(raw, out); err != nil {
		// Corrupt entry — log and pretend it's a miss. Next write fixes it.
		c.logger.Warn("cache: unmarshal failed",
			zap.String("key", key), zap.Error(err))
		_ = c.rdb.Del(ctx, c.keyFor(key)).Err()
		return false, nil
	}
	return true, nil
}

// Set serialises v as JSON and stores it under key with the given TTL.
// TTL must be > 0; zero or negative values are clamped to 60s so a typo
// never produces an infinite cache.
func (c *Cache) Set(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	if c.disabled() {
		return nil
	}
	if ttl <= 0 {
		c.logger.Warn("cache: non-positive TTL clamped to 60s", zap.String("key", key))
		ttl = 60 * time.Second
	}
	raw, err := json.Marshal(v)
	if err != nil {
		c.logger.Warn("cache: marshal failed", zap.String("key", key), zap.Error(err))
		return err
	}
	if err := c.rdb.Set(ctx, c.keyFor(key), raw, ttl).Err(); err != nil {
		c.logger.Warn("cache: set failed", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// Del removes one or more keys from the namespace. Errors are logged and
// swallowed — failing to evict is non-fatal (TTL will eventually expire).
func (c *Cache) Del(ctx context.Context, keys ...string) {
	if c.disabled() || len(keys) == 0 {
		return
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = c.keyFor(k)
	}
	if err := c.rdb.Del(ctx, full...).Err(); err != nil {
		c.logger.Warn("cache: del failed", zap.Strings("keys", keys), zap.Error(err))
	}
}

// DelPattern evicts every key matching `<namespace>:<pattern>` (Redis glob
// syntax — typically "<prefix>:*"). Use sparingly: SCAN-based delete on
// large keyspaces can be slow. Failures are logged + swallowed.
func (c *Cache) DelPattern(ctx context.Context, pattern string) {
	if c.disabled() {
		return
	}
	full := c.keyFor(pattern)
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, full, 200).Result()
		if err != nil {
			c.logger.Warn("cache: scan failed",
				zap.String("pattern", pattern), zap.Error(err))
			return
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				c.logger.Warn("cache: del-pattern failed",
					zap.String("pattern", pattern), zap.Error(err))
			}
		}
		if next == 0 {
			return
		}
		cursor = next
	}
}
