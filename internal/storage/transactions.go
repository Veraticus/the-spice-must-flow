package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/common"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
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
	if schemaVersion >= 5 {
		// New schema with generic fields
		stmt, err = tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO transactions (
				id, hash, date, name, merchant_name, amount, 
				categories, account_id, transaction_type, check_number
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	} else {
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
			categoriesBytes, err := json.Marshal(txn.Category)
			if err == nil {
				categoriesJSON = string(categoriesBytes)
			}
		}

		if schemaVersion >= 5 {
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
		} else {
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

	err := q.QueryRowContext(ctx, `
		SELECT id, hash, date, name, merchant_name, 
		       amount, plaid_categories, account_id
		FROM transactions
		WHERE id = ?
	`, id).Scan(
		&txn.ID,
		&txn.Hash,
		&txn.Date,
		&txn.Name,
		&txn.MerchantName,
		&txn.Amount,
		&txn.PlaidCategory,
		&txn.AccountID,
	)

	if err == sql.ErrNoRows {
		return nil, common.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return &txn, nil
}

// queryable is an interface satisfied by both *sql.DB and *sql.Tx.
type queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
