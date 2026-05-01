package transcode

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type fakeEncoder struct {
	calls     atomic.Int32
	failTimes int32 // first N calls fail with err
	err       error
}

func (f *fakeEncoder) Transcode(_ context.Context, _, _, _ string, _ int) error {
	n := f.calls.Add(1)
	if n <= f.failTimes {
		return f.err
	}
	return nil
}

func newPool(t *testing.T, encoder Encoder, concurrency int) (*Pool, *Queue) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := NewQueue(client, "")
	q.blockTTL = 100 * time.Millisecond
	pool := NewPool(q, encoder, zap.NewNop(), concurrency)
	return pool, q
}

func TestWorker_DrainsQueue(t *testing.T) {
	enc := &fakeEncoder{}
	pool, q := newPool(t, enc, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		_ = q.Enqueue(ctx, &Job{ID: "j", SourceKey: "s", TargetKey: "t", Format: "webp"})
	}

	done := make(chan struct{})
	go func() { pool.Run(ctx); close(done) }()

	deadline := time.After(1500 * time.Millisecond)
	for enc.calls.Load() < 5 {
		select {
		case <-deadline:
			t.Fatalf("only %d/5 calls observed", enc.calls.Load())
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestWorker_RetriesTransientFailure(t *testing.T) {
	// First 2 calls fail; 3rd succeeds. Single attempt = success after retries.
	enc := &fakeEncoder{failTimes: 2, err: errors.New("transient")}
	pool, q := newPool(t, enc, 1)
	pool.maxAttempts = 3
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = q.Enqueue(ctx, &Job{ID: "j", Format: "webp"})

	done := make(chan struct{})
	go func() { pool.Run(ctx); close(done) }()

	deadline := time.After(20 * time.Second) // 2s + 4s backoff
	for enc.calls.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("expected 3 calls (2 fail + 1 ok), saw %d", enc.calls.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestWorker_DeadLettersAfterMaxAttempts(t *testing.T) {
	enc := &fakeEncoder{failTimes: 99, err: errors.New("permanent")}
	pool, q := newPool(t, enc, 1)
	pool.maxAttempts = 2
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = q.Enqueue(ctx, &Job{ID: "j", Format: "webp"})

	done := make(chan struct{})
	go func() { pool.Run(ctx); close(done) }()

	deadline := time.After(15 * time.Second)
	for enc.calls.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("expected 2 attempts before dead-letter, saw %d", enc.calls.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}
	// allow dead-letter write to land
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	deadCount, err := q.client.LLen(context.Background(), q.deadKey).Result()
	if err != nil {
		t.Fatal(err)
	}
	if deadCount != 1 {
		t.Fatalf("dead-letter count = %d, want 1", deadCount)
	}
}
