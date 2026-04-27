// Package transcode is a Redis-backed job queue for asynchronous image
// transcoding (currently only WebP encode). Foundation only — upload
// handlers still encode synchronously today. To migrate:
//
//   1. Upload handler stores the original image to MinIO under a
//      "pending" key, returns the URL immediately.
//   2. Handler calls Queue.Enqueue with the source key + target format.
//   3. A pool of [Worker]s consumes jobs, fetches the source, encodes,
//      writes the encoded variant under the canonical key, deletes
//      the pending object.
//   4. Mobile clients retry-on-404 with exponential backoff (or use
//      Cache-Control: no-cache for the first ~60s after upload).
//
// The queue uses LPUSH + BRPOP so it survives worker restarts and provides
// at-least-once delivery. Failed jobs land in a dead-letter list keyed by
// "<queueKey>:dead" with the original payload + error string.

package transcode

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultQueueKey   = "transcode:webp"
	defaultBlockTimeout = 5 * time.Second
)

// Job describes a unit of transcoding work.
type Job struct {
	ID         string `json:"id"`
	SourceKey  string `json:"source_key"`  // MinIO key of the original
	TargetKey  string `json:"target_key"`  // MinIO key for the encoded output
	Format     string `json:"format"`      // "webp" for now
	Quality    int    `json:"quality"`
	EnqueuedAt int64  `json:"enqueued_at"`
}

// Queue is a Redis-backed FIFO of [Job].
type Queue struct {
	client   *redis.Client
	key      string
	deadKey  string
	blockTTL time.Duration
}

// NewQueue constructs a queue at queueKey (defaults to "transcode:webp").
func NewQueue(client *redis.Client, queueKey string) *Queue {
	if queueKey == "" {
		queueKey = defaultQueueKey
	}
	return &Queue{
		client:   client,
		key:      queueKey,
		deadKey:  queueKey + ":dead",
		blockTTL: defaultBlockTimeout,
	}
}

// Enqueue adds a job to the tail of the queue.
func (q *Queue) Enqueue(ctx context.Context, job *Job) error {
	if job.EnqueuedAt == 0 {
		job.EnqueuedAt = time.Now().Unix()
	}
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("transcode: marshal: %w", err)
	}
	if err := q.client.LPush(ctx, q.key, body).Err(); err != nil {
		return fmt.Errorf("transcode: lpush: %w", err)
	}
	return nil
}

// Dequeue blocks for up to blockTTL waiting for a job. Returns (nil, nil)
// on idle timeout so callers can re-check ctx and loop.
func (q *Queue) Dequeue(ctx context.Context) (*Job, error) {
	res, err := q.client.BRPop(ctx, q.blockTTL, q.key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("transcode: brpop: %w", err)
	}
	if len(res) < 2 {
		return nil, nil
	}
	var job Job
	if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
		return nil, fmt.Errorf("transcode: unmarshal: %w", err)
	}
	return &job, nil
}

// DeadLetter records a permanently failed job for inspection. Capped at
// 1000 entries (LPUSH + LTRIM) so it doesn't grow unbounded.
func (q *Queue) DeadLetter(ctx context.Context, job *Job, reason string) error {
	body, err := json.Marshal(struct {
		*Job
		Reason   string `json:"reason"`
		FailedAt int64  `json:"failed_at"`
	}{Job: job, Reason: reason, FailedAt: time.Now().Unix()})
	if err != nil {
		return fmt.Errorf("transcode: marshal dead: %w", err)
	}
	pipe := q.client.Pipeline()
	pipe.LPush(ctx, q.deadKey, body)
	pipe.LTrim(ctx, q.deadKey, 0, 999)
	_, err = pipe.Exec(ctx)
	return err
}

// PendingCount returns the number of jobs waiting to be processed.
// Diagnostics only.
func (q *Queue) PendingCount(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.key).Result()
}
