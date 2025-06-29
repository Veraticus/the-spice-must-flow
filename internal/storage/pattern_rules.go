package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// CreatePatternRule creates a new pattern rule.
func (s *SQLiteStorage) CreatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if err := validatePatternRule(rule); err != nil {
		return err
	}

	// Verify category exists
	var categoryCount int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM categories WHERE name = ? AND is_active = 1",
		rule.DefaultCategory).Scan(&categoryCount)
	if err != nil {
		return fmt.Errorf("failed to verify category: %w", err)
	}
	if categoryCount == 0 {
		return fmt.Errorf("category %q does not exist or is inactive", rule.DefaultCategory)
	}

	query := `
		INSERT INTO pattern_rules (
			name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		rule.Name, rule.Description, rule.MerchantPattern, rule.IsRegex,
		rule.AmountCondition, rule.AmountValue, rule.AmountMin, rule.AmountMax,
		directionToNullString(rule.Direction), rule.DefaultCategory,
		rule.Confidence, rule.Priority, rule.IsActive,
	)
	if err != nil {
		return fmt.Errorf("failed to create pattern rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get pattern rule ID: %w", err)
	}

	rule.ID = int(id)
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	return nil
}

// GetPatternRule retrieves a pattern rule by ID.
func (s *SQLiteStorage) GetPatternRule(ctx context.Context, id int) (*model.PatternRule, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active,
			created_at, updated_at, use_count
		FROM pattern_rules
		WHERE id = ?
	`

	var rule model.PatternRule
	var direction sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.Description, &rule.MerchantPattern, &rule.IsRegex,
		&rule.AmountCondition, &rule.AmountValue, &rule.AmountMin, &rule.AmountMax,
		&direction, &rule.DefaultCategory, &rule.Confidence, &rule.Priority, &rule.IsActive,
		&rule.CreatedAt, &rule.UpdatedAt, &rule.UseCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pattern rule not found")
		}
		return nil, fmt.Errorf("failed to get pattern rule: %w", err)
	}

	rule.Direction = nullStringToDirection(direction)

	return &rule, nil
}

// GetActivePatternRules retrieves all active pattern rules ordered by priority.
func (s *SQLiteStorage) GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active,
			created_at, updated_at, use_count
		FROM pattern_rules
		WHERE is_active = 1
		ORDER BY priority DESC, id ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active pattern rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rules []model.PatternRule
	for rows.Next() {
		var rule model.PatternRule
		var direction sql.NullString
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.MerchantPattern, &rule.IsRegex,
			&rule.AmountCondition, &rule.AmountValue, &rule.AmountMin, &rule.AmountMax,
			&direction, &rule.DefaultCategory, &rule.Confidence, &rule.Priority, &rule.IsActive,
			&rule.CreatedAt, &rule.UpdatedAt, &rule.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pattern rule: %w", err)
		}
		rule.Direction = nullStringToDirection(direction)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pattern rules: %w", err)
	}

	return rules, nil
}

// UpdatePatternRule updates an existing pattern rule.
func (s *SQLiteStorage) UpdatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if err := validatePatternRule(rule); err != nil {
		return err
	}

	// Verify category exists
	var categoryCount int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM categories WHERE name = ? AND is_active = 1",
		rule.DefaultCategory).Scan(&categoryCount)
	if err != nil {
		return fmt.Errorf("failed to verify category: %w", err)
	}
	if categoryCount == 0 {
		return fmt.Errorf("category %q does not exist or is inactive", rule.DefaultCategory)
	}

	query := `
		UPDATE pattern_rules SET
			name = ?, description = ?, merchant_pattern = ?, is_regex = ?,
			amount_condition = ?, amount_value = ?, amount_min = ?, amount_max = ?,
			direction = ?, default_category = ?, confidence = ?, priority = ?, is_active = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		rule.Name, rule.Description, rule.MerchantPattern, rule.IsRegex,
		rule.AmountCondition, rule.AmountValue, rule.AmountMin, rule.AmountMax,
		directionToNullString(rule.Direction), rule.DefaultCategory,
		rule.Confidence, rule.Priority, rule.IsActive,
		rule.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update pattern rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pattern rule not found")
	}

	return nil
}

// DeletePatternRule deletes a pattern rule.
func (s *SQLiteStorage) DeletePatternRule(ctx context.Context, id int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM pattern_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete pattern rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pattern rule not found")
	}

	return nil
}

// IncrementPatternRuleUseCount increments the use count for a pattern rule.
func (s *SQLiteStorage) IncrementPatternRuleUseCount(ctx context.Context, id int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	query := `UPDATE pattern_rules SET use_count = use_count + 1 WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment pattern rule use count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pattern rule not found")
	}

	return nil
}

// GetPatternRulesByCategory retrieves all pattern rules for a specific category.
func (s *SQLiteStorage) GetPatternRulesByCategory(ctx context.Context, category string) ([]model.PatternRule, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if err := validateString(category, "category"); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active,
			created_at, updated_at, use_count
		FROM pattern_rules
		WHERE default_category = ?
		ORDER BY priority DESC, id ASC
	`

	rows, err := s.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to get pattern rules by category: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rules []model.PatternRule
	for rows.Next() {
		var rule model.PatternRule
		var direction sql.NullString
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.MerchantPattern, &rule.IsRegex,
			&rule.AmountCondition, &rule.AmountValue, &rule.AmountMin, &rule.AmountMax,
			&direction, &rule.DefaultCategory, &rule.Confidence, &rule.Priority, &rule.IsActive,
			&rule.CreatedAt, &rule.UpdatedAt, &rule.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pattern rule: %w", err)
		}
		rule.Direction = nullStringToDirection(direction)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pattern rules: %w", err)
	}

	return rules, nil
}

// validatePatternRule validates a pattern rule.
func validatePatternRule(rule *model.PatternRule) error {
	if rule == nil {
		return fmt.Errorf("pattern rule cannot be nil")
	}
	if err := validateString(rule.Name, "name"); err != nil {
		return err
	}
	if err := validateString(rule.DefaultCategory, "default_category"); err != nil {
		return err
	}
	if rule.Confidence < 0 || rule.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	// Validate amount condition
	validConditions := map[string]bool{
		"lt": true, "le": true, "eq": true, "ge": true,
		"gt": true, "range": true, "any": true,
	}
	if !validConditions[rule.AmountCondition] {
		return fmt.Errorf("invalid amount condition: %s", rule.AmountCondition)
	}

	// Validate amount values based on condition
	switch rule.AmountCondition {
	case "lt", "le", "eq", "ge", "gt":
		if rule.AmountValue == nil {
			return fmt.Errorf("amount_value required for condition %s", rule.AmountCondition)
		}
	case "range":
		if rule.AmountMin == nil && rule.AmountMax == nil {
			return fmt.Errorf("at least one of amount_min or amount_max required for range condition")
		}
	}

	return nil
}

// directionToNullString converts a TransactionDirection pointer to sql.NullString.
func directionToNullString(dir *model.TransactionDirection) sql.NullString {
	if dir == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(*dir), Valid: true}
}

// nullStringToDirection converts sql.NullString to TransactionDirection pointer.
func nullStringToDirection(ns sql.NullString) *model.TransactionDirection {
	if !ns.Valid {
		return nil
	}
	dir := model.TransactionDirection(ns.String)
	return &dir
}

// Transaction implementations for pattern rules

// CreatePatternRule creates a new pattern rule within a transaction.
func (t *sqliteTransaction) CreatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if err := validatePatternRule(rule); err != nil {
		return err
	}

	// Verify category exists
	var categoryCount int
	err := t.tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM categories WHERE name = ? AND is_active = 1",
		rule.DefaultCategory).Scan(&categoryCount)
	if err != nil {
		return fmt.Errorf("failed to verify category: %w", err)
	}
	if categoryCount == 0 {
		return fmt.Errorf("category %q does not exist or is inactive", rule.DefaultCategory)
	}

	query := `
		INSERT INTO pattern_rules (
			name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := t.tx.ExecContext(ctx, query,
		rule.Name, rule.Description, rule.MerchantPattern, rule.IsRegex,
		rule.AmountCondition, rule.AmountValue, rule.AmountMin, rule.AmountMax,
		directionToNullString(rule.Direction), rule.DefaultCategory,
		rule.Confidence, rule.Priority, rule.IsActive,
	)
	if err != nil {
		return fmt.Errorf("failed to create pattern rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get pattern rule ID: %w", err)
	}

	rule.ID = int(id)
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	return nil
}

// GetPatternRule retrieves a pattern rule by ID within a transaction.
func (t *sqliteTransaction) GetPatternRule(ctx context.Context, id int) (*model.PatternRule, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active,
			created_at, updated_at, use_count
		FROM pattern_rules
		WHERE id = ?
	`

	var rule model.PatternRule
	var direction sql.NullString
	err := t.tx.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.Description, &rule.MerchantPattern, &rule.IsRegex,
		&rule.AmountCondition, &rule.AmountValue, &rule.AmountMin, &rule.AmountMax,
		&direction, &rule.DefaultCategory, &rule.Confidence, &rule.Priority, &rule.IsActive,
		&rule.CreatedAt, &rule.UpdatedAt, &rule.UseCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pattern rule not found")
		}
		return nil, fmt.Errorf("failed to get pattern rule: %w", err)
	}

	rule.Direction = nullStringToDirection(direction)

	return &rule, nil
}

// GetActivePatternRules retrieves all active pattern rules within a transaction.
func (t *sqliteTransaction) GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active,
			created_at, updated_at, use_count
		FROM pattern_rules
		WHERE is_active = 1
		ORDER BY priority DESC, id ASC
	`

	rows, err := t.tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active pattern rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rules []model.PatternRule
	for rows.Next() {
		var rule model.PatternRule
		var direction sql.NullString
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.MerchantPattern, &rule.IsRegex,
			&rule.AmountCondition, &rule.AmountValue, &rule.AmountMin, &rule.AmountMax,
			&direction, &rule.DefaultCategory, &rule.Confidence, &rule.Priority, &rule.IsActive,
			&rule.CreatedAt, &rule.UpdatedAt, &rule.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pattern rule: %w", err)
		}
		rule.Direction = nullStringToDirection(direction)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pattern rules: %w", err)
	}

	return rules, nil
}

// UpdatePatternRule updates an existing pattern rule within a transaction.
func (t *sqliteTransaction) UpdatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if err := validatePatternRule(rule); err != nil {
		return err
	}

	// Verify category exists
	var categoryCount int
	err := t.tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM categories WHERE name = ? AND is_active = 1",
		rule.DefaultCategory).Scan(&categoryCount)
	if err != nil {
		return fmt.Errorf("failed to verify category: %w", err)
	}
	if categoryCount == 0 {
		return fmt.Errorf("category %q does not exist or is inactive", rule.DefaultCategory)
	}

	query := `
		UPDATE pattern_rules SET
			name = ?, description = ?, merchant_pattern = ?, is_regex = ?,
			amount_condition = ?, amount_value = ?, amount_min = ?, amount_max = ?,
			direction = ?, default_category = ?, confidence = ?, priority = ?, is_active = ?
		WHERE id = ?
	`

	result, err := t.tx.ExecContext(ctx, query,
		rule.Name, rule.Description, rule.MerchantPattern, rule.IsRegex,
		rule.AmountCondition, rule.AmountValue, rule.AmountMin, rule.AmountMax,
		directionToNullString(rule.Direction), rule.DefaultCategory,
		rule.Confidence, rule.Priority, rule.IsActive,
		rule.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update pattern rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pattern rule not found")
	}

	return nil
}

// DeletePatternRule deletes a pattern rule within a transaction.
func (t *sqliteTransaction) DeletePatternRule(ctx context.Context, id int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	result, err := t.tx.ExecContext(ctx, "DELETE FROM pattern_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete pattern rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pattern rule not found")
	}

	return nil
}

// IncrementPatternRuleUseCount increments the use count within a transaction.
func (t *sqliteTransaction) IncrementPatternRuleUseCount(ctx context.Context, id int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	query := `UPDATE pattern_rules SET use_count = use_count + 1 WHERE id = ?`
	result, err := t.tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment pattern rule use count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pattern rule not found")
	}

	return nil
}

// GetPatternRulesByCategory retrieves pattern rules by category within a transaction.
func (t *sqliteTransaction) GetPatternRulesByCategory(ctx context.Context, category string) ([]model.PatternRule, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if err := validateString(category, "category"); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, merchant_pattern, is_regex,
			amount_condition, amount_value, amount_min, amount_max,
			direction, default_category, confidence, priority, is_active,
			created_at, updated_at, use_count
		FROM pattern_rules
		WHERE default_category = ?
		ORDER BY priority DESC, id ASC
	`

	rows, err := t.tx.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to get pattern rules by category: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rules []model.PatternRule
	for rows.Next() {
		var rule model.PatternRule
		var direction sql.NullString
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.MerchantPattern, &rule.IsRegex,
			&rule.AmountCondition, &rule.AmountValue, &rule.AmountMin, &rule.AmountMax,
			&direction, &rule.DefaultCategory, &rule.Confidence, &rule.Priority, &rule.IsActive,
			&rule.CreatedAt, &rule.UpdatedAt, &rule.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pattern rule: %w", err)
		}
		rule.Direction = nullStringToDirection(direction)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pattern rules: %w", err)
	}

	return rules, nil
}
