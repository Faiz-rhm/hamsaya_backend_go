package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := utils.InitLogger(cfg.Server.LogLevel); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.Sync()

	logger := utils.GetLogger()

	// Connect to database
	logger.Info("Connecting to database...")
	db, err := database.New(&cfg.Database)
	if err != nil {
		logger.Fatalw("Failed to connect to database", "error", err)
	}
	defer db.Close()
	logger.Info("Database connected successfully")

	// Create migrator
	migrator := database.NewMigrator(db, "./migrations")

	// Get command from args
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute command
	switch command {
	case "up":
		logger.Info("Running migrations...")
		if err := migrator.Up(ctx); err != nil {
			logger.Fatalw("Failed to run migrations", "error", err)
		}

	case "down":
		logger.Info("Rolling back last migration...")
		if err := migrator.Down(ctx); err != nil {
			logger.Fatalw("Failed to rollback migration", "error", err)
		}

	case "status":
		logger.Info("Checking migration status...")
		if err := migrator.Status(ctx); err != nil {
			logger.Fatalw("Failed to get migration status", "error", err)
		}

	default:
		fmt.Println("Usage: migrate [up|down|status]")
		os.Exit(1)
	}

	logger.Info("Migration command completed successfully")
}
