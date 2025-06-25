package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// SaveTransactions saves multiple transactions to the database.
func (s *SQLiteStorage) SaveTransactions(ctx context.Context, transactions []model.Transaction) error {
	// Validate inputs
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateTransactions(transactions); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.saveTransactionsTx(ctx, tx, transactions); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStorage) saveTransactionsTx(ctx context.Context, tx *sql.Tx, transactions []model.Transaction) error {
	// Check schema version to determine which columns to use
	var schemaVersion int
	err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	// Use appropriate columns based on schema version
	var stmt *sql.Stmt
	switch {
	case schemaVersion >= 7:
		// Schema with direction field
		stmt, err = tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO transactions (
				id, hash, date, name, merchant_name, amount, 
				categories, account_id, transaction_type, check_number, direction
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	case schemaVersion >= 5:
		// New schema with generic fields
		stmt, err = tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO transactions (
				id, hash, date, name, merchant_name, amount, 
				categories, account_id, transaction_type, check_number
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	default:
		// Old schema
		stmt, err = tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO transactions (
				id, hash, date, name, merchant_name, amount, 
				plaid_categories, account_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`)
	}

	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, txn := range transactions {
		// Generate hash if not already set
		if txn.Hash == "" {
			txn.Hash = txn.GenerateHash()
		}

		// Convert categories slice to JSON string
		categoriesJSON := ""
		if len(txn.Category) > 0 {
			categoriesBytes, marshalErr := json.Marshal(txn.Category)
			if marshalErr == nil {
				categoriesJSON = string(categoriesBytes)
			}
		}

		switch {
		case schemaVersion >= 7:
			_, err = stmt.ExecContext(ctx,
				txn.ID,
				txn.Hash,
				txn.Date,
				txn.Name,
				txn.MerchantName,
				txn.Amount,
				categoriesJSON,
				txn.AccountID,
				txn.Type,
				txn.CheckNumber,
				string(txn.Direction),
			)
		case schemaVersion >= 5:
			_, err = stmt.ExecContext(ctx,
				txn.ID,
				txn.Hash,
				txn.Date,
				txn.Name,
				txn.MerchantName,
				txn.Amount,
				categoriesJSON,
				txn.AccountID,
				txn.Type,
				txn.CheckNumber,
			)
		default:
			// For old schema, just use categories as plaid_categories
			_, err = stmt.ExecContext(ctx,
				txn.ID,
				txn.Hash,
				txn.Date,
				txn.Name,
				txn.MerchantName,
				txn.Amount,
				categoriesJSON,
				txn.AccountID,
			)
		}
		if err != nil {
			return fmt.Errorf("failed to insert transaction %s: %w", txn.ID, err)
		}
	}

	return nil
}

// GetTransactionsToClassify retrieves unclassified transactions.
func (s *SQLiteStorage) GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return s.getTransactionsToClassifyTx(ctx, s.db, fromDate)
}

func (s *SQLiteStorage) getTransactionsToClassifyTx(ctx context.Context, q queryable, fromDate *time.Time) ([]model.Transaction, error) {
	// Check schema version
	var schemaVersion int
	err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema version: %w", err)
	}

	// Build query based on schema version
	var query string
	if schemaVersion >= 5 {
		query = `
			SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
			       t.amount, t.categories, t.account_id, 
			       t.transaction_type, t.check_number
			FROM transactions t
			LEFT JOIN classifications c ON t.id = c.transaction_id
			WHERE c.transaction_id IS NULL
		`
	} else {
		query = `
			SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
			       t.amount, t.plaid_categories, t.account_id
			FROM transactions t
			LEFT JOIN classifications c ON t.id = c.transaction_id
			WHERE c.transaction_id IS NULL
		`
	}

	args := []any{}

	if fromDate != nil {
		query += " AND t.date > ?"
		args = append(args, *fromDate)
	}

	query += " ORDER BY t.date ASC"

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transactions []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		var categoriesJSON sql.NullString
		var txType sql.NullString
		var checkNum sql.NullString

		if schemaVersion >= 5 {
			err := rows.Scan(
				&txn.ID,
				&txn.Hash,
				&txn.Date,
				&txn.Name,
				&txn.MerchantName,
				&txn.Amount,
				&categoriesJSON,
				&txn.AccountID,
				&txType,
				&checkNum,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan transaction: %w", err)
			}

			// Parse categories JSON
			if categoriesJSON.Valid && categoriesJSON.String != "" {
				if err := json.Unmarshal([]byte(categoriesJSON.String), &txn.Category); err != nil {
					// Log but don't fail on JSON parse error
					slog.Warn("Failed to parse categories JSON", "error", err, "json", categoriesJSON.String)
				}
			}

			// Set type and check number
			if txType.Valid {
				txn.Type = txType.String
			}
			if checkNum.Valid {
				txn.CheckNumber = checkNum.String
			}
		} else {
			// Old schema
			err := rows.Scan(
				&txn.ID,
				&txn.Hash,
				&txn.Date,
				&txn.Name,
				&txn.MerchantName,
				&txn.Amount,
				&categoriesJSON,
				&txn.AccountID,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan transaction: %w", err)
			}

			// Parse categories from old plaid_categories column
			if categoriesJSON.Valid && categoriesJSON.String != "" {
				// Try to parse as JSON array first
				if err := json.Unmarshal([]byte(categoriesJSON.String), &txn.Category); err != nil {
					// If that fails, treat as comma-separated string
					txn.Category = strings.Split(categoriesJSON.String, ",")
					for i := range txn.Category {
						txn.Category[i] = strings.TrimSpace(txn.Category[i])
					}
				}
			}
		}

		transactions = append(transactions, txn)
	}

	return transactions, rows.Err()
}

// GetTransactionByID retrieves a single transaction by ID.
func (s *SQLiteStorage) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(id, "id"); err != nil {
		return nil, err
	}
	return s.getTransactionByIDTx(ctx, s.db, id)
}

func (s *SQLiteStorage) getTransactionByIDTx(ctx context.Context, q queryable, id string) (*model.Transaction, error) {
	var txn model.Transaction
	var categories sql.NullString

	err := q.QueryRowContext(ctx, `
		SELECT id, hash, date, name, merchant_name, 
		       amount, categories, account_id
		FROM transactions
		WHERE id = ?
	`, id).Scan(
		&txn.ID,
		&txn.Hash,
		&txn.Date,
		&txn.Name,
		&txn.MerchantName,
		&txn.Amount,
		&categories,
		&txn.AccountID,
	)

	if err == sql.ErrNoRows {
		return nil, common.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Parse categories JSON
	if categories.Valid && categories.String != "" {
		if err := json.Unmarshal([]byte(categories.String), &txn.Category); err != nil {
			return nil, fmt.Errorf("failed to parse categories: %w", err)
		}
	}

	return &txn, nil
}

// GetTransactionsByCategory retrieves all transactions for a specific category.
func (s *SQLiteStorage) GetTransactionsByCategory(ctx context.Context, categoryName string) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(categoryName, "categoryName"); err != nil {
		return nil, err
	}
	return s.getTransactionsByCategoryTx(ctx, s.db, categoryName)
}

func (s *SQLiteStorage) getTransactionsByCategoryTx(ctx context.Context, q queryable, categoryName string) ([]model.Transaction, error) {
	// Check schema version
	var schemaVersion int
	err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema version: %w", err)
	}

	// Build query based on schema version
	var query string
	switch {
	case schemaVersion >= 7:
		// Schema with direction field
		query = `
			SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
			       t.amount, t.categories, t.account_id, 
			       t.transaction_type, t.check_number, t.direction
			FROM transactions t
			JOIN classifications c ON t.id = c.transaction_id
			WHERE c.category_name = ?
			ORDER BY t.date DESC
		`
	case schemaVersion >= 5:
		query = `
			SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
			       t.amount, t.categories, t.account_id, 
			       t.transaction_type, t.check_number
			FROM transactions t
			JOIN classifications c ON t.id = c.transaction_id
			WHERE c.category_name = ?
			ORDER BY t.date DESC
		`
	default:
		query = `
			SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
			       t.amount, t.plaid_categories, t.account_id
			FROM transactions t
			JOIN classifications c ON t.id = c.transaction_id
			WHERE c.category_name = ?
			ORDER BY t.date DESC
		`
	}

	rows, err := q.QueryContext(ctx, query, categoryName)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return s.scanTransactions(ctx, rows, schemaVersion)
}

// GetTransactionsByCategoryID retrieves all transactions for a specific category ID.
func (s *SQLiteStorage) GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	// First get the category name from ID
	var categoryName string
	err := s.db.QueryRowContext(ctx, `
		SELECT name FROM categories WHERE id = ?
	`, categoryID).Scan(&categoryName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category with ID %d not found", categoryID)
		}
		return nil, fmt.Errorf("failed to get category name: %w", err)
	}

	return s.GetTransactionsByCategory(ctx, categoryName)
}

// UpdateTransactionCategories updates all transactions from one category to another.
func (s *SQLiteStorage) UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateString(fromCategory, "fromCategory"); err != nil {
		return err
	}
	if err := validateString(toCategory, "toCategory"); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify toCategory exists
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND is_active = 1)
	`, toCategory).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check category existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("category '%s' does not exist", toCategory)
	}

	// Update classifications
	_, err = tx.ExecContext(ctx, `
		UPDATE classifications 
		SET category_name = ?, updated_at = ?
		WHERE category_name = ?
	`, toCategory, time.Now(), fromCategory)
	if err != nil {
		return fmt.Errorf("failed to update classifications: %w", err)
	}

	return tx.Commit()
}

// UpdateTransactionCategoriesByID updates all transactions from one category ID to another.
func (s *SQLiteStorage) UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	// Get category names from IDs
	var fromCategory, toCategory string
	err := s.db.QueryRowContext(ctx, `
		SELECT name FROM categories WHERE id = ?
	`, fromCategoryID).Scan(&fromCategory)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("category with ID %d not found", fromCategoryID)
		}
		return fmt.Errorf("failed to get from category name: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT name FROM categories WHERE id = ?
	`, toCategoryID).Scan(&toCategory)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("category with ID %d not found", toCategoryID)
		}
		return fmt.Errorf("failed to get to category name: %w", err)
	}

	return s.UpdateTransactionCategories(ctx, fromCategory, toCategory)
}

// GetTransactionCount returns the total number of transactions.
func (s *SQLiteStorage) GetTransactionCount(ctx context.Context) (int, error) {
	if err := validateContext(ctx); err != nil {
		return 0, err
	}

	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM transactions
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction count: %w", err)
	}

	return count, nil
}

// GetTransactionCountByCategory returns the number of transactions for a specific category.
func (s *SQLiteStorage) GetTransactionCountByCategory(ctx context.Context, categoryName string) (int, error) {
	if err := validateContext(ctx); err != nil {
		return 0, err
	}
	if err := validateString(categoryName, "categoryName"); err != nil {
		return 0, err
	}

	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM transactions t
		JOIN classifications c ON t.id = c.transaction_id
		WHERE c.category_name = ?
	`, categoryName).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction count by category: %w", err)
	}

	return count, nil
}

// GetEarliestTransactionDate returns the date of the earliest transaction.
func (s *SQLiteStorage) GetEarliestTransactionDate(ctx context.Context) (time.Time, error) {
	if err := validateContext(ctx); err != nil {
		return time.Time{}, err
	}

	var date time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT MIN(date) FROM transactions
	`).Scan(&date)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, common.ErrNotFound
		}
		return time.Time{}, fmt.Errorf("failed to get earliest transaction date: %w", err)
	}

	return date, nil
}

// GetLatestTransactionDate returns the date of the latest transaction.
func (s *SQLiteStorage) GetLatestTransactionDate(ctx context.Context) (time.Time, error) {
	if err := validateContext(ctx); err != nil {
		return time.Time{}, err
	}

	var date time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(date) FROM transactions
	`).Scan(&date)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, common.ErrNotFound
		}
		return time.Time{}, fmt.Errorf("failed to get latest transaction date: %w", err)
	}

	return date, nil
}

// GetCategorySummary returns a summary of transaction amounts by category for a date range.
func (s *SQLiteStorage) GetCategorySummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.category_name, SUM(t.amount) as total
		FROM transactions t
		JOIN classifications c ON t.id = c.transaction_id
		WHERE t.date >= ? AND t.date <= ?
		GROUP BY c.category_name
		ORDER BY total DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query category summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	summary := make(map[string]float64)
	for rows.Next() {
		var category string
		var total float64
		if err := rows.Scan(&category, &total); err != nil {
			return nil, fmt.Errorf("failed to scan category summary: %w", err)
		}
		summary[category] = total
	}

	return summary, rows.Err()
}

// GetMerchantSummary returns a summary of transaction amounts by merchant for a date range.
func (s *SQLiteStorage) GetMerchantSummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT merchant_name, SUM(amount) as total
		FROM transactions
		WHERE date >= ? AND date <= ? AND merchant_name != ''
		GROUP BY merchant_name
		ORDER BY total DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query merchant summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	summary := make(map[string]float64)
	for rows.Next() {
		var merchant string
		var total float64
		if err := rows.Scan(&merchant, &total); err != nil {
			return nil, fmt.Errorf("failed to scan merchant summary: %w", err)
		}
		summary[merchant] = total
	}

	return summary, rows.Err()
}

// scanTransactions is a helper method to scan transaction rows based on schema version.
func (s *SQLiteStorage) scanTransactions(_ context.Context, rows *sql.Rows, schemaVersion int) ([]model.Transaction, error) {
	var transactions []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		var categoriesJSON sql.NullString
		var txType sql.NullString
		var checkNum sql.NullString
		var direction sql.NullString

		switch {
		case schemaVersion >= 7:
			// Schema with direction field
			err := rows.Scan(
				&txn.ID,
				&txn.Hash,
				&txn.Date,
				&txn.Name,
				&txn.MerchantName,
				&txn.Amount,
				&categoriesJSON,
				&txn.AccountID,
				&txType,
				&checkNum,
				&direction,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan transaction: %w", err)
			}

			if direction.Valid {
				txn.Direction = model.TransactionDirection(direction.String)
			}
		case schemaVersion >= 5:
			err := rows.Scan(
				&txn.ID,
				&txn.Hash,
				&txn.Date,
				&txn.Name,
				&txn.MerchantName,
				&txn.Amount,
				&categoriesJSON,
				&txn.AccountID,
				&txType,
				&checkNum,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan transaction: %w", err)
			}
		default:
			// Old schema
			err := rows.Scan(
				&txn.ID,
				&txn.Hash,
				&txn.Date,
				&txn.Name,
				&txn.MerchantName,
				&txn.Amount,
				&categoriesJSON,
				&txn.AccountID,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan transaction: %w", err)
			}
		}

		// Parse categories JSON
		if categoriesJSON.Valid && categoriesJSON.String != "" {
			if err := json.Unmarshal([]byte(categoriesJSON.String), &txn.Category); err != nil {
				// Log but don't fail on JSON parse error
				slog.Warn("Failed to parse categories JSON", "error", err, "json", categoriesJSON.String)
			}
		}

		// Set type and check number
		if txType.Valid {
			txn.Type = txType.String
		}
		if checkNum.Valid {
			txn.CheckNumber = checkNum.String
		}

		transactions = append(transactions, txn)
	}

	return transactions, rows.Err()
}

// queryable is an interface satisfied by both *sql.DB and *sql.Tx.
type queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
