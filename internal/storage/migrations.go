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
const ExpectedSchemaVersion = 16

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
	{
		Version:     11,
		Description: "Remove active column from check_patterns",
		Up: func(tx *sql.Tx) error {
			// Create new table without active column
			if _, err := tx.Exec(`
				CREATE TABLE check_patterns_new (
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
					use_count INTEGER DEFAULT 0,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)
			`); err != nil {
				return fmt.Errorf("failed to create new check_patterns table: %w", err)
			}

			// Copy data from old table (excluding inactive patterns)
			if _, err := tx.Exec(`
				INSERT INTO check_patterns_new
				SELECT id, pattern_name, amount_min, amount_max, check_number_pattern,
					day_of_month_min, day_of_month_max, category, notes,
					confidence_boost, use_count, created_at, updated_at
				FROM check_patterns
				WHERE active = 1
			`); err != nil {
				return fmt.Errorf("failed to copy check patterns: %w", err)
			}

			// Drop old table
			if _, err := tx.Exec(`DROP TABLE check_patterns`); err != nil {
				return fmt.Errorf("failed to drop old check_patterns table: %w", err)
			}

			// Rename new table
			if _, err := tx.Exec(`ALTER TABLE check_patterns_new RENAME TO check_patterns`); err != nil {
				return fmt.Errorf("failed to rename check_patterns table: %w", err)
			}

			// Recreate indexes
			if _, err := tx.Exec(`CREATE INDEX idx_check_patterns_amount ON check_patterns(amount_min, amount_max)`); err != nil {
				return fmt.Errorf("failed to create amount index: %w", err)
			}
			if _, err := tx.Exec(`CREATE INDEX idx_check_patterns_category ON check_patterns(category)`); err != nil {
				return fmt.Errorf("failed to create category index: %w", err)
			}

			return nil
		},
	},
	{
		Version:     12,
		Description: "Add amounts column to check_patterns",
		Up: func(tx *sql.Tx) error {
			// Add amounts column to store multiple specific amounts as JSON
			if _, err := tx.Exec(`ALTER TABLE check_patterns ADD COLUMN amounts TEXT`); err != nil {
				return fmt.Errorf("failed to add amounts column: %w", err)
			}

			return nil
		},
	},
	{
		Version:     13,
		Description: "Convert negative transaction amounts to absolute values",
		Up: func(tx *sql.Tx) error {
			// First, let's see how many negative amounts we have
			var negativeCount int
			err := tx.QueryRow(`SELECT COUNT(*) FROM transactions WHERE amount < 0`).Scan(&negativeCount)
			if err != nil {
				return fmt.Errorf("failed to count negative amounts: %w", err)
			}

			if negativeCount > 0 {
				slog.Info("Converting negative transaction amounts to positive", "count", negativeCount)

				// Convert all negative amounts to positive (absolute values)
				_, err := tx.Exec(`UPDATE transactions SET amount = ABS(amount) WHERE amount < 0`)
				if err != nil {
					return fmt.Errorf("failed to convert negative amounts: %w", err)
				}

				// Verify the conversion
				var remainingNegative int
				err = tx.QueryRow(`SELECT COUNT(*) FROM transactions WHERE amount < 0`).Scan(&remainingNegative)
				if err != nil {
					return fmt.Errorf("failed to verify conversion: %w", err)
				}

				if remainingNegative > 0 {
					return fmt.Errorf("conversion failed: %d transactions still have negative amounts", remainingNegative)
				}

				slog.Info("Successfully converted all negative amounts to positive")
			}

			return nil
		},
	},
	{
		Version:     14,
		Description: "Add source column to vendors for tracking creation origin",
		Up: func(tx *sql.Tx) error {
			// Add source column with default value 'AUTO'
			if _, err := tx.Exec(`ALTER TABLE vendors ADD COLUMN source TEXT DEFAULT 'AUTO'`); err != nil {
				return fmt.Errorf("failed to add source column: %w", err)
			}

			// Update existing vendors using heuristic: vendors with use_count > 10 are considered confirmed
			result, err := tx.Exec(`UPDATE vendors SET source = 'AUTO_CONFIRMED' WHERE use_count > 10`)
			if err != nil {
				return fmt.Errorf("failed to update vendor sources: %w", err)
			}

			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("failed to get rows affected: %w", err)
			}

			slog.Info("Updated vendor sources based on usage", "vendors_marked_confirmed", rowsAffected)

			// Create index on source column for filtering
			if _, err := tx.Exec(`CREATE INDEX idx_vendors_source ON vendors(source)`); err != nil {
				return fmt.Errorf("failed to create source index: %w", err)
			}

			return nil
		},
	},
	{
		Version:     15,
		Description: "Add regex support to vendors",
		Up: func(tx *sql.Tx) error {
			// Add is_regex column to track whether the vendor name is a regex pattern
			if _, err := tx.Exec(`ALTER TABLE vendors ADD COLUMN is_regex BOOLEAN DEFAULT FALSE`); err != nil {
				return fmt.Errorf("failed to add is_regex column: %w", err)
			}

			// Create index on is_regex for efficient filtering
			if _, err := tx.Exec(`CREATE INDEX idx_vendors_is_regex ON vendors(is_regex)`); err != nil {
				return fmt.Errorf("failed to create is_regex index: %w", err)
			}

			slog.Info("Added regex support to vendors table")
			return nil
		},
	},
	{
		Version:     16,
		Description: "Remove confidence boost from check patterns",
		Up: func(tx *sql.Tx) error {
			// SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
			// Step 1: Create new table without confidence_boost
			if _, err := tx.Exec(`
				CREATE TABLE check_patterns_new (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					pattern_name TEXT NOT NULL,
					amount_min REAL,
					amount_max REAL,
					check_number_pattern TEXT,
					day_of_month_min INTEGER,
					day_of_month_max INTEGER,
					category TEXT NOT NULL,
					notes TEXT,
					use_count INTEGER DEFAULT 0,
					amounts TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)
			`); err != nil {
				return fmt.Errorf("failed to create new check_patterns table: %w", err)
			}

			// Step 2: Copy data from old table to new table (excluding confidence_boost)
			if _, err := tx.Exec(`
				INSERT INTO check_patterns_new (
					id, pattern_name, amount_min, amount_max, check_number_pattern,
					day_of_month_min, day_of_month_max, category, notes,
					use_count, amounts, created_at, updated_at
				)
				SELECT 
					id, pattern_name, amount_min, amount_max, check_number_pattern,
					day_of_month_min, day_of_month_max, category, notes,
					use_count, amounts, created_at, updated_at
				FROM check_patterns
			`); err != nil {
				return fmt.Errorf("failed to copy check patterns data: %w", err)
			}

			// Step 3: Drop old table
			if _, err := tx.Exec(`DROP TABLE check_patterns`); err != nil {
				return fmt.Errorf("failed to drop old check_patterns table: %w", err)
			}

			// Step 4: Rename new table to original name
			if _, err := tx.Exec(`ALTER TABLE check_patterns_new RENAME TO check_patterns`); err != nil {
				return fmt.Errorf("failed to rename check_patterns table: %w", err)
			}

			// Step 5: Recreate indexes
			if _, err := tx.Exec(`CREATE INDEX idx_check_patterns_category ON check_patterns(category)`); err != nil {
				return fmt.Errorf("failed to create category index: %w", err)
			}
			if _, err := tx.Exec(`CREATE INDEX idx_check_patterns_amount ON check_patterns(amount_min, amount_max)`); err != nil {
				return fmt.Errorf("failed to create amount index: %w", err)
			}

			// Step 6: Recreate trigger for updated_at
			if _, err := tx.Exec(`
				CREATE TRIGGER update_check_patterns_updated_at
				AFTER UPDATE ON check_patterns
				FOR EACH ROW
				BEGIN
					UPDATE check_patterns SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END
			`); err != nil {
				return fmt.Errorf("failed to create updated_at trigger: %w", err)
			}

			slog.Info("Removed confidence boost from check patterns")
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
