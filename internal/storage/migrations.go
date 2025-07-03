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
const ExpectedSchemaVersion = 22

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
	{
		Version:     17,
		Description: "Add business_percent to classifications",
		Up: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`
				ALTER TABLE classifications 
				ADD COLUMN business_percent REAL DEFAULT 0
			`); err != nil {
				return fmt.Errorf("failed to add business_percent column: %w", err)
			}

			slog.Info("Added business_percent column to classifications table")
			return nil
		},
	},
	{
		Version:     18,
		Description: "Add pattern rules for intelligent categorization",
		Up: func(tx *sql.Tx) error {
			// Create pattern_rules table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS pattern_rules (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT NOT NULL,
					description TEXT,
					merchant_pattern TEXT,
					is_regex BOOLEAN DEFAULT FALSE,
					amount_condition TEXT CHECK(amount_condition IN ('lt', 'le', 'eq', 'ge', 'gt', 'range', 'any')),
					amount_value REAL,
					amount_min REAL,
					amount_max REAL,
					direction TEXT CHECK(direction IN ('income', 'expense', 'transfer') OR direction IS NULL),
					default_category TEXT NOT NULL,
					confidence REAL DEFAULT 0.8 CHECK(confidence >= 0 AND confidence <= 1),
					priority INTEGER DEFAULT 0,
					is_active BOOLEAN DEFAULT TRUE,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					use_count INTEGER DEFAULT 0
				)
			`); err != nil {
				return fmt.Errorf("failed to create pattern_rules table: %w", err)
			}

			// Create indexes for pattern_rules
			indexes := []string{
				`CREATE INDEX idx_pattern_rules_merchant ON pattern_rules(merchant_pattern)`,
				`CREATE INDEX idx_pattern_rules_category ON pattern_rules(default_category)`,
				`CREATE INDEX idx_pattern_rules_active ON pattern_rules(is_active)`,
				`CREATE INDEX idx_pattern_rules_priority ON pattern_rules(priority DESC)`,
			}

			for _, index := range indexes {
				if _, err := tx.Exec(index); err != nil {
					return fmt.Errorf("failed to create index: %w", err)
				}
			}

			// Create trigger to update updated_at timestamp
			if _, err := tx.Exec(`
				CREATE TRIGGER update_pattern_rules_timestamp 
				AFTER UPDATE ON pattern_rules
				FOR EACH ROW
				BEGIN
					UPDATE pattern_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END
			`); err != nil {
				return fmt.Errorf("failed to create updated_at trigger: %w", err)
			}

			slog.Info("Created pattern_rules table for intelligent categorization")
			return nil
		},
	},
	{
		Version:     19,
		Description: "Add AI analysis session and report tables",
		Up: func(tx *sql.Tx) error {
			// Create analysis_sessions table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS analysis_sessions (
					id TEXT PRIMARY KEY,
					started_at DATETIME NOT NULL,
					last_attempt DATETIME NOT NULL,
					completed_at DATETIME,
					status TEXT NOT NULL CHECK(status IN ('pending', 'in_progress', 'validating', 'completed', 'failed')),
					attempts INTEGER DEFAULT 0,
					error TEXT,
					report_id TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)
			`); err != nil {
				return fmt.Errorf("failed to create analysis_sessions table: %w", err)
			}

			// Create analysis_reports table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS analysis_reports (
					id TEXT PRIMARY KEY,
					session_id TEXT NOT NULL,
					generated_at DATETIME NOT NULL,
					period_start DATETIME NOT NULL,
					period_end DATETIME NOT NULL,
					coherence_score REAL NOT NULL CHECK(coherence_score >= 0 AND coherence_score <= 1),
					insights TEXT, -- JSON array
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (session_id) REFERENCES analysis_sessions(id)
				)
			`); err != nil {
				return fmt.Errorf("failed to create analysis_reports table: %w", err)
			}

			// Create analysis_issues table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS analysis_issues (
					id TEXT PRIMARY KEY,
					report_id TEXT NOT NULL,
					type TEXT NOT NULL CHECK(type IN ('miscategorized', 'inconsistent', 'missing_pattern', 'duplicate_pattern', 'ambiguous_vendor')),
					severity TEXT NOT NULL CHECK(severity IN ('critical', 'high', 'medium', 'low')),
					description TEXT NOT NULL,
					current_category TEXT,
					suggested_category TEXT,
					transaction_ids TEXT NOT NULL, -- JSON array
					affected_count INTEGER NOT NULL,
					confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (report_id) REFERENCES analysis_reports(id)
				)
			`); err != nil {
				return fmt.Errorf("failed to create analysis_issues table: %w", err)
			}

			// Create analysis_fixes table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS analysis_fixes (
					id TEXT PRIMARY KEY,
					issue_id TEXT NOT NULL,
					type TEXT NOT NULL,
					description TEXT NOT NULL,
					data TEXT NOT NULL, -- JSON object
					applied BOOLEAN DEFAULT FALSE,
					applied_at DATETIME,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (issue_id) REFERENCES analysis_issues(id)
				)
			`); err != nil {
				return fmt.Errorf("failed to create analysis_fixes table: %w", err)
			}

			// Create analysis_suggested_patterns table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS analysis_suggested_patterns (
					id TEXT PRIMARY KEY,
					report_id TEXT NOT NULL,
					name TEXT NOT NULL,
					description TEXT NOT NULL,
					impact TEXT NOT NULL,
					pattern TEXT NOT NULL, -- JSON object representing PatternRule
					example_txn_ids TEXT NOT NULL, -- JSON array
					match_count INTEGER NOT NULL,
					confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (report_id) REFERENCES analysis_reports(id)
				)
			`); err != nil {
				return fmt.Errorf("failed to create analysis_suggested_patterns table: %w", err)
			}

			// Create analysis_category_stats table
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS analysis_category_stats (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					report_id TEXT NOT NULL,
					category_id TEXT NOT NULL,
					category_name TEXT NOT NULL,
					transaction_count INTEGER NOT NULL,
					total_amount REAL NOT NULL,
					consistency REAL NOT NULL CHECK(consistency >= 0 AND consistency <= 1),
					issues INTEGER NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (report_id) REFERENCES analysis_reports(id)
				)
			`); err != nil {
				return fmt.Errorf("failed to create analysis_category_stats table: %w", err)
			}

			// Create indexes for analysis tables
			indexes := []string{
				// Sessions indexes
				`CREATE INDEX idx_analysis_sessions_status ON analysis_sessions(status)`,
				`CREATE INDEX idx_analysis_sessions_report_id ON analysis_sessions(report_id)`,
				`CREATE INDEX idx_analysis_sessions_started_at ON analysis_sessions(started_at DESC)`,

				// Reports indexes
				`CREATE INDEX idx_analysis_reports_session_id ON analysis_reports(session_id)`,
				`CREATE INDEX idx_analysis_reports_period ON analysis_reports(period_start, period_end)`,

				// Issues indexes
				`CREATE INDEX idx_analysis_issues_report_id ON analysis_issues(report_id)`,
				`CREATE INDEX idx_analysis_issues_type ON analysis_issues(type)`,
				`CREATE INDEX idx_analysis_issues_severity ON analysis_issues(severity)`,

				// Fixes indexes
				`CREATE INDEX idx_analysis_fixes_issue_id ON analysis_fixes(issue_id)`,
				`CREATE INDEX idx_analysis_fixes_applied ON analysis_fixes(applied)`,

				// Suggested patterns indexes
				`CREATE INDEX idx_analysis_suggested_patterns_report_id ON analysis_suggested_patterns(report_id)`,

				// Category stats indexes
				`CREATE INDEX idx_analysis_category_stats_report_id ON analysis_category_stats(report_id)`,
				`CREATE INDEX idx_analysis_category_stats_category_id ON analysis_category_stats(category_id)`,
			}

			for _, index := range indexes {
				if _, err := tx.Exec(index); err != nil {
					return fmt.Errorf("failed to create index: %w", err)
				}
			}

			// Create triggers for updated_at timestamps
			if _, err := tx.Exec(`
				CREATE TRIGGER update_analysis_sessions_timestamp 
				AFTER UPDATE ON analysis_sessions
				FOR EACH ROW
				BEGIN
					UPDATE analysis_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END
			`); err != nil {
				return fmt.Errorf("failed to create analysis_sessions updated_at trigger: %w", err)
			}

			slog.Info("Created AI analysis tables for session management and report storage")
			return nil
		},
	},
	{
		Version:     20,
		Description: "Remove CHECK constraint on analysis issue types to allow AI flexibility",
		Up: func(tx *sql.Tx) error {
			// SQLite doesn't support ALTER TABLE DROP CONSTRAINT
			// We need to recreate the table without the constraint

			// Create new table without CHECK constraint
			if _, err := tx.Exec(`
				CREATE TABLE analysis_issues_new (
					id TEXT PRIMARY KEY,
					report_id TEXT NOT NULL,
					type TEXT NOT NULL,
					severity TEXT NOT NULL CHECK(severity IN ('critical', 'high', 'medium', 'low')),
					description TEXT NOT NULL,
					current_category TEXT,
					suggested_category TEXT,
					transaction_ids TEXT NOT NULL,
					affected_count INTEGER NOT NULL,
					confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (report_id) REFERENCES analysis_reports(id)
				)
			`); err != nil {
				return fmt.Errorf("failed to create new analysis_issues table: %w", err)
			}

			// Copy data from old table
			if _, err := tx.Exec(`
				INSERT INTO analysis_issues_new 
				SELECT * FROM analysis_issues
			`); err != nil {
				return fmt.Errorf("failed to copy analysis_issues data: %w", err)
			}

			// Drop old table
			if _, err := tx.Exec(`DROP TABLE analysis_issues`); err != nil {
				return fmt.Errorf("failed to drop old analysis_issues table: %w", err)
			}

			// Rename new table
			if _, err := tx.Exec(`ALTER TABLE analysis_issues_new RENAME TO analysis_issues`); err != nil {
				return fmt.Errorf("failed to rename analysis_issues table: %w", err)
			}

			// Recreate indexes
			indexes := []string{
				`CREATE INDEX idx_analysis_issues_report_id ON analysis_issues(report_id)`,
				`CREATE INDEX idx_analysis_issues_type ON analysis_issues(type)`,
				`CREATE INDEX idx_analysis_issues_severity ON analysis_issues(severity)`,
			}

			for _, index := range indexes {
				if _, err := tx.Exec(index); err != nil {
					return fmt.Errorf("failed to recreate index: %w", err)
				}
			}

			slog.Info("Removed CHECK constraint on analysis issue types to allow AI flexibility")
			return nil
		},
	},
	{
		Version:     21,
		Description: "Add default business percentage to categories",
		Up: func(tx *sql.Tx) error {
			// Add default_business_percent column to categories table
			if _, err := tx.Exec(`
				ALTER TABLE categories 
				ADD COLUMN default_business_percent INTEGER DEFAULT 0
			`); err != nil {
				return fmt.Errorf("failed to add default_business_percent column: %w", err)
			}

			// Set sensible defaults based on category names and types
			defaults := []struct {
				pattern string
				percent int
			}{
				// Office/Work categories - 100% business
				{pattern: "%office%", percent: 100},
				{pattern: "%work%", percent: 100},
				{pattern: "%business%", percent: 100},
				{pattern: "%professional%", percent: 100},
				{pattern: "%consulting%", percent: 100},
				{pattern: "%freelance%", percent: 100},

				// Partially deductible categories - 50%
				{pattern: "%meal%", percent: 50},
				{pattern: "%dining%", percent: 50},
				{pattern: "%restaurant%", percent: 50},
				{pattern: "%entertainment%", percent: 50},
				{pattern: "%conference%", percent: 50},
				{pattern: "%travel%", percent: 50},

				// Personal categories - 0%
				{pattern: "%personal%", percent: 0},
				{pattern: "%home%", percent: 0},
				{pattern: "%family%", percent: 0},
				{pattern: "%groceries%", percent: 0},
				{pattern: "%medical%", percent: 0},
				{pattern: "%health%", percent: 0},
			}

			// Apply defaults based on patterns
			for _, def := range defaults {
				if _, err := tx.Exec(`
					UPDATE categories 
					SET default_business_percent = ?
					WHERE LOWER(name) LIKE LOWER(?) 
					AND default_business_percent = 0
				`, def.percent, def.pattern); err != nil {
					return fmt.Errorf("failed to set default for pattern %s: %w", def.pattern, err)
				}
			}

			// For expense categories without a match, default to 0%
			// Income and system categories should always be 0%
			if _, err := tx.Exec(`
				UPDATE categories 
				SET default_business_percent = 0
				WHERE type IN ('income', 'system')
			`); err != nil {
				return fmt.Errorf("failed to set defaults for income/system categories: %w", err)
			}

			slog.Info("Added default business percentage to categories")
			return nil
		},
	},
	{
		Version:     22,
		Description: "Reset all category default business percentages to 0",
		Up: func(tx *sql.Tx) error {
			// Reset all default business percentages to 0
			// Users should explicitly set business percentages as needed
			if _, err := tx.Exec(`
				UPDATE categories 
				SET default_business_percent = 0
			`); err != nil {
				return fmt.Errorf("failed to reset default business percentages: %w", err)
			}

			slog.Info("Reset all category default business percentages to 0")
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
