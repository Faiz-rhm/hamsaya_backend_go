package database

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// recordingTracer captures the order of TraceQueryStart / TraceQueryEnd
// calls. Used to assert that MultiTracer fans events out in declaration
// order and threads the ctx chain correctly.
type recordingTracer struct {
	name     string
	events   *[]string
	mu       *sync.Mutex
	ctxKey   interface{}
	ctxValue string
}

func (r *recordingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	*r.events = append(*r.events, r.name+":start")
	r.mu.Unlock()
	if r.ctxKey != nil {
		ctx = context.WithValue(ctx, r.ctxKey, r.ctxValue)
	}
	return ctx
}

func (r *recordingTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	r.mu.Lock()
	tag := r.name + ":end"
	if r.ctxKey != nil {
		if v, ok := ctx.Value(r.ctxKey).(string); ok {
			tag += "(saw:" + v + ")"
		}
	}
	*r.events = append(*r.events, tag)
	r.mu.Unlock()
}

func TestMultiTracer_PreservesOrder(t *testing.T) {
	var events []string
	var mu sync.Mutex
	a := &recordingTracer{name: "a", events: &events, mu: &mu}
	b := &recordingTracer{name: "b", events: &events, mu: &mu}

	mt := NewMultiTracer(a, b)
	ctx := mt.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})
	mt.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	assert.Equal(t, []string{"a:start", "b:start", "a:end", "b:end"}, events)
}

func TestMultiTracer_ContextChained(t *testing.T) {
	// Each tracer's TraceQueryStart receives the ctx returned by the
	// previous tracer, so values stored upstream are visible downstream.
	// Verify the returned ctx carries the upstream value.
	var events []string
	var mu sync.Mutex
	type aKey struct{}
	a := &recordingTracer{name: "a", events: &events, mu: &mu, ctxKey: aKey{}, ctxValue: "hello"}
	// b does not store anything (ctxKey nil so it skips WithValue) — its
	// only role is to verify ctx threads through unmodified.
	b := &recordingTracer{name: "b", events: &events, mu: &mu}

	mt := NewMultiTracer(a, b)
	finalCtx := mt.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})

	got, _ := finalCtx.Value(aKey{}).(string)
	assert.Equal(t, "hello", got, "downstream tracers must see upstream ctx values")
}

func TestMultiTracer_NilEntriesDropped(t *testing.T) {
	// Passing nil tracers shouldn't panic — they're silently filtered.
	mt := NewMultiTracer(nil, nil)
	assert.NotPanics(t, func() {
		ctx := mt.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})
		mt.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})
}

func TestMultiTracer_EmptyVarargs(t *testing.T) {
	mt := NewMultiTracer()
	assert.NotPanics(t, func() {
		ctx := mt.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})
		mt.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})
}
