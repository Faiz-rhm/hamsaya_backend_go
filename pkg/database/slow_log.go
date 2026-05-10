package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// SlowQueryTracer logs every SQL query whose execution time exceeds the
// configured threshold. Implements [pgx.QueryTracer] so it plugs in via
// `poolConfig.ConnConfig.Tracer = &SlowQueryTracer{...}`.
//
// Threshold defaults to 200ms when zero. Queries below the threshold are not
// logged at all so this stays cheap on hot paths.
type SlowQueryTracer struct {
	Logger    *zap.Logger
	Threshold time.Duration
}

type slowQueryStartCtxKey struct{}

type slowQueryStartCtx struct {
	startedAt time.Time
	sql       string
}

// TraceQueryStart is called when a query starts.
func (t *SlowQueryTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	return context.WithValue(ctx, slowQueryStartCtxKey{}, &slowQueryStartCtx{
		startedAt: time.Now(),
		sql:       data.SQL,
	})
}

// TraceQueryEnd is called when a query completes.
func (t *SlowQueryTracer) TraceQueryEnd(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryEndData,
) {
	start, ok := ctx.Value(slowQueryStartCtxKey{}).(*slowQueryStartCtx)
	if !ok || start == nil {
		return
	}
	threshold := t.Threshold
	if threshold <= 0 {
		threshold = 200 * time.Millisecond
	}
	elapsed := time.Since(start.startedAt)
	if elapsed < threshold {
		return
	}
	if t.Logger == nil {
		return
	}
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Stringp("sql", &start.sql),
	}
	if data.Err != nil {
		fields = append(fields, zap.Error(data.Err))
	}
	t.Logger.Warn("slow query", fields...)
}
