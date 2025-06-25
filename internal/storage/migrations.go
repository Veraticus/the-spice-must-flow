package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// ExpectedSchemaVersion is the latest schema version that the application expects.
// If the database cannot be migrated to this version, it's a fatal error.
const ExpectedSchemaVersion = 10

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
	{
		Version:     4,
		Description: "Add categories table for dynamic categorization",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				`CREATE TABLE IF NOT EXISTS categories (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT UNIQUE NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					is_active BOOLEAN DEFAULT 1
				)`,
				`CREATE INDEX idx_categories_name ON categories(name)`,
				`CREATE INDEX idx_categories_active ON categories(is_active)`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query '%s': %w", query, err)
				}
			}
			return nil
		},
	},
	{
		Version:     5,
		Description: "Add generic transaction metadata fields",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				// Rename plaid_categories to categories for source-agnostic design
				`ALTER TABLE transactions RENAME COLUMN plaid_categories TO categories`,
				// Add new fields for transaction metadata
				`ALTER TABLE transactions ADD COLUMN transaction_type TEXT`,
				`ALTER TABLE transactions ADD COLUMN check_number TEXT`,
				// Add index for transaction type
				`CREATE INDEX idx_transactions_type ON transactions(transaction_type)`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query '%s': %w", query, err)
				}
			}
			return nil
		},
	},
	{
		Version:     6,
		Description: "Add description field to categories",
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				ALTER TABLE categories 
				ADD COLUMN description TEXT DEFAULT ''
			`)
			if err != nil {
				return fmt.Errorf("failed to add description column: %w", err)
			}
			return nil
		},
	},
	{
		Version:     7,
		Description: "Add checkpoint metadata table",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				`CREATE TABLE IF NOT EXISTS checkpoint_metadata (
					id TEXT PRIMARY KEY,
					created_at DATETIME NOT NULL,
					description TEXT,
					file_size INTEGER,
					row_counts TEXT,
					schema_version INTEGER,
					is_auto BOOLEAN DEFAULT 0,
					parent_checkpoint TEXT
				)`,
				`CREATE INDEX idx_checkpoint_metadata_created_at ON checkpoint_metadata(created_at)`,
				`CREATE INDEX idx_checkpoint_metadata_is_auto ON checkpoint_metadata(is_auto)`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query '%s': %w", query, err)
				}
			}
			return nil
		},
	},
	{
		Version:     8,
		Description: "Add check patterns table for intelligent check categorization",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				`CREATE TABLE IF NOT EXISTS check_patterns (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					pattern_name TEXT NOT NULL,
					amount_min REAL,
					amount_max REAL,
					check_number_pattern TEXT,
					day_of_month_min INTEGER,
					day_of_month_max INTEGER,
					category TEXT NOT NULL,
					notes TEXT,
					confidence_boost REAL DEFAULT 0.3,
					active BOOLEAN DEFAULT 1,
					use_count INTEGER DEFAULT 0,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE INDEX idx_check_patterns_amount ON check_patterns(amount_min, amount_max)`,
				`CREATE INDEX idx_check_patterns_active ON check_patterns(active)`,
				`CREATE INDEX idx_check_patterns_category ON check_patterns(category)`,
				// Add trigger to update updated_at timestamp
				`CREATE TRIGGER update_check_patterns_timestamp
				AFTER UPDATE ON check_patterns
				FOR EACH ROW
				WHEN NEW.updated_at = OLD.updated_at
				BEGIN
					UPDATE check_patterns SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query '%s': %w", query, err)
				}
			}
			return nil
		},
	},
	{
		Version:     9,
		Description: "Add income/expense tracking fields",
		Up: func(tx *sql.Tx) error {
			queries := []string{
				// Add direction field to transactions
				`ALTER TABLE transactions ADD COLUMN direction TEXT`,
				// Add refund tracking fields
				`ALTER TABLE transactions ADD COLUMN is_refund BOOLEAN DEFAULT 0`,
				`ALTER TABLE transactions ADD COLUMN refund_category TEXT`,
				// Add type field to categories
				`ALTER TABLE categories ADD COLUMN type TEXT`,
				// Create indexes for new fields
				`CREATE INDEX idx_transactions_direction ON transactions(direction)`,
				`CREATE INDEX idx_categories_type ON categories(type)`,
			}

			for _, query := range queries {
				if _, err := tx.Exec(query); err != nil {
					return fmt.Errorf("failed to execute query '%s': %w", query, err)
				}
			}
			return nil
		},
	},
	{
		Version:     10,
		Description: "Clean up category descriptions with DESCRIPTION/CONFIDENCE prefixes",
		Up: func(tx *sql.Tx) error {
			// First, let's check what categories need cleaning
			rows, err := tx.Query(`SELECT id, description FROM categories WHERE description LIKE 'DESCRIPTION:%'`)
			if err != nil {
				return fmt.Errorf("failed to query categories: %w", err)
			}
			defer func() {
				if err := rows.Close(); err != nil {
					slog.Warn("failed to close rows", "error", err)
				}
			}()

			type categoryUpdate struct {
				description string
				id          int
			}
			var updates []categoryUpdate

			for rows.Next() {
				var id int
				var desc string
				if err := rows.Scan(&id, &desc); err != nil {
					return fmt.Errorf("failed to scan category: %w", err)
				}

				// Extract just the description part
				lines := strings.Split(desc, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "DESCRIPTION:") {
						cleanDesc := strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
						if cleanDesc != "" {
							updates = append(updates, categoryUpdate{id: id, description: cleanDesc})
							break
						}
					}
				}
			}

			if err := rows.Err(); err != nil {
				return fmt.Errorf("error iterating categories: %w", err)
			}

			// Apply updates
			for _, update := range updates {
				_, err := tx.Exec(`UPDATE categories SET description = ? WHERE id = ?`, update.description, update.id)
				if err != nil {
					return fmt.Errorf("failed to update category %d: %w", update.id, err)
				}
				slog.Info("cleaned category description", "id", update.id)
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

		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return fmt.Errorf("failed to begin transaction: %w", txErr)
		}

		if upErr := migration.Up(tx); upErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", migration.Version, upErr)
		}

		// Update version
		if _, execErr := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", migration.Version)); execErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update schema version: %w", execErr)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, commitErr)
		}

		slog.Info("Applied migration",
			"version", migration.Version,
			"description", migration.Description)
	}

	// Verify we're at the expected schema version
	var finalVersion int
	err = s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&finalVersion)
	if err != nil {
		return fmt.Errorf("failed to verify final schema version: %w", err)
	}

	if finalVersion != ExpectedSchemaVersion {
		return fmt.Errorf("database schema version mismatch: expected %d, got %d", ExpectedSchemaVersion, finalVersion)
	}

	return nil
}
