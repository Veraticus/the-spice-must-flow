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
		SELECT id, name, description, created_at, is_active
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
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
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
		SELECT id, name, description, created_at, is_active
		FROM categories
		WHERE name = ? AND is_active = 1`

	var cat model.Category
	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	return &cat, nil
}

// CreateCategory creates a new category.
func (s *SQLiteStorage) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check if category already exists (including inactive ones)
	existingQuery := `
		SELECT id, name, description, created_at, is_active
		FROM categories
		WHERE name = ?`

	var existing model.Category
	err := s.db.QueryRowContext(ctx, existingQuery, name).Scan(
		&existing.ID, &existing.Name, &existing.Description, &existing.CreatedAt, &existing.IsActive,
	)

	if err == nil {
		// Category exists
		if !existing.IsActive {
			// Reactivate it
			updateQuery := `UPDATE categories SET is_active = 1 WHERE id = ?`
			if _, updateErr := s.db.ExecContext(ctx, updateQuery, existing.ID); updateErr != nil {
				return nil, fmt.Errorf("failed to reactivate category: %w", updateErr)
			}
			existing.IsActive = true
			slog.Info("reactivated existing category", "name", name)
		}
		return &existing, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing category: %w", err)
	}

	// Create new category
	insertQuery := `
		INSERT INTO categories (name, description, created_at, is_active)
		VALUES (?, ?, ?, 1)`

	now := time.Now()
	result, err := s.db.ExecContext(ctx, insertQuery, name, description, now)
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
	}

	slog.Info("created new category", "name", name, "id", id)
	return category, nil
}

// Transaction implementations for category operations

// GetCategories returns all active categories within a transaction.
func (t *sqliteTransaction) GetCategories(ctx context.Context) ([]model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, created_at, is_active
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
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
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
		SELECT id, name, description, created_at, is_active
		FROM categories
		WHERE name = ? AND is_active = 1`

	var cat model.Category
	err := t.tx.QueryRowContext(ctx, query, name).Scan(
		&cat.ID, &cat.Name, &cat.Description, &cat.CreatedAt, &cat.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	return &cat, nil
}

// CreateCategory creates a new category within a transaction.
func (t *sqliteTransaction) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check if category already exists (including inactive ones)
	existingQuery := `
		SELECT id, name, description, created_at, is_active
		FROM categories
		WHERE name = ?`

	var existing model.Category
	err := t.tx.QueryRowContext(ctx, existingQuery, name).Scan(
		&existing.ID, &existing.Name, &existing.Description, &existing.CreatedAt, &existing.IsActive,
	)

	if err == nil {
		// Category exists
		if !existing.IsActive {
			// Reactivate it
			updateQuery := `UPDATE categories SET is_active = 1 WHERE id = ?`
			if _, updateErr := t.tx.ExecContext(ctx, updateQuery, existing.ID); updateErr != nil {
				return nil, fmt.Errorf("failed to reactivate category: %w", updateErr)
			}
			existing.IsActive = true
		}
		return &existing, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing category: %w", err)
	}

	// Create new category
	insertQuery := `
		INSERT INTO categories (name, description, created_at, is_active)
		VALUES (?, ?, ?, 1)`

	now := time.Now()
	result, err := t.tx.ExecContext(ctx, insertQuery, name, description, now)
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
