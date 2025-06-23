package main

import (
	"context"
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/config"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/viper"
)

// initStorage initializes the storage service with proper path expansion.
func initStorage(ctx context.Context) (service.Storage, error) {
	// Get database path from config
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "$HOME/.local/share/spice/spice.db"
	}

	// Expand tilde and environment variables
	dbPath = config.ExpandPath(dbPath)

	// Initialize storage
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, err
	}

	// Run migrations
	if err := store.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}
