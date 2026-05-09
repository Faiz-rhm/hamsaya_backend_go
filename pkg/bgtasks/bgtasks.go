// Package bgtasks provides a tiny worker pool for background fire-and-forget
// jobs that must outlive a request but still drain on graceful shutdown.
//
// Migration target for `go func() { … context.WithoutCancel(parent) … }()`
// patterns where the outer request context can't be inherited (would be
// cancelled when the HTTP handler returns) but unbounded leaked goroutines
// are also unacceptable.
//
// Usage:
//
//	pool := bgtasks.New(logger)
//	pool.Submit(func(ctx context.Context) { … })
//
//	// On shutdown:
//	pool.Shutdown(30 * time.Second)
package bgtasks

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Pool schedules background tasks against a single shared context that is
// cancelled on Shutdown. All in-flight tasks are awaited up to the shutdown
// timeout before Shutdown returns.
type Pool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
	closeMu sync.Mutex
	logger  *zap.Logger
}

// New constructs an idle pool. Pass nil logger to use the no-op logger.
func New(logger *zap.Logger) *Pool {
	if logger == nil {
		logger = zap.NewNop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{ctx: ctx, cancel: cancel, logger: logger}
}

// Submit schedules `task` on a fresh goroutine. After Shutdown is called
// further submissions are dropped (logged at warn level) so the caller does
// not need a separate gate.
func (p *Pool) Submit(task func(ctx context.Context)) {
	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		p.logger.Warn("bgtasks: submit after shutdown — dropped")
		return
	}
	p.wg.Add(1)
	p.closeMu.Unlock()

	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("bgtasks: task panicked", zap.Any("panic", r))
			}
		}()
		task(p.ctx)
	}()
}

// Shutdown cancels the shared context and waits up to `timeout` for all
// in-flight tasks to return. Returns true on clean drain, false on timeout.
func (p *Pool) Shutdown(timeout time.Duration) bool {
	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		return true
	}
	p.closed = true
	p.closeMu.Unlock()

	p.cancel()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		p.logger.Warn("bgtasks: shutdown timeout — some tasks did not drain",
			zap.Duration("timeout", timeout))
		return false
	}
}
