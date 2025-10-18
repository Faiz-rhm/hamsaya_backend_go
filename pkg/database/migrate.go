package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// Migrator handles database migrations
type Migrator struct {
	db             *DB
	migrationsPath string
}

// NewMigrator creates a new migrator
func NewMigrator(db *DB, migrationsPath string) *Migrator {
	return &Migrator{
		db:             db,
		migrationsPath: migrationsPath,
	}
}

// ensureMigrationsTable creates the migrations tracking table if it doesn't exist
func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`
	_, err := m.db.Pool.Exec(ctx, query)
	return err
}

// getAppliedMigrations returns a map of applied migration versions
func (m *Migrator) getAppliedMigrations(ctx context.Context) (map[int]bool, error) {
	applied := make(map[int]bool)

	rows, err := m.db.Pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// loadMigrations loads all migration files from the migrations directory
func (m *Migrator) loadMigrations() ([]Migration, error) {
	var migrations []Migration

	err := filepath.Walk(m.migrationsPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		filename := filepath.Base(path)
		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			return nil // Skip invalid filenames
		}

		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			return nil // Skip files with invalid version numbers
		}

		isUp := strings.HasSuffix(filename, ".up.sql")
		isDown := strings.HasSuffix(filename, ".down.sql")

		if !isUp && !isDown {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		// Find or create migration entry
		var migration *Migration
		for i := range migrations {
			if migrations[i].Version == version {
				migration = &migrations[i]
				break
			}
		}

		if migration == nil {
			// Create new migration entry
			name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".up.sql")
			name = strings.TrimSuffix(name, ".down.sql")
			migrations = append(migrations, Migration{
				Version: version,
				Name:    name,
			})
			migration = &migrations[len(migrations)-1]
		}

		if isUp {
			migration.UpSQL = string(content)
		} else {
			migration.DownSQL = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Up applies all pending migrations
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	for _, migration := range migrations {
		if applied[migration.Version] {
			continue // Skip already applied migrations
		}

		if migration.UpSQL == "" {
			return fmt.Errorf("migration %d (%s) is missing up.sql", migration.Version, migration.Name)
		}

		fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Name)

		// Execute migration in a transaction
		tx, err := m.db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		if err := m.executeMigration(ctx, tx, migration); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		fmt.Printf("✓ Migration %d applied successfully\n", migration.Version)
	}

	fmt.Println("All migrations applied successfully")
	return nil
}

// executeMigration executes a single migration
func (m *Migrator) executeMigration(ctx context.Context, tx pgx.Tx, migration Migration) error {
	// Execute the migration SQL
	if _, err := tx.Exec(ctx, migration.UpSQL); err != nil {
		return err
	}

	// Record the migration
	_, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES ($1, $2, $3)",
		migration.Version, migration.Name, time.Now(),
	)
	return err
}

// Down rolls back the last applied migration
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get the last applied migration
	var version int
	var name string
	err := m.db.Pool.QueryRow(ctx,
		"SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1",
	).Scan(&version, &name)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("No migrations to roll back")
			return nil
		}
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Find the migration
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == version {
			migration = &migrations[i]
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %d not found in files", version)
	}

	if migration.DownSQL == "" {
		return fmt.Errorf("migration %d (%s) is missing down.sql", version, name)
	}

	fmt.Printf("Rolling back migration %d: %s\n", version, name)

	// Execute rollback in a transaction
	tx, err := m.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if _, err := tx.Exec(ctx, migration.DownSQL); err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("failed to execute down migration: %w", err)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	fmt.Printf("✓ Migration %d rolled back successfully\n", version)
	return nil
}

// Status shows the current migration status
func (m *Migrator) Status(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	fmt.Println("Migration Status:")
	fmt.Println("================")
	for _, migration := range migrations {
		status := "✗ Pending"
		if applied[migration.Version] {
			status = "✓ Applied"
		}
		fmt.Printf("%s - %d: %s\n", status, migration.Version, migration.Name)
	}

	return nil
}
