package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// Migration represents a database schema migration.
type Migration struct {
	Up          func(*sql.Tx) error
	Description string
	Version     int
}

var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				`CREATE TABLE IF NOT EXISTS transactions (
					id TEXT PRIMARY KEY,
					hash TEXT UNIQUE NOT NULL,
					date DATETIME NOT NULL,
					name TEXT NOT NULL,
					merchant_name TEXT,
					amount REAL NOT NULL,
					plaid_categories TEXT,
					account_id TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE INDEX idx_transactions_date ON transactions(date)`,
				`CREATE INDEX idx_transactions_merchant ON transactions(merchant_name)`,
				`CREATE INDEX idx_transactions_hash ON transactions(hash)`,

				`CREATE TABLE IF NOT EXISTS vendors (
					name TEXT PRIMARY KEY,
					category TEXT NOT NULL,
					last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
					use_count INTEGER DEFAULT 0
				)`,

				`CREATE TABLE IF NOT EXISTS classifications (
					transaction_id TEXT PRIMARY KEY,
					category TEXT NOT NULL,
					status TEXT NOT NULL,
					confidence REAL DEFAULT 0,
					classified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					notes TEXT,
					FOREIGN KEY (transaction_id) REFERENCES transactions(id)
				)`,
				`CREATE INDEX idx_classifications_category ON classifications(category)`,

				`CREATE TABLE IF NOT EXISTS progress (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					last_processed_id TEXT,
					last_processed_date DATETIME,
					total_processed INTEGER DEFAULT 0,
					started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query: %w", err)
				}
			}
			return nil
		},
	},
	{
		Version:     2,
		Description: "Add classification history for auditing",
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS classification_history (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					transaction_id TEXT NOT NULL,
					category TEXT NOT NULL,
					status TEXT NOT NULL,
					confidence REAL DEFAULT 0,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (transaction_id) REFERENCES transactions(id)
				)
			`)
			return err
		},
	},
	{
		Version:     3,
		Description: "Optimize database indexes",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				// Add missing index for foreign key lookups
				`CREATE INDEX IF NOT EXISTS idx_classification_history_transaction_id ON classification_history(transaction_id)`,
				// Drop redundant index (UNIQUE constraint already creates an index)
				`DROP INDEX IF EXISTS idx_transactions_hash`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query '%s': %w", query, err)
				}
			}
			return nil
		},
	},
}

// Migrate applies all pending database migrations.
func (s *SQLiteStorage) Migrate(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	// Get current version
	var currentVersion int
	err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	// Apply migrations
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if err := migration.Up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		// Update version
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", migration.Version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update schema version: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		slog.Info("Applied migration",
			"version", migration.Version,
			"description", migration.Description)
	}

	return nil
}
