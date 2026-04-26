// Package redislock provides a lightweight Redis-backed mutex for cross-instance
// leader election of periodic background jobs.
package redislock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotAcquired = errors.New("redislock: lock not acquired")

// releaseScript ensures only the holder of the lock can release it.
const releaseScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`

// Lock represents an acquired Redis lock.
type Lock struct {
	client *redis.Client
	key    string
	token  string
}

// Acquire tries to take ownership of the named lock for the given TTL.
// Returns ErrNotAcquired if another instance currently holds it.
func Acquire(ctx context.Context, client *redis.Client, key string, ttl time.Duration) (*Lock, error) {
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	ok, err := client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotAcquired
	}
	return &Lock{client: client, key: key, token: token}, nil
}

// Release deletes the lock if and only if this Lock instance still owns it.
// Safe to call on an expired lock — returns nil in that case.
func (l *Lock) Release(ctx context.Context) error {
	if l == nil {
		return nil
	}
	_, err := l.client.Eval(ctx, releaseScript, []string{l.key}, l.token).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}

func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
