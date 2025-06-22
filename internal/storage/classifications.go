// Package storage provides the data persistence layer for the spice application.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// SaveClassification saves a classification for a transaction.
func (s *SQLiteStorage) SaveClassification(ctx context.Context, classification *model.Classification) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateClassification(classification); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.saveClassificationTx(ctx, tx, classification); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStorage) saveClassificationTx(ctx context.Context, tx *sql.Tx, classification *model.Classification) error {
	// Set ClassifiedAt if not set
	if classification.ClassifiedAt.IsZero() {
		classification.ClassifiedAt = time.Now()
	}

	// Validate category exists (only if status is not unclassified)
	if classification.Status != model.StatusUnclassified && classification.Category != "" {
		var categoryExists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND is_active = 1)
		`, classification.Category).Scan(&categoryExists)

		if err != nil {
			return fmt.Errorf("failed to check category existence: %w", err)
		}

		if !categoryExists {
			return fmt.Errorf("category '%s' does not exist", classification.Category)
		}
	}

	// Insert classification
	_, err := tx.ExecContext(ctx, `
		INSERT INTO classifications (
			transaction_id, category, status, confidence, 
			classified_at, notes
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(transaction_id) DO UPDATE SET
			category = excluded.category,
			status = excluded.status,
			confidence = excluded.confidence,
			classified_at = excluded.classified_at,
			notes = excluded.notes
	`,
		classification.Transaction.ID,
		classification.Category,
		string(classification.Status),
		classification.Confidence,
		classification.ClassifiedAt,
		classification.Notes,
	)

	if err != nil {
		return fmt.Errorf("failed to save classification: %w", err)
	}

	// Add to history for auditing
	_, err = tx.ExecContext(ctx, `
		INSERT INTO classification_history (
			transaction_id, category, status, confidence
		) VALUES (?, ?, ?, ?)
	`,
		classification.Transaction.ID,
		classification.Category,
		string(classification.Status),
		classification.Confidence,
	)

	if err != nil {
		return fmt.Errorf("failed to save classification history: %w", err)
	}

	// If this is a user-modified or rule-based classification, create/update vendor rule
	if (classification.Status == model.StatusUserModified || classification.Status == model.StatusClassifiedByRule) && classification.Transaction.MerchantName != "" {
		// Create a transaction wrapper to use vendor methods
		txWrapper := &sqliteTransaction{tx: tx, storage: s}

		// Check if vendor exists
		vendor, err := txWrapper.GetVendor(ctx, classification.Transaction.MerchantName)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to check vendor: %w", err)
		}

		if vendor == nil {
			// Create new vendor
			vendor = &model.Vendor{
				Name:     classification.Transaction.MerchantName,
				Category: classification.Category,
				UseCount: 1,
			}
		} else {
			// Update existing vendor
			vendor.Category = classification.Category
			vendor.UseCount++
		}

		if err := txWrapper.SaveVendor(ctx, vendor); err != nil {
			return fmt.Errorf("failed to save vendor rule: %w", err)
		}
	}

	return nil
}

// GetClassificationsByDateRange retrieves classifications within a date range.
func (s *SQLiteStorage) GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, fmt.Errorf("%w: end date %v is before start date %v", ErrInvalidDateRange, end, start)
	}
	return s.getClassificationsByDateRangeTx(ctx, s.db, start, end)
}

func (s *SQLiteStorage) getClassificationsByDateRangeTx(ctx context.Context, q queryable, start, end time.Time) ([]model.Classification, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT 
			t.id, t.hash, t.date, t.name, t.merchant_name,
			t.amount, t.categories, t.account_id,
			t.transaction_type, t.check_number,
			c.category, c.status, c.confidence, c.classified_at, c.notes
		FROM classifications c
		JOIN transactions t ON c.transaction_id = t.id
		WHERE t.date >= ? AND t.date <= ?
		ORDER BY t.date
	`, start, end)

	if err != nil {
		return nil, fmt.Errorf("failed to query classifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var classifications []model.Classification
	for rows.Next() {
		var c model.Classification
		var statusStr string
		var categories sql.NullString
		var txType sql.NullString
		var checkNum sql.NullString

		err := rows.Scan(
			&c.Transaction.ID,
			&c.Transaction.Hash,
			&c.Transaction.Date,
			&c.Transaction.Name,
			&c.Transaction.MerchantName,
			&c.Transaction.Amount,
			&categories,
			&c.Transaction.AccountID,
			&txType,
			&checkNum,
			&c.Category,
			&statusStr,
			&c.Confidence,
			&c.ClassifiedAt,
			&c.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan classification: %w", err)
		}

		c.Status = model.ClassificationStatus(statusStr)

		// Parse categories JSON
		if categories.Valid && categories.String != "" {
			if err := json.Unmarshal([]byte(categories.String), &c.Transaction.Category); err != nil {
				return nil, fmt.Errorf("failed to parse categories: %w", err)
			}
		}

		// Set transaction type and check number
		if txType.Valid {
			c.Transaction.Type = txType.String
		}
		if checkNum.Valid {
			c.Transaction.CheckNumber = checkNum.String
		}

		classifications = append(classifications, c)
	}

	return classifications, rows.Err()
}

// SaveProgress saves classification progress.
func (s *SQLiteStorage) SaveProgress(ctx context.Context, progress *model.ClassificationProgress) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateProgress(progress); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.saveProgressTx(ctx, tx, progress); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStorage) saveProgressTx(ctx context.Context, tx *sql.Tx, progress *model.ClassificationProgress) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO progress (
			last_processed_id, last_processed_date, 
			total_processed, started_at, updated_at
		) VALUES (?, ?, ?, ?, ?)
	`,
		progress.LastProcessedID,
		progress.LastProcessedDate,
		progress.TotalProcessed,
		progress.StartedAt,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	return nil
}

// GetLatestProgress retrieves the most recent progress record.
func (s *SQLiteStorage) GetLatestProgress(ctx context.Context) (*model.ClassificationProgress, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return s.getLatestProgressTx(ctx, s.db)
}

func (s *SQLiteStorage) getLatestProgressTx(ctx context.Context, q queryable) (*model.ClassificationProgress, error) {
	var progress model.ClassificationProgress

	err := q.QueryRowContext(ctx, `
		SELECT last_processed_id, last_processed_date, 
		       total_processed, started_at
		FROM progress
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&progress.LastProcessedID,
		&progress.LastProcessedDate,
		&progress.TotalProcessed,
		&progress.StartedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows // No progress found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get progress: %w", err)
	}

	return &progress, nil
}

// ClearProgress removes all progress records.
func (s *SQLiteStorage) ClearProgress(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM progress")
	if err != nil {
		return fmt.Errorf("failed to clear progress: %w", err)
	}
	return nil
}
