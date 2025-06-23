package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
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
	// Always use the latest schema
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO transactions (
			id, hash, date, name, merchant_name, amount, 
			categories, account_id, transaction_type, check_number,
			direction, is_refund, refund_category
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
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

		// Always use the latest schema
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
			txn.Direction,
			txn.IsRefund,
			txn.RefundCategory,
		)
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
	// Always use the latest schema
	query := `
		SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
		       t.amount, t.categories, t.account_id, 
		       t.transaction_type, t.check_number,
		       t.direction, t.is_refund, t.refund_category
		FROM transactions t
		LEFT JOIN classifications c ON t.id = c.transaction_id
		WHERE c.transaction_id IS NULL
	`

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
		var direction sql.NullString
		var isRefund sql.NullBool
		var refundCategory sql.NullString

		// Always use the latest schema
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
			&isRefund,
			&refundCategory,
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

		// Set direction, refund fields
		if direction.Valid && direction.String != "" {
			txn.Direction = model.TransactionDirection(direction.String)
		}
		if isRefund.Valid {
			txn.IsRefund = isRefund.Bool
		}
		if refundCategory.Valid {
			txn.RefundCategory = refundCategory.String
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
	var txType sql.NullString
	var checkNum sql.NullString

	var direction sql.NullString
	var isRefund sql.NullBool
	var refundCategory sql.NullString

	err := q.QueryRowContext(ctx, `
		SELECT id, hash, date, name, merchant_name, 
		       amount, categories, account_id, transaction_type, check_number,
		       direction, is_refund, refund_category
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
		&txType,
		&checkNum,
		&direction,
		&isRefund,
		&refundCategory,
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

	// Set type and check number
	if txType.Valid {
		txn.Type = txType.String
	}
	if checkNum.Valid {
		txn.CheckNumber = checkNum.String
	}

	// Set direction, refund fields
	if direction.Valid && direction.String != "" {
		txn.Direction = model.TransactionDirection(direction.String)
	}
	if isRefund.Valid {
		txn.IsRefund = isRefund.Bool
	}
	if refundCategory.Valid {
		txn.RefundCategory = refundCategory.String
	}

	return &txn, nil
}

// queryable is an interface satisfied by both *sql.DB and *sql.Tx.
type queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// GetTransactionsByCategory returns all transactions with the specified category.
func (s *SQLiteStorage) GetTransactionsByCategory(ctx context.Context, category string) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT t.id, t.account_id, t.name, t.merchant_name, 
		       t.amount, t.date, t.categories, 
		       t.transaction_type, t.check_number
		FROM transactions t
		INNER JOIN classifications c ON t.id = c.transaction_id
		WHERE c.category = ? AND c.status != 'unclassified'
		ORDER BY t.date DESC
	`

	rows, err := s.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by category: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("Failed to close rows", "error", err)
		}
	}()

	var transactions []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		var categoriesJSON sql.NullString

		err := rows.Scan(
			&txn.ID,
			&txn.AccountID,
			&txn.Name,
			&txn.MerchantName,
			&txn.Amount,
			&txn.Date,
			&categoriesJSON,
			&txn.Type,
			&txn.CheckNumber,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}

		// Parse categories JSON
		if categoriesJSON.Valid {
			if err := json.Unmarshal([]byte(categoriesJSON.String), &txn.Category); err != nil {
				slog.Warn("Failed to parse categories JSON", "error", err, "transaction_id", txn.ID)
			}
		}

		transactions = append(transactions, txn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transaction rows: %w", err)
	}

	return transactions, nil
}

// UpdateTransactionCategories updates all transactions from one category to another.
func (s *SQLiteStorage) UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if fromCategory == "" || toCategory == "" {
		return fmt.Errorf("both fromCategory and toCategory must be provided")
	}

	query := `
		UPDATE classifications 
		SET category = ?
		WHERE category = ? AND status != 'unclassified'
	`

	result, err := s.db.ExecContext(ctx, query, toCategory, fromCategory)
	if err != nil {
		return fmt.Errorf("failed to update transaction categories: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	slog.Info("Updated transaction categories",
		"from", fromCategory,
		"to", toCategory,
		"transactions_updated", rowsAffected)

	return nil
}

// GetTransactionsByCategoryID returns all transactions with the specified category ID.
func (s *SQLiteStorage) GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if categoryID <= 0 {
		return nil, fmt.Errorf("invalid category ID: %d", categoryID)
	}

	// First get the category name
	category, err := s.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	// Then use the existing method
	return s.GetTransactionsByCategory(ctx, category.Name)
}

// UpdateTransactionCategoriesByID updates all transactions from one category ID to another.
func (s *SQLiteStorage) UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if fromCategoryID <= 0 || toCategoryID <= 0 {
		return fmt.Errorf("invalid category IDs: from=%d, to=%d", fromCategoryID, toCategoryID)
	}

	// Get both categories
	fromCategory, err := s.GetCategoryByID(ctx, fromCategoryID)
	if err != nil {
		return fmt.Errorf("failed to get source category: %w", err)
	}

	toCategory, err := s.GetCategoryByID(ctx, toCategoryID)
	if err != nil {
		return fmt.Errorf("failed to get target category: %w", err)
	}

	// Use the existing method
	return s.UpdateTransactionCategories(ctx, fromCategory.Name, toCategory.Name)
}

// GetTransactions retrieves transactions based on the provided filter.
func (s *SQLiteStorage) GetTransactions(ctx context.Context, filter service.TransactionFilter) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return s.getTransactionsTx(ctx, s.db, filter)
}

func (s *SQLiteStorage) getTransactionsTx(ctx context.Context, q queryable, filter service.TransactionFilter) ([]model.Transaction, error) {
	query := `
		SELECT id, hash, date, name, merchant_name, 
		       amount, categories, account_id, 
		       transaction_type, check_number,
		       direction, is_refund, refund_category
		FROM transactions
		WHERE 1=1
	`

	args := []any{}

	// Apply date filters
	if filter.StartDate != nil {
		query += " AND date >= ?"
		args = append(args, *filter.StartDate)
	}
	if filter.EndDate != nil {
		query += " AND date <= ?"
		args = append(args, *filter.EndDate)
	}

	query += " ORDER BY date ASC"

	// Apply limit and offset
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)

		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

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
		var direction sql.NullString
		var isRefund sql.NullBool
		var refundCategory sql.NullString

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
			&isRefund,
			&refundCategory,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Parse categories JSON
		if categoriesJSON.Valid && categoriesJSON.String != "" {
			if err := json.Unmarshal([]byte(categoriesJSON.String), &txn.Category); err != nil {
				slog.Warn("Failed to parse categories JSON", "error", err, "json", categoriesJSON.String)
			}
		}

		// Set optional fields
		if txType.Valid {
			txn.Type = txType.String
		}
		if checkNum.Valid {
			txn.CheckNumber = checkNum.String
		}
		if direction.Valid && direction.String != "" {
			txn.Direction = model.TransactionDirection(direction.String)
		}
		if isRefund.Valid {
			txn.IsRefund = isRefund.Bool
		}
		if refundCategory.Valid {
			txn.RefundCategory = refundCategory.String
		}

		transactions = append(transactions, txn)
	}

	return transactions, rows.Err()
}

// UpdateTransactionDirection updates the direction of a single transaction.
func (s *SQLiteStorage) UpdateTransactionDirection(ctx context.Context, transactionID string, direction model.TransactionDirection) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateString(transactionID, "transactionID"); err != nil {
		return err
	}
	return s.updateTransactionDirectionTx(ctx, s.db, transactionID, direction)
}

func (s *SQLiteStorage) updateTransactionDirectionTx(ctx context.Context, q queryable, transactionID string, direction model.TransactionDirection) error {
	// Validate direction
	switch direction {
	case model.DirectionIncome, model.DirectionExpense, model.DirectionTransfer:
		// Valid
	default:
		return fmt.Errorf("invalid transaction direction: %s", direction)
	}

	result, err := q.ExecContext(ctx, `
		UPDATE transactions 
		SET direction = ?
		WHERE id = ?
	`, direction, transactionID)

	if err != nil {
		return fmt.Errorf("failed to update transaction direction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return common.ErrNotFound
	}

	return nil
}

// GetIncomeByPeriod retrieves income transactions for a specific period.
func (s *SQLiteStorage) GetIncomeByPeriod(ctx context.Context, start, end time.Time) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, hash, date, name, merchant_name, 
		       amount, categories, account_id, 
		       transaction_type, check_number,
		       direction, is_refund, refund_category
		FROM transactions
		WHERE direction = ? AND date >= ? AND date <= ?
		ORDER BY date ASC
	`

	rows, err := s.db.QueryContext(ctx, query, model.DirectionIncome, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query income transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return s.scanTransactions(rows)
}

// GetExpensesByPeriod retrieves expense transactions for a specific period.
func (s *SQLiteStorage) GetExpensesByPeriod(ctx context.Context, start, end time.Time) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, hash, date, name, merchant_name, 
		       amount, categories, account_id, 
		       transaction_type, check_number,
		       direction, is_refund, refund_category
		FROM transactions
		WHERE direction = ? AND date >= ? AND date <= ?
		ORDER BY date ASC
	`

	rows, err := s.db.QueryContext(ctx, query, model.DirectionExpense, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query expense transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return s.scanTransactions(rows)
}

// GetCashFlow calculates cash flow summary for a specific period.
func (s *SQLiteStorage) GetCashFlow(ctx context.Context, _, _ time.Time) (*service.CashFlowSummary, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	// This method would typically be more efficient with direct SQL aggregations,
	// but for now we'll return nil to let the caller use the fallback calculation
	return nil, fmt.Errorf("GetCashFlow not implemented - use calculateCashFlowSummary")
}

// scanTransactions is a helper to scan transaction rows.
func (s *SQLiteStorage) scanTransactions(rows *sql.Rows) ([]model.Transaction, error) {
	var transactions []model.Transaction

	for rows.Next() {
		var txn model.Transaction
		var categoriesJSON sql.NullString
		var txType sql.NullString
		var checkNum sql.NullString
		var direction sql.NullString
		var isRefund sql.NullBool
		var refundCategory sql.NullString

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
			&isRefund,
			&refundCategory,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Parse categories JSON
		if categoriesJSON.Valid && categoriesJSON.String != "" {
			if err := json.Unmarshal([]byte(categoriesJSON.String), &txn.Category); err != nil {
				slog.Warn("Failed to parse categories JSON", "error", err, "json", categoriesJSON.String)
			}
		}

		// Set optional fields
		if txType.Valid {
			txn.Type = txType.String
		}
		if checkNum.Valid {
			txn.CheckNumber = checkNum.String
		}
		if direction.Valid && direction.String != "" {
			txn.Direction = model.TransactionDirection(direction.String)
		}
		if isRefund.Valid {
			txn.IsRefund = isRefund.Bool
		}
		if refundCategory.Valid {
			txn.RefundCategory = refundCategory.String
		}

		transactions = append(transactions, txn)
	}

	return transactions, rows.Err()
}
