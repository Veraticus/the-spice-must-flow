package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Veraticus/the-spice-must-flow/internal/config"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/viper"
)

// getDatabase returns a database connection and a cleanup function.
func getDatabase() (*storage.SQLiteStorage, func(), error) {
	// Get database path from config
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "$HOME/.local/share/spice/spice.db"
	}

	// Expand tilde and environment variables
	dbPath = config.ExpandPath(dbPath)

	// Open database
	db, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			slog.Error("Failed to close database", "error", err)
		}
	}

	return db, cleanup, nil
}
