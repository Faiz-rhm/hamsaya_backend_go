package observability

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// pgxTracer implements pgx/v5's QueryTracer. Every query that flows through
// a pgxpool with this tracer attached emits db_query_duration_seconds and
// db_queries_total via the global Metrics handle. Wired in main.go via
// database.NewWithTracer(cfg, observability.NewPGXTracer()).
//
// The tracer is intentionally allocation-light:
//
//   - Stores nothing on the connection. Per-query start time lives in
//     ctx via a private key so concurrent queries on the same conn don't
//     stomp each other.
//   - Coarse `operation` label (SELECT / INSERT / UPDATE / DELETE / OTHER)
//     and best-effort `table` extraction. Full SQL text is never emitted
//     as a metric label — that would explode Prometheus cardinality.
type pgxTracer struct{}

// NewPGXTracer returns a pgx.QueryTracer wired to the global metrics
// handle. Safe to use even when SetGlobal hasn't been called — the
// metric emitters fail open.
func NewPGXTracer() pgx.QueryTracer {
	return &pgxTracer{}
}

type pgxTracerKey struct{}

// queryStartInfo carries per-query timing across TraceQueryStart →
// TraceQueryEnd. Pulled from ctx so concurrent queries don't collide.
type queryStartInfo struct {
	start     time.Time
	operation string
	table     string
}

// TraceQueryStart captures the query start time and a coarse classification
// of the SQL. Called by pgx on every query.
func (t *pgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	op, table := classifySQL(data.SQL)
	return context.WithValue(ctx, pgxTracerKey{}, queryStartInfo{
		start:     time.Now(),
		operation: op,
		table:     table,
	})
}

// TraceQueryEnd records the query metrics. Errors flow through as a
// success=false tag so dashboards can split failure rate from total rate.
func (t *pgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	info, ok := ctx.Value(pgxTracerKey{}).(queryStartInfo)
	if !ok {
		return // start hook never ran — defensive, shouldn't happen
	}
	m := loadGlobal()
	if m == nil {
		return
	}
	m.RecordDBQuery(ctx, info.operation, info.table, time.Since(info.start), data.Err == nil)
}

// classifySQL extracts a coarse (operation, table) tuple from raw SQL.
// Heuristic-only — never aim for parser-grade accuracy. The goal is
// keeping Prometheus label cardinality bounded: <5 operations, ~30
// tables. SQL that doesn't match common shapes lands in ("OTHER", "").
func classifySQL(sql string) (operation, table string) {
	s := strings.TrimSpace(sql)
	if s == "" {
		return "OTHER", ""
	}
	upper := strings.ToUpper(s)
	switch {
	case strings.HasPrefix(upper, "SELECT"):
		operation = "SELECT"
		table = extractFromTable(upper, " FROM ")
	case strings.HasPrefix(upper, "INSERT INTO"):
		operation = "INSERT"
		table = firstToken(strings.TrimPrefix(upper, "INSERT INTO "))
	case strings.HasPrefix(upper, "UPDATE"):
		operation = "UPDATE"
		table = firstToken(strings.TrimPrefix(upper, "UPDATE "))
	case strings.HasPrefix(upper, "DELETE FROM"):
		operation = "DELETE"
		table = firstToken(strings.TrimPrefix(upper, "DELETE FROM "))
	case strings.HasPrefix(upper, "WITH "):
		// CTE — peek for the first SELECT/INSERT/UPDATE keyword.
		operation = "CTE"
	case strings.HasPrefix(upper, "BEGIN"),
		strings.HasPrefix(upper, "COMMIT"),
		strings.HasPrefix(upper, "ROLLBACK"):
		operation = "TX"
	default:
		operation = "OTHER"
	}
	return operation, table
}

// extractFromTable returns the table name immediately following the
// first occurrence of `marker` (e.g. " FROM "). Returns "" if not found.
func extractFromTable(upper, marker string) string {
	idx := strings.Index(upper, marker)
	if idx < 0 {
		return ""
	}
	return firstToken(upper[idx+len(marker):])
}

// firstToken returns the leading identifier (letters, digits, underscore,
// dot) from s, stripping quotes. Used to recover table names from
// substrings of upper-cased SQL.
func firstToken(s string) string {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) {
		c := s[end]
		isIdent := (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '.'
		if !isIdent {
			break
		}
		end++
	}
	return strings.ToLower(strings.Trim(s[:end], "\"'"))
}
