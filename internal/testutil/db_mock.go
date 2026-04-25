package testutil

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"

	"github.com/hamsaya/backend/pkg/database"
)

// MockPool is a testify/mock implementation of database.Pool.
type MockPool struct {
	mock.Mock
}

// Exec mocks pool.Exec. Match with On("Exec", ctx, sql, args) where args is []any.
func (m *MockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	ret := m.Called(ctx, sql, args)
	return ret.Get(0).(pgconn.CommandTag), ret.Error(1)
}

// Query mocks pool.Query. Match with On("Query", ctx, sql, args) where args is []any.
func (m *MockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	ret := m.Called(ctx, sql, args)
	if ret.Get(0) == nil {
		return nil, ret.Error(1)
	}
	return ret.Get(0).(pgx.Rows), ret.Error(1)
}

// QueryRow mocks pool.QueryRow. Match with On("QueryRow", ctx, sql, args) where args is []any.
func (m *MockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	ret := m.Called(ctx, sql, args)
	return ret.Get(0).(pgx.Row)
}

func (m *MockPool) Begin(ctx context.Context) (pgx.Tx, error) {
	ret := m.Called(ctx)
	if ret.Get(0) == nil {
		return nil, ret.Error(1)
	}
	return ret.Get(0).(pgx.Tx), ret.Error(1)
}

func (m *MockPool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	ret := m.Called(ctx, b)
	return ret.Get(0).(pgx.BatchResults)
}

func (m *MockPool) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockPool) Stat() *pgxpool.Stat {
	ret := m.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).(*pgxpool.Stat)
}

func (m *MockPool) Close() {
	m.Called()
}

// NewTestDB wraps a MockPool in a database.DB for use in repository tests.
func NewTestDB(pool *MockPool) *database.DB {
	return &database.DB{Pool: pool}
}

// MockRow implements pgx.Row for stubbing QueryRow results.
type MockRow struct {
	scanFn func(dest ...any) error
}

func NewMockRow(scanFn func(dest ...any) error) *MockRow {
	return &MockRow{scanFn: scanFn}
}

func (r *MockRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

// ErrRow returns a MockRow that always returns the given error from Scan.
func ErrRow(err error) *MockRow {
	return &MockRow{scanFn: func(dest ...any) error { return err }}
}

// MockRows implements pgx.Rows for stubbing Query results.
type MockRows struct {
	rows   [][]any
	idx    int
	closed bool
	err    error
}

func NewMockRows(rows [][]any) *MockRows {
	return &MockRows{rows: rows}
}

func (r *MockRows) Next() bool {
	r.idx++
	return r.idx <= len(r.rows)
}

func (r *MockRows) Scan(dest ...any) error {
	if r.idx < 1 || r.idx > len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		assignScanDest(d, row[i])
	}
	return nil
}

func (r *MockRows) Close()                                       { r.closed = true }
func (r *MockRows) Err() error                                   { return r.err }
func (r *MockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *MockRows) RawValues() [][]byte                          { return nil }
func (r *MockRows) Conn() *pgx.Conn                              { return nil }
func (r *MockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *MockRows) Values() ([]any, error)                       { return nil, nil }

// FuncRows implements pgx.Rows using per-row scan functions.
// Useful when columns include types that assignScanDest can't handle.
type FuncRows struct {
	scanFns []func(dest ...any) error
	idx     int
	err     error
}

// NewFuncRows creates a FuncRows where each entry in scanFns corresponds to one result row.
func NewFuncRows(scanFns ...func(dest ...any) error) *FuncRows {
	return &FuncRows{scanFns: scanFns}
}

// EmptyRows returns a FuncRows with no rows (simulates empty result set).
func EmptyRows() *FuncRows { return &FuncRows{} }

func (r *FuncRows) Next() bool {
	r.idx++
	return r.idx <= len(r.scanFns)
}

func (r *FuncRows) Scan(dest ...any) error {
	if r.idx < 1 || r.idx > len(r.scanFns) {
		return pgx.ErrNoRows
	}
	return r.scanFns[r.idx-1](dest...)
}

func (r *FuncRows) Close()                                       {}
func (r *FuncRows) Err() error                                   { return r.err }
func (r *FuncRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *FuncRows) RawValues() [][]byte                          { return nil }
func (r *FuncRows) Conn() *pgx.Conn                              { return nil }
func (r *FuncRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *FuncRows) Values() ([]any, error)                       { return nil, nil }

// MockBatchResults implements pgx.BatchResults for testing SendBatch flows.
type MockBatchResults struct {
	mock.Mock
	execErr error
}

// NewMockBatchResults returns a MockBatchResults where every Exec() call returns the given error.
func NewMockBatchResults(execErr error) *MockBatchResults {
	return &MockBatchResults{execErr: execErr}
}

func (m *MockBatchResults) Exec() (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 1"), m.execErr
}
func (m *MockBatchResults) Query() (pgx.Rows, error)  { return nil, nil }
func (m *MockBatchResults) QueryRow() pgx.Row          { return ErrRow(nil) }
func (m *MockBatchResults) Close() error               { return nil }

// AssignValue assigns src into a pointer dest using type switches for common types.
// Exported for use in repository test files.
func AssignValue(dest, src any) {
	assignScanDest(dest, src)
}

// assignScanDest assigns src into a pointer dest using type switches for common types.
func assignScanDest(dest, src any) {
	switch d := dest.(type) {
	case *string:
		if s, ok := src.(string); ok {
			*d = s
		}
	case **string:
		if src == nil {
			*d = nil
		} else if s, ok := src.(string); ok {
			*d = &s
		}
	case *int:
		if v, ok := src.(int); ok {
			*d = v
		}
	case *bool:
		if v, ok := src.(bool); ok {
			*d = v
		}
	}
}
