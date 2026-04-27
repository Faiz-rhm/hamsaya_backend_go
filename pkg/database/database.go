package database

import (
	"context"
	"fmt"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the interface for database pool operations, satisfied by *pgxpool.Pool.
type Pool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	Ping(ctx context.Context) error
	Stat() *pgxpool.Stat
	Close()
}

// DB holds the database connection pools.
//
// Pool is the canonical writer pool (every write + every read by default).
// ReplicaPool, when non-nil, is an additional read-only pool routed to
// a Postgres read replica for hot read paths (feed, search, profile).
// Repository methods opt in by reading from [DB.Reader] which falls back
// to the writer when no replica is configured.
type DB struct {
	Pool        Pool
	ReplicaPool Pool
}

// Reader returns the pool that should be used for read-only queries.
// Falls back to the writer when DB_REPLICA_HOST is unset.
func (db *DB) Reader() Pool {
	if db.ReplicaPool != nil {
		return db.ReplicaPool
	}
	return db.Pool
}

// New creates a new database connection. When cfg.ReplicaHost is non-empty,
// also opens a separate replica pool. Replica failures are non-fatal —
// the system degrades to writer-only and logs a warning via the caller.
func New(cfg *config.DatabaseConfig) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build connection string
	dsn := cfg.GetDSN()

	// Configure connection pool
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Set pool configuration
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	db := &DB{Pool: pool}

	// Open optional read-replica pool. We tolerate failures: an unhealthy
	// replica must not block server start; reads will go to the writer.
	if replicaDSN := cfg.GetReplicaDSN(); replicaDSN != "" {
		replicaConfig, rErr := pgxpool.ParseConfig(replicaDSN)
		if rErr == nil {
			replicaConfig.MaxConns = cfg.MaxConns
			replicaConfig.MinConns = cfg.MinConns
			replicaConfig.MaxConnLifetime = cfg.MaxConnLifetime
			replicaConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
			replicaPool, rErr := pgxpool.NewWithConfig(ctx, replicaConfig)
			if rErr == nil {
				if pingErr := replicaPool.Ping(ctx); pingErr == nil {
					db.ReplicaPool = replicaPool
				} else {
					replicaPool.Close()
				}
			}
		}
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() {
	db.Pool.Close()
	if db.ReplicaPool != nil {
		db.ReplicaPool.Close()
	}
}

// Health checks database health
func (db *DB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.Pool.Ping(ctx)
}

// Stats returns database connection pool statistics
func (db *DB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}

// Begin starts a new transaction
func (db *DB) Begin(ctx context.Context) (pgx.Tx, error) {
	return db.Pool.Begin(ctx)
}

// WithTransaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
func (db *DB) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
