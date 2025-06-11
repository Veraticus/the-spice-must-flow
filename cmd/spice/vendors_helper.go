package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joshsymonds/the-spice-must-flow/internal/storage"
	"github.com/spf13/viper"
)

// getDatabase returns a database connection and a cleanup function.
func getDatabase() (*storage.SQLiteStorage, func(), error) {
	// Get database path from config
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "~/.local/share/spice/spice.db"
	}

	// Expand ~ to home directory
	if strings.HasPrefix(dbPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = home + dbPath[1:]
	}

	// Open database
	db, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			slog.Error("Failed to close database", "error", err)
		}
	}

	return db, cleanup, nil
}
