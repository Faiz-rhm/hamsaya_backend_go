// Worker pool that drains [Queue]. Each worker fetches the source object
// from storage, encodes WebP, writes the result, and deletes the pending
// original. Failures retry with exponential backoff up to maxAttempts;
// permanent failures land in the dead-letter list.

package transcode

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Encoder is implemented by pkg/storage to perform the actual transcode.
// Kept minimal so the worker package doesn't depend on the storage package
// (avoids import cycles).
type Encoder interface {
	// Transcode fetches sourceKey, encodes to format/quality, writes to
	// targetKey, and deletes sourceKey on success.
	Transcode(ctx context.Context, sourceKey, targetKey, format string, quality int) error
}

// Pool runs N workers consuming from a Queue.
type Pool struct {
	queue       *Queue
	encoder     Encoder
	logger      *zap.Logger
	concurrency int
	maxAttempts int
}

// NewPool creates a worker pool. concurrency=4 is a reasonable default for
// CPU-bound WebP encoding on a 4-core node; tune via env.
func NewPool(queue *Queue, encoder Encoder, logger *zap.Logger, concurrency int) *Pool {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Pool{
		queue:       queue,
		encoder:     encoder,
		logger:      logger,
		concurrency: concurrency,
		maxAttempts: 3,
	}
}

// Run blocks until ctx is cancelled. Workers drain the queue concurrently;
// each worker has its own backoff state for transient failures.
func (p *Pool) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			p.workerLoop(ctx, id)
		}(i)
	}
	wg.Wait()
	p.logger.Info("transcode pool shut down")
}

func (p *Pool) workerLoop(ctx context.Context, workerID int) {
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := p.queue.Dequeue(ctx)
		if err != nil {
			p.logger.Warn("transcode dequeue", zap.Int("worker", workerID), zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		if job == nil {
			continue // idle timeout
		}
		p.process(ctx, workerID, job)
	}
}

func (p *Pool) process(ctx context.Context, workerID int, job *Job) {
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		err := p.encoder.Transcode(ctx, job.SourceKey, job.TargetKey, job.Format, job.Quality)
		if err == nil {
			p.logger.Info("transcode ok",
				zap.Int("worker", workerID),
				zap.String("id", job.ID),
				zap.Int("attempt", attempt),
			)
			return
		}
		if attempt >= p.maxAttempts {
			p.logger.Error("transcode dead",
				zap.Int("worker", workerID),
				zap.String("id", job.ID),
				zap.Error(err),
			)
			_ = p.queue.DeadLetter(ctx, job, err.Error())
			return
		}
		backoff := time.Duration(attempt) * 2 * time.Second
		p.logger.Warn("transcode retry",
			zap.Int("worker", workerID),
			zap.String("id", job.ID),
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}
