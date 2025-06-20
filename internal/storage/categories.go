package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// GetCategories returns all active categories.
func (s *SQLiteStorage) GetCategories(ctx context.Context) ([]model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, created_at, is_active
		FROM categories
		WHERE is_active = 1
		ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var cat model.Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.CreatedAt, &cat.IsActive); err != nil {
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
		SELECT id, name, created_at, is_active
		FROM categories
		WHERE name = ? AND is_active = 1`

	var cat model.Category
	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&cat.ID, &cat.Name, &cat.CreatedAt, &cat.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Category not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	return &cat, nil
}

// CreateCategory creates a new category.
func (s *SQLiteStorage) CreateCategory(ctx context.Context, name string) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check if category already exists (including inactive ones)
	existingQuery := `
		SELECT id, name, created_at, is_active
		FROM categories
		WHERE name = ?`

	var existing model.Category
	err := s.db.QueryRowContext(ctx, existingQuery, name).Scan(
		&existing.ID, &existing.Name, &existing.CreatedAt, &existing.IsActive,
	)

	if err == nil {
		// Category exists
		if !existing.IsActive {
			// Reactivate it
			updateQuery := `UPDATE categories SET is_active = 1 WHERE id = ?`
			if _, err := s.db.ExecContext(ctx, updateQuery, existing.ID); err != nil {
				return nil, fmt.Errorf("failed to reactivate category: %w", err)
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
		INSERT INTO categories (name, created_at, is_active)
		VALUES (?, ?, 1)`

	now := time.Now()
	result, err := s.db.ExecContext(ctx, insertQuery, name, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get category ID: %w", err)
	}

	category := &model.Category{
		ID:        int(id),
		Name:      name,
		CreatedAt: now,
		IsActive:  true,
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
		SELECT id, name, created_at, is_active
		FROM categories
		WHERE is_active = 1
		ORDER BY name`

	rows, err := t.tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var cat model.Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.CreatedAt, &cat.IsActive); err != nil {
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
		SELECT id, name, created_at, is_active
		FROM categories
		WHERE name = ? AND is_active = 1`

	var cat model.Category
	err := t.tx.QueryRowContext(ctx, query, name).Scan(
		&cat.ID, &cat.Name, &cat.CreatedAt, &cat.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Category not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query category: %w", err)
	}

	return &cat, nil
}

// CreateCategory creates a new category within a transaction.
func (t *sqliteTransaction) CreateCategory(ctx context.Context, name string) (*model.Category, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check if category already exists (including inactive ones)
	existingQuery := `
		SELECT id, name, created_at, is_active
		FROM categories
		WHERE name = ?`

	var existing model.Category
	err := t.tx.QueryRowContext(ctx, existingQuery, name).Scan(
		&existing.ID, &existing.Name, &existing.CreatedAt, &existing.IsActive,
	)

	if err == nil {
		// Category exists
		if !existing.IsActive {
			// Reactivate it
			updateQuery := `UPDATE categories SET is_active = 1 WHERE id = ?`
			if _, err := t.tx.ExecContext(ctx, updateQuery, existing.ID); err != nil {
				return nil, fmt.Errorf("failed to reactivate category: %w", err)
			}
			existing.IsActive = true
		}
		return &existing, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing category: %w", err)
	}

	// Create new category
	insertQuery := `
		INSERT INTO categories (name, created_at, is_active)
		VALUES (?, ?, 1)`

	now := time.Now()
	result, err := t.tx.ExecContext(ctx, insertQuery, name, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get category ID: %w", err)
	}

	category := &model.Category{
		ID:        int(id),
		Name:      name,
		CreatedAt: now,
		IsActive:  true,
	}

	return category, nil
}