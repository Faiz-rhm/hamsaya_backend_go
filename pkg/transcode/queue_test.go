package transcode

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestQueue(t *testing.T) (*Queue, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := NewQueue(client, "")
	q.blockTTL = 100 * time.Millisecond // shorten for tests
	return q, mr, client
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	q, _, _ := newTestQueue(t)
	ctx := context.Background()

	job := &Job{ID: "j1", SourceKey: "src", TargetKey: "tgt", Format: "webp", Quality: 85}
	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatal(err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil || got == nil {
		t.Fatalf("dequeue: %v %v", got, err)
	}
	if got.ID != "j1" || got.SourceKey != "src" || got.Quality != 85 {
		t.Fatalf("decoded mismatch: %+v", got)
	}
	if got.EnqueuedAt == 0 {
		t.Fatal("Enqueue should stamp EnqueuedAt")
	}
}

func TestQueue_DequeueIdleTimesOut(t *testing.T) {
	q, _, _ := newTestQueue(t)
	ctx := context.Background()

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("idle should not error: %v", err)
	}
	if got != nil {
		t.Fatalf("idle should return nil, got %+v", got)
	}
}

func TestQueue_DeadLetterCappedAt1000(t *testing.T) {
	q, _, client := newTestQueue(t)
	ctx := context.Background()

	for i := 0; i < 1100; i++ {
		if err := q.DeadLetter(ctx, &Job{ID: "x"}, "boom"); err != nil {
			t.Fatal(err)
		}
	}

	n, err := client.LLen(ctx, q.deadKey).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1000 {
		t.Fatalf("dead-letter cap: got %d, want 1000", n)
	}
}

func TestQueue_FIFOOrder(t *testing.T) {
	q, _, _ := newTestQueue(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c"} {
		_ = q.Enqueue(ctx, &Job{ID: id})
	}

	for _, want := range []string{"a", "b", "c"} {
		got, err := q.Dequeue(ctx)
		if err != nil || got == nil {
			t.Fatalf("dequeue: %v %v", got, err)
		}
		if got.ID != want {
			t.Fatalf("FIFO order: got %s want %s", got.ID, want)
		}
	}
}

func TestQueue_PendingCount(t *testing.T) {
	q, _, _ := newTestQueue(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = q.Enqueue(ctx, &Job{ID: "x"})
	}
	n, err := q.PendingCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("pending count: got %d want 3", n)
	}
}
