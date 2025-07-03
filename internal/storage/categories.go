package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// ErrCategoryNotFound is returned when a category is not found.
var ErrCategoryNotFound = errors.New("category not found")

// GetCategories returns all active categories.
func (s *SQLiteStorage) GetCategories(ctx context.Context) ([]model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, created_at, is_active, type, default_business_percent
		FROM categories
		WHERE is_active = 1
		ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var categories []model.Category
	for rows.Next() {
		var cat model.Category
		var catType sql.NullString
		var defaultBusinessPercent sql.NullInt64
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive, &catType, &defaultBusinessPercent); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		// Set category type
		if catType.Valid && catType.String != "" {
			cat.Type = model.CategoryType(catType.String)
		} else {
			cat.Type = model.CategoryTypeExpense // default
		}
		// Set default business percent
		if defaultBusinessPercent.Valid {
			cat.DefaultBusinessPercent = int(defaultBusinessPercent.Int64)
		}
		categories = append(categories, cat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	slog.Debug("retrieved categories", "count", len(categories))
	return categories, nil
}

// GetCategoryByName returns a category by its name.
func (s *SQLiteStorage) GetCategoryByName(ctx context.Context, name string) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	query := `
		SELECT id, name, description, created_at, is_active, type, default_business_percent
		FROM categories
		WHERE name = ? AND is_active = 1`

	var cat model.Category
	var catType sql.NullString
	var defaultBusinessPercent sql.NullInt64
	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive, &catType, &defaultBusinessPercent,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	// Set category type
	if catType.Valid && catType.String != "" {
		cat.Type = model.CategoryType(catType.String)
	} else {
		cat.Type = model.CategoryTypeExpense // default
	}
	// Set default business percent
	if defaultBusinessPercent.Valid {
		cat.DefaultBusinessPercent = int(defaultBusinessPercent.Int64)
	}

	return &cat, nil
}

// GetCategoryByID returns a category by its ID.
func (s *SQLiteStorage) GetCategoryByID(ctx context.Context, id int) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if id <= 0 {
		return nil, fmt.Errorf("invalid category ID: %d", id)
	}

	query := `
		SELECT id, name, description, created_at, is_active, type, default_business_percent
		FROM categories
		WHERE id = ? AND is_active = 1`

	var cat model.Category
	var catType sql.NullString
	var defaultBusinessPercent sql.NullInt64
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive, &catType, &defaultBusinessPercent,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	// Set category type
	if catType.Valid && catType.String != "" {
		cat.Type = model.CategoryType(catType.String)
	} else {
		cat.Type = model.CategoryTypeExpense // default
	}
	// Set default business percent
	if defaultBusinessPercent.Valid {
		cat.DefaultBusinessPercent = int(defaultBusinessPercent.Int64)
	}

	return &cat, nil
}

// CreateCategory creates a new category with default expense type.
func (s *SQLiteStorage) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	return s.CreateCategoryWithType(ctx, name, description, model.CategoryTypeExpense)
}

// CreateCategoryWithType creates a new category with the specified type.
func (s *SQLiteStorage) CreateCategoryWithType(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check if category already exists (including inactive ones)
	existingQuery := `
		SELECT id, name, description, created_at, is_active, type, default_business_percent
		FROM categories
		WHERE name = ?`

	var existing model.Category
	var typeStr sql.NullString
	err := s.db.QueryRowContext(ctx, existingQuery, name).Scan(
		&existing.ID, &existing.Name, &existing.Description, &existing.CreatedAt, &existing.IsActive, &typeStr, &existing.DefaultBusinessPercent,
	)

	if err == nil {
		// Category exists
		if typeStr.Valid {
			existing.Type = model.CategoryType(typeStr.String)
		} else {
			existing.Type = model.CategoryTypeExpense
		}

		if !existing.IsActive {
			// Reactivate it and update type if needed
			updateQuery := `UPDATE categories SET is_active = 1, type = ? WHERE id = ?`
			if _, updateErr := s.db.ExecContext(ctx, updateQuery, string(categoryType), existing.ID); updateErr != nil {
				return nil, fmt.Errorf("failed to reactivate category: %w", updateErr)
			}
			existing.IsActive = true
			existing.Type = categoryType
			slog.Info("reactivated existing category", "name", name, "type", categoryType)
		}
		return &existing, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing category: %w", err)
	}

	// Create new category
	insertQuery := `
		INSERT INTO categories (name, description, created_at, is_active, type)
		VALUES (?, ?, ?, 1, ?)`

	now := time.Now()
	result, err := s.db.ExecContext(ctx, insertQuery, name, description, now, string(categoryType))
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get category ID: %w", err)
	}

	// Query the created category to get all fields including default_business_percent
	category, err := s.GetCategoryByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created category: %w", err)
	}

	slog.Info("created new category", "name", name, "id", id, "type", categoryType)
	return category, nil
}

// Transaction implementations for category operations

// GetCategories returns all active categories within a transaction.
func (t *sqliteTransaction) GetCategories(ctx context.Context) ([]model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, created_at, is_active, type, default_business_percent
		FROM categories
		WHERE is_active = 1
		ORDER BY name`

	rows, err := t.tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var categories []model.Category
	for rows.Next() {
		var cat model.Category
		var catType sql.NullString
		var defaultBusinessPercent sql.NullInt64
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive, &catType, &defaultBusinessPercent); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		// Set category type
		if catType.Valid && catType.String != "" {
			cat.Type = model.CategoryType(catType.String)
		} else {
			cat.Type = model.CategoryTypeExpense // default
		}
		// Set default business percent
		if defaultBusinessPercent.Valid {
			cat.DefaultBusinessPercent = int(defaultBusinessPercent.Int64)
		}
		categories = append(categories, cat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

// GetCategoryByName returns a category by its name within a transaction.
func (t *sqliteTransaction) GetCategoryByName(ctx context.Context, name string) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	query := `
		SELECT id, name, description, created_at, is_active, type, default_business_percent
		FROM categories
		WHERE name = ? AND is_active = 1`

	var cat model.Category
	var catType sql.NullString
	var defaultBusinessPercent sql.NullInt64
	err := t.tx.QueryRowContext(ctx, query, name).Scan(
		&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive, &catType, &defaultBusinessPercent,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	// Set category type
	if catType.Valid && catType.String != "" {
		cat.Type = model.CategoryType(catType.String)
	} else {
		cat.Type = model.CategoryTypeExpense // default
	}
	// Set default business percent
	if defaultBusinessPercent.Valid {
		cat.DefaultBusinessPercent = int(defaultBusinessPercent.Int64)
	}

	return &cat, nil
}

// CreateCategory creates a new category with default expense type within a transaction.
func (t *sqliteTransaction) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	return t.CreateCategoryWithType(ctx, name, description, model.CategoryTypeExpense)
}

// CreateCategoryWithType creates a new category with the specified type within a transaction.
func (t *sqliteTransaction) CreateCategoryWithType(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check if category already exists (including inactive ones)
	existingQuery := `
		SELECT id, name, description, created_at, is_active, type
		FROM categories
		WHERE name = ?`

	var existing model.Category
	var typeStr sql.NullString
	err := t.tx.QueryRowContext(ctx, existingQuery, name).Scan(
		&existing.ID, &existing.Name, &existing.Description, &existing.CreatedAt, &existing.IsActive, &typeStr,
	)

	if err == nil {
		// Category exists
		if typeStr.Valid {
			existing.Type = model.CategoryType(typeStr.String)
		} else {
			existing.Type = model.CategoryTypeExpense
		}

		if !existing.IsActive {
			// Reactivate it and update type if needed
			updateQuery := `UPDATE categories SET is_active = 1, type = ? WHERE id = ?`
			if _, updateErr := t.tx.ExecContext(ctx, updateQuery, string(categoryType), existing.ID); updateErr != nil {
				return nil, fmt.Errorf("failed to reactivate category: %w", updateErr)
			}
			existing.IsActive = true
			existing.Type = categoryType
		}
		return &existing, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing category: %w", err)
	}

	// Create new category
	insertQuery := `
		INSERT INTO categories (name, description, created_at, is_active, type)
		VALUES (?, ?, ?, 1, ?)`

	now := time.Now()
	result, err := t.tx.ExecContext(ctx, insertQuery, name, description, now, string(categoryType))
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get category ID: %w", err)
	}

	category := &model.Category{
		ID:          int(id),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		IsActive:    true,
		Type:        categoryType,
	}

	return category, nil
}

// UpdateCategory updates an existing category.
func (s *SQLiteStorage) UpdateCategory(ctx context.Context, id int, name, description string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if name == "" {
		return fmt.Errorf("category name cannot be empty")
	}

	// Check if another category already has this name
	var existingID int
	checkQuery := `SELECT id FROM categories WHERE name = ? AND id != ? AND is_active = 1`
	err := s.db.QueryRowContext(ctx, checkQuery, name, id).Scan(&existingID)
	if err != sql.ErrNoRows {
		if err == nil {
			return fmt.Errorf("category with name %q already exists", name)
		}
		return fmt.Errorf("failed to check for duplicate category: %w", err)
	}

	updateQuery := `
		UPDATE categories 
		SET name = ?, description = ?
		WHERE id = ? AND is_active = 1`

	result, err := s.db.ExecContext(ctx, updateQuery, name, description, id)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found or inactive")
	}

	slog.Info("updated category", "id", id, "name", name)
	return nil
}

// UpdateCategoryBusinessPercent updates the default business percentage for a category.
func (s *SQLiteStorage) UpdateCategoryBusinessPercent(ctx context.Context, id int, businessPercent int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if businessPercent < 0 || businessPercent > 100 {
		return fmt.Errorf("business percentage must be between 0 and 100")
	}

	// Check if category exists and is not income/system type
	var catType sql.NullString
	checkQuery := `SELECT type FROM categories WHERE id = ? AND is_active = 1`
	err := s.db.QueryRowContext(ctx, checkQuery, id).Scan(&catType)
	if err == sql.ErrNoRows {
		return fmt.Errorf("category with ID %d not found", id)
	}
	if err != nil {
		return fmt.Errorf("failed to check category: %w", err)
	}

	// Only allow business percent updates for expense categories
	if catType.Valid && (catType.String == string(model.CategoryTypeIncome) || catType.String == string(model.CategoryTypeSystem)) {
		return fmt.Errorf("cannot set business percentage for %s categories", catType.String)
	}

	updateQuery := `
		UPDATE categories 
		SET default_business_percent = ?
		WHERE id = ? AND is_active = 1`

	result, err := s.db.ExecContext(ctx, updateQuery, businessPercent, id)
	if err != nil {
		return fmt.Errorf("failed to update category business percentage: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("category with ID %d not found", id)
	}

	slog.Info("updated category business percentage", "id", id, "business_percent", businessPercent)
	return nil
}

// DeleteCategory soft-deletes a category by setting is_active to false.
func (s *SQLiteStorage) DeleteCategory(ctx context.Context, id int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	// Check if category is in use
	var usageCount int
	checkQuery := `
		SELECT COUNT(*) 
		FROM classifications 
		WHERE category = (SELECT name FROM categories WHERE id = ?)`

	if err := s.db.QueryRowContext(ctx, checkQuery, id).Scan(&usageCount); err != nil {
		return fmt.Errorf("failed to check category usage: %w", err)
	}

	if usageCount > 0 {
		return fmt.Errorf("cannot delete category: %d transactions are using it", usageCount)
	}

	// Soft delete the category
	deleteQuery := `UPDATE categories SET is_active = 0 WHERE id = ?`
	result, err := s.db.ExecContext(ctx, deleteQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found")
	}

	slog.Info("deleted category", "id", id)
	return nil
}

// Transaction implementations for UpdateCategory and DeleteCategory

// UpdateCategory updates an existing category within a transaction.
func (t *sqliteTransaction) UpdateCategory(ctx context.Context, id int, name, description string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if name == "" {
		return fmt.Errorf("category name cannot be empty")
	}

	// Check if another category already has this name
	var existingID int
	checkQuery := `SELECT id FROM categories WHERE name = ? AND id != ? AND is_active = 1`
	err := t.tx.QueryRowContext(ctx, checkQuery, name, id).Scan(&existingID)
	if err != sql.ErrNoRows {
		if err == nil {
			return fmt.Errorf("category with name %q already exists", name)
		}
		return fmt.Errorf("failed to check for duplicate category: %w", err)
	}

	updateQuery := `
		UPDATE categories 
		SET name = ?, description = ?
		WHERE id = ? AND is_active = 1`

	result, err := t.tx.ExecContext(ctx, updateQuery, name, description, id)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found or inactive")
	}

	return nil
}

// UpdateCategoryBusinessPercent updates the default business percentage for a category within a transaction.
func (t *sqliteTransaction) UpdateCategoryBusinessPercent(ctx context.Context, id int, businessPercent int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if businessPercent < 0 || businessPercent > 100 {
		return fmt.Errorf("business percentage must be between 0 and 100")
	}

	// Check if category exists and is not income/system type
	var catType sql.NullString
	checkQuery := `SELECT type FROM categories WHERE id = ? AND is_active = 1`
	err := t.tx.QueryRowContext(ctx, checkQuery, id).Scan(&catType)
	if err == sql.ErrNoRows {
		return fmt.Errorf("category with ID %d not found", id)
	}
	if err != nil {
		return fmt.Errorf("failed to check category: %w", err)
	}

	// Only allow business percent updates for expense categories
	if catType.Valid && (catType.String == string(model.CategoryTypeIncome) || catType.String == string(model.CategoryTypeSystem)) {
		return fmt.Errorf("cannot set business percentage for %s categories", catType.String)
	}

	updateQuery := `
		UPDATE categories 
		SET default_business_percent = ?
		WHERE id = ? AND is_active = 1`

	result, err := t.tx.ExecContext(ctx, updateQuery, businessPercent, id)
	if err != nil {
		return fmt.Errorf("failed to update category business percentage: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("category with ID %d not found", id)
	}

	return nil
}

// DeleteCategory soft-deletes a category within a transaction.
func (t *sqliteTransaction) DeleteCategory(ctx context.Context, id int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	// Check if category is in use
	var usageCount int
	checkQuery := `
		SELECT COUNT(*) 
		FROM classifications 
		WHERE category = (SELECT name FROM categories WHERE id = ?)`

	if err := t.tx.QueryRowContext(ctx, checkQuery, id).Scan(&usageCount); err != nil {
		return fmt.Errorf("failed to check category usage: %w", err)
	}

	if usageCount > 0 {
		return fmt.Errorf("cannot delete category: %d transactions are using it", usageCount)
	}

	// Soft delete the category
	deleteQuery := `UPDATE categories SET is_active = 0 WHERE id = ?`
	result, err := t.tx.ExecContext(ctx, deleteQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}
