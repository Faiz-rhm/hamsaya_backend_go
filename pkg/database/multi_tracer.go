package database

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// MultiTracer fans out pgx tracer events to multiple downstream tracers.
// Useful when you want both a slow-query log AND metrics — each does its
// own thing without coupling.
//
// Order matters for TraceQueryStart: each tracer can return a derived
// context, and the next tracer in the chain receives that derived ctx.
// TraceQueryEnd uses the same chained ctx so tracers can read keys that
// earlier tracers stored.
type MultiTracer struct {
	tracers []pgx.QueryTracer
}

// NewMultiTracer returns a tracer that delegates to every non-nil tracer
// in the input order. Nil entries are silently dropped so callers can
// pass optional tracers without nil-checks.
func NewMultiTracer(tracers ...pgx.QueryTracer) *MultiTracer {
	out := make([]pgx.QueryTracer, 0, len(tracers))
	for _, t := range tracers {
		if t != nil {
			out = append(out, t)
		}
	}
	return &MultiTracer{tracers: out}
}

// TraceQueryStart chains every tracer's start hook. Each tracer's returned
// ctx becomes the input for the next, so context keys accumulate.
func (m *MultiTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	for _, t := range m.tracers {
		ctx = t.TraceQueryStart(ctx, conn, data)
	}
	return ctx
}

// TraceQueryEnd invokes every tracer's end hook in the same order. Errors
// from earlier tracers don't short-circuit later ones — each tracer needs
// to see the end event to clean up its own per-query state.
func (m *MultiTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	for _, t := range m.tracers {
		t.TraceQueryEnd(ctx, conn, data)
	}
}
