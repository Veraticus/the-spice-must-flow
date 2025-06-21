package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// ErrCheckPatternNotFound is returned when a check pattern is not found.
var ErrCheckPatternNotFound = errors.New("check pattern not found")

// CreateCheckPattern creates a new check pattern.
func (s *SQLiteStorage) CreateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if pattern == nil {
		return fmt.Errorf("pattern cannot be nil")
	}

	if err := pattern.Validate(); err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	// Convert CheckNumberPattern to JSON if present
	var checkNumberJSON *string
	if pattern.CheckNumberPattern != nil {
		data, err := json.Marshal(pattern.CheckNumberPattern)
		if err != nil {
			return fmt.Errorf("failed to marshal check number pattern: %w", err)
		}
		str := string(data)
		checkNumberJSON = &str
	}

	query := `
		INSERT INTO check_patterns (
			pattern_name, amount_min, amount_max, check_number_pattern,
			day_of_month_min, day_of_month_max, category, notes,
			confidence_boost, active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		pattern.PatternName, pattern.AmountMin, pattern.AmountMax, checkNumberJSON,
		pattern.DayOfMonthMin, pattern.DayOfMonthMax, pattern.Category, pattern.Notes,
		pattern.ConfidenceBoost, pattern.Active,
	)

	if err != nil {
		return fmt.Errorf("failed to create check pattern: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	pattern.ID = id
	slog.Info("created check pattern", "id", id, "name", pattern.PatternName)
	return nil
}

// GetCheckPattern retrieves a check pattern by ID.
func (s *SQLiteStorage) GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, pattern_name, amount_min, amount_max, check_number_pattern,
			day_of_month_min, day_of_month_max, category, notes,
			confidence_boost, active, use_count, created_at, updated_at
		FROM check_patterns
		WHERE id = ?`

	pattern := &model.CheckPattern{}
	var checkNumberJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&pattern.ID, &pattern.PatternName, &pattern.AmountMin, &pattern.AmountMax,
		&checkNumberJSON, &pattern.DayOfMonthMin, &pattern.DayOfMonthMax,
		&pattern.Category, &pattern.Notes, &pattern.ConfidenceBoost,
		&pattern.Active, &pattern.UseCount, &pattern.CreatedAt, &pattern.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCheckPatternNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query check pattern: %w", err)
	}

	// Parse check number pattern if present
	if checkNumberJSON.Valid {
		var matcher model.CheckNumberMatcher
		if err := json.Unmarshal([]byte(checkNumberJSON.String), &matcher); err != nil {
			return nil, fmt.Errorf("failed to unmarshal check number pattern: %w", err)
		}
		pattern.CheckNumberPattern = &matcher
	}

	return pattern, nil
}

// GetActiveCheckPatterns returns all active check patterns.
func (s *SQLiteStorage) GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, pattern_name, amount_min, amount_max, check_number_pattern,
			day_of_month_min, day_of_month_max, category, notes,
			confidence_boost, active, use_count, created_at, updated_at
		FROM check_patterns
		WHERE active = 1
		ORDER BY use_count DESC, pattern_name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query check patterns: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var patterns []model.CheckPattern
	for rows.Next() {
		var pattern model.CheckPattern
		var checkNumberJSON sql.NullString

		if err := rows.Scan(
			&pattern.ID, &pattern.PatternName, &pattern.AmountMin, &pattern.AmountMax,
			&checkNumberJSON, &pattern.DayOfMonthMin, &pattern.DayOfMonthMax,
			&pattern.Category, &pattern.Notes, &pattern.ConfidenceBoost,
			&pattern.Active, &pattern.UseCount, &pattern.CreatedAt, &pattern.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan check pattern: %w", err)
		}

		// Parse check number pattern if present
		if checkNumberJSON.Valid {
			var matcher model.CheckNumberMatcher
			if err := json.Unmarshal([]byte(checkNumberJSON.String), &matcher); err != nil {
				return nil, fmt.Errorf("failed to unmarshal check number pattern: %w", err)
			}
			pattern.CheckNumberPattern = &matcher
		}

		patterns = append(patterns, pattern)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating check patterns: %w", err)
	}

	slog.Debug("retrieved check patterns", "count", len(patterns))
	return patterns, nil
}

// GetMatchingCheckPatterns returns all active patterns that match the given transaction.
func (s *SQLiteStorage) GetMatchingCheckPatterns(ctx context.Context, txn model.Transaction) ([]model.CheckPattern, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	// First get all active patterns
	patterns, err := s.GetActiveCheckPatterns(ctx)
	if err != nil {
		return nil, err
	}

	// Filter patterns that match the transaction
	var matching []model.CheckPattern
	for _, pattern := range patterns {
		if pattern.Matches(txn) {
			matching = append(matching, pattern)
		}
	}

	slog.Debug("found matching check patterns", "transaction_id", txn.ID, "count", len(matching))
	return matching, nil
}

// UpdateCheckPattern updates an existing check pattern.
func (s *SQLiteStorage) UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if pattern == nil {
		return fmt.Errorf("pattern cannot be nil")
	}

	if err := pattern.Validate(); err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	// Convert CheckNumberPattern to JSON if present
	var checkNumberJSON *string
	if pattern.CheckNumberPattern != nil {
		data, err := json.Marshal(pattern.CheckNumberPattern)
		if err != nil {
			return fmt.Errorf("failed to marshal check number pattern: %w", err)
		}
		str := string(data)
		checkNumberJSON = &str
	}

	query := `
		UPDATE check_patterns SET
			pattern_name = ?, amount_min = ?, amount_max = ?, 
			check_number_pattern = ?, day_of_month_min = ?, 
			day_of_month_max = ?, category = ?, notes = ?,
			confidence_boost = ?, active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query,
		pattern.PatternName, pattern.AmountMin, pattern.AmountMax, checkNumberJSON,
		pattern.DayOfMonthMin, pattern.DayOfMonthMax, pattern.Category, pattern.Notes,
		pattern.ConfidenceBoost, pattern.Active, pattern.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update check pattern: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrCheckPatternNotFound
	}

	slog.Info("updated check pattern", "id", pattern.ID, "name", pattern.PatternName)
	return nil
}

// DeleteCheckPattern performs a soft delete on a check pattern.
func (s *SQLiteStorage) DeleteCheckPattern(ctx context.Context, id int64) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	query := `
		UPDATE check_patterns 
		SET active = 0, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete check pattern: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrCheckPatternNotFound
	}

	slog.Info("deleted check pattern", "id", id)
	return nil
}

// IncrementCheckPatternUseCount increments the use count for a check pattern.
func (s *SQLiteStorage) IncrementCheckPatternUseCount(ctx context.Context, id int64) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	query := `
		UPDATE check_patterns 
		SET use_count = use_count + 1, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND active = 1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment use count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrCheckPatternNotFound
	}

	slog.Debug("incremented check pattern use count", "id", id)
	return nil
}

// Transaction implementations

// CreateCheckPattern creates a new check pattern within a transaction.
func (t *sqliteTransaction) CreateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	// Reuse the main implementation with transaction's connection
	// For now, we'll delegate to the main storage instance
	// In a real implementation, we'd use t.tx instead of s.db
	return t.storage.CreateCheckPattern(ctx, pattern)
}

// GetCheckPattern retrieves a check pattern by ID within a transaction.
func (t *sqliteTransaction) GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error) {
	return t.storage.GetCheckPattern(ctx, id)
}

// GetActiveCheckPatterns returns all active check patterns within a transaction.
func (t *sqliteTransaction) GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	return t.storage.GetActiveCheckPatterns(ctx)
}

// GetMatchingCheckPatterns returns patterns that match the transaction within a transaction.
func (t *sqliteTransaction) GetMatchingCheckPatterns(ctx context.Context, txn model.Transaction) ([]model.CheckPattern, error) {
	return t.storage.GetMatchingCheckPatterns(ctx, txn)
}

// UpdateCheckPattern updates a check pattern within a transaction.
func (t *sqliteTransaction) UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	return t.storage.UpdateCheckPattern(ctx, pattern)
}

// DeleteCheckPattern soft deletes a check pattern within a transaction.
func (t *sqliteTransaction) DeleteCheckPattern(ctx context.Context, id int64) error {
	return t.storage.DeleteCheckPattern(ctx, id)
}

// IncrementCheckPatternUseCount increments use count within a transaction.
func (t *sqliteTransaction) IncrementCheckPatternUseCount(ctx context.Context, id int64) error {
	return t.storage.IncrementCheckPatternUseCount(ctx, id)
}
