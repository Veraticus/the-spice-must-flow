package storage

import (
	"context"
	"database/sql"
	"fmt"
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
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO transactions (
			id, hash, date, name, merchant_name, amount, 
			plaid_categories, account_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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

		_, err = stmt.ExecContext(ctx,
			txn.ID,
			txn.Hash,
			txn.Date,
			txn.Name,
			txn.MerchantName,
			txn.Amount,
			txn.PlaidCategory,
			txn.AccountID,
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
	query := `
		SELECT t.id, t.hash, t.date, t.name, t.merchant_name, 
		       t.amount, t.plaid_categories, t.account_id
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

		err := rows.Scan(
			&txn.ID,
			&txn.Hash,
			&txn.Date,
			&txn.Name,
			&txn.MerchantName,
			&txn.Amount,
			&txn.PlaidCategory,
			&txn.AccountID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
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
