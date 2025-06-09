package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joshsymonds/the-spice-must-flow/internal/storage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long: `Initialize or update the database schema to the latest version.
		
This command ensures your local database has all the required
tables and indexes for the application to function properly.`,
		RunE: runMigrate,
	}

	// Flags
	cmd.Flags().Bool("force", false, "Force migration even if already at latest version")
	cmd.Flags().Bool("status", false, "Show current migration status without applying changes")

	return cmd
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	status, _ := cmd.Flags().GetBool("status")

	// Get database path from config
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		// Default path
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(home, ".local", "share", "spice", "spice.db")
	}

	slog.Info("Starting database migration",
		"database", dbPath,
		"force", force,
		"status_only", status)

	// Create storage instance
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = store.Close() }()

	if status {
		// TODO: Add method to get current schema version
		slog.Info("📊 Database Migration Status")
		slog.Info("Database", "path", dbPath)
		slog.Info("Current version: (checking...)")
		slog.Info("Latest version: 2")
		slog.Warn("Migration status check not yet fully implemented")
		return nil
	}

	slog.Info("🗄️  Running database migrations...")
	slog.Info("Database", "path", dbPath)

	// Run migrations
	ctx := cmd.Context()
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	slog.Info("✅ Database migrations completed successfully!")

	return nil
}
