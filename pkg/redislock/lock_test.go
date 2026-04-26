package redislock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return c, mr
}

func TestAcquire_Succeeds_WhenKeyFree(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()
	lock, err := Acquire(ctx, c, "lock:job:test", time.Minute)
	if err != nil {
		t.Fatalf("Acquire: unexpected error: %v", err)
	}
	if lock == nil {
		t.Fatal("Acquire: returned nil lock")
	}
	if err := lock.Release(ctx); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestAcquire_Contention_SecondCallerFails(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	first, err := Acquire(ctx, c, "lock:job:contend", time.Minute)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer first.Release(ctx)

	_, err = Acquire(ctx, c, "lock:job:contend", time.Minute)
	if !errors.Is(err, ErrNotAcquired) {
		t.Fatalf("expected ErrNotAcquired on contention, got %v", err)
	}
}

func TestAcquire_AfterTTLExpiry_AnotherCallerWins(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	first, err := Acquire(ctx, c, "lock:job:ttl", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	_ = first // intentionally do not release; let it expire

	mr.FastForward(200 * time.Millisecond)

	second, err := Acquire(ctx, c, "lock:job:ttl", time.Minute)
	if err != nil {
		t.Fatalf("post-expiry Acquire: %v", err)
	}
	if second == nil {
		t.Fatal("expected non-nil lock after expiry")
	}
}

func TestRelease_DoesNotDeleteOtherHoldersToken(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	first, err := Acquire(ctx, c, "lock:job:steal", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	// First lock expires, second caller acquires the same key.
	mr.FastForward(200 * time.Millisecond)
	second, err := Acquire(ctx, c, "lock:job:steal", time.Minute)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}

	// First instance attempting to release must NOT delete the second holder's lock.
	if err := first.Release(ctx); err != nil {
		t.Fatalf("Release on stale lock returned error: %v", err)
	}
	if val, err := c.Get(ctx, "lock:job:steal").Result(); err != nil {
		t.Fatalf("Get after stale release: %v (val=%q)", err, val)
	} else if val != second.token {
		t.Fatalf("stale Release deleted second holder's token: have %q, want %q", val, second.token)
	}
}

// TestAcquire_ConcurrentCallers asserts that under high concurrency, exactly
// one caller wins and the rest receive ErrNotAcquired — protects the leader
// election guarantee for background jobs.
func TestAcquire_ConcurrentCallers(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	const goroutines = 50
	var (
		wg      sync.WaitGroup
		winners int
		mu      sync.Mutex
	)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := Acquire(ctx, c, "lock:job:concurrent", time.Minute)
			if err == nil && lock != nil {
				mu.Lock()
				winners++
				mu.Unlock()
			} else if !errors.Is(err, ErrNotAcquired) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if winners != 1 {
		t.Fatalf("expected exactly 1 winner across %d goroutines, got %d", goroutines, winners)
	}
}
