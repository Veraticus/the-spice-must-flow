package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// GetVendor retrieves a vendor by name.
func (s *SQLiteStorage) GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(merchantName, "merchantName"); err != nil {
		return nil, err
	}

	// Check cache first
	if vendor := s.getCachedVendor(merchantName); vendor != nil {
		return vendor, nil
	}

	return s.getVendorTx(ctx, s.db, merchantName)
}

func (s *SQLiteStorage) getVendorTx(ctx context.Context, q queryable, merchantName string) (*model.Vendor, error) {
	var vendor model.Vendor

	err := q.QueryRowContext(ctx, `
		SELECT name, category, last_updated, use_count
		FROM vendors
		WHERE name = ?
	`, merchantName).Scan(
		&vendor.Name,
		&vendor.Category,
		&vendor.LastUpdated,
		&vendor.UseCount,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows // Not an error, just not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get vendor: %w", err)
	}

	// Update cache
	s.cacheVendor(&vendor)

	return &vendor, nil
}

// SaveVendor saves or updates a vendor rule.
func (s *SQLiteStorage) SaveVendor(ctx context.Context, vendor *model.Vendor) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateVendor(vendor); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.saveVendorTx(ctx, tx, vendor); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStorage) saveVendorTx(ctx context.Context, tx *sql.Tx, vendor *model.Vendor) error {
	// Set LastUpdated if not set
	if vendor.LastUpdated.IsZero() {
		vendor.LastUpdated = time.Now()
	}

	// Validate category exists
	var categoryExists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND is_active = 1)
	`, vendor.Category).Scan(&categoryExists)

	if err != nil {
		return fmt.Errorf("failed to check category existence: %w", err)
	}

	if !categoryExists {
		return fmt.Errorf("category '%s' does not exist", vendor.Category)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO vendors (name, category, last_updated, use_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			category = excluded.category,
			last_updated = excluded.last_updated,
			use_count = excluded.use_count
	`, vendor.Name, vendor.Category, vendor.LastUpdated, vendor.UseCount)

	if err != nil {
		return fmt.Errorf("failed to save vendor: %w", err)
	}

	// Update cache
	s.cacheVendor(vendor)

	return nil
}

// GetAllVendors retrieves all vendor rules.
func (s *SQLiteStorage) GetAllVendors(ctx context.Context) ([]model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return s.getAllVendorsTx(ctx, s.db)
}

func (s *SQLiteStorage) getAllVendorsTx(ctx context.Context, q queryable) ([]model.Vendor, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT name, category, last_updated, use_count
		FROM vendors
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query vendors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var vendors []model.Vendor
	for rows.Next() {
		var vendor model.Vendor
		err := rows.Scan(
			&vendor.Name,
			&vendor.Category,
			&vendor.LastUpdated,
			&vendor.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vendor: %w", err)
		}
		vendors = append(vendors, vendor)
	}

	return vendors, rows.Err()
}

// DeleteVendor deletes a vendor rule.
func (s *SQLiteStorage) DeleteVendor(ctx context.Context, merchantName string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateString(merchantName, "merchantName"); err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM vendors WHERE name = ?
	`, merchantName)

	if err != nil {
		return fmt.Errorf("failed to delete vendor: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return common.ErrNotFound
	}

	// Remove from cache with proper locking
	s.cacheMutex.Lock()
	delete(s.vendorCache, merchantName)
	s.cacheMutex.Unlock()

	return nil
}

// getCachedVendor retrieves a vendor from the cache.
func (s *SQLiteStorage) getCachedVendor(name string) *model.Vendor {
	s.cacheMutex.RLock()

	if time.Now().After(s.cacheExpiry) {
		// Cache expired, needs to be cleared
		// Upgrade to write lock
		s.cacheMutex.RUnlock()
		s.cacheMutex.Lock()
		defer s.cacheMutex.Unlock()

		// Double-check after acquiring write lock
		if time.Now().After(s.cacheExpiry) {
			s.vendorCache = make(map[string]*model.Vendor)
		}
		return nil
	}

	vendor := s.vendorCache[name]
	s.cacheMutex.RUnlock()
	return vendor
}

// cacheVendor adds a vendor to the cache.
func (s *SQLiteStorage) cacheVendor(vendor *model.Vendor) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	if len(s.vendorCache) == 0 {
		// Set cache expiry on first entry
		s.cacheExpiry = time.Now().Add(5 * time.Minute)
	}
	s.vendorCache[vendor.Name] = vendor
}

// WarmVendorCache loads all vendors into the cache.
func (s *SQLiteStorage) WarmVendorCache(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	vendors, err := s.GetAllVendors(ctx)
	if err != nil {
		return err
	}

	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.vendorCache = make(map[string]*model.Vendor)
	for i := range vendors {
		s.vendorCache[vendors[i].Name] = &vendors[i]
	}

	s.cacheExpiry = time.Now().Add(5 * time.Minute)
	return nil
}

// GetVendorsByCategory returns all vendors with the specified category.
func (s *SQLiteStorage) GetVendorsByCategory(ctx context.Context, category string) ([]model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT name, category, last_updated, use_count
		FROM vendors
		WHERE category = ?
		ORDER BY name
	`

	rows, err := s.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query vendors by category: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("Failed to close rows", "error", err)
		}
	}()

	var vendors []model.Vendor
	for rows.Next() {
		var vendor model.Vendor
		err := rows.Scan(
			&vendor.Name,
			&vendor.Category,
			&vendor.LastUpdated,
			&vendor.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vendor row: %w", err)
		}
		vendors = append(vendors, vendor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vendor rows: %w", err)
	}

	return vendors, nil
}

// UpdateVendorCategories updates all vendors from one category to another.
func (s *SQLiteStorage) UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if fromCategory == "" || toCategory == "" {
		return fmt.Errorf("both fromCategory and toCategory must be provided")
	}

	query := `
		UPDATE vendors 
		SET category = ?, last_updated = ?
		WHERE category = ?
	`

	result, err := s.db.ExecContext(ctx, query, toCategory, time.Now(), fromCategory)
	if err != nil {
		return fmt.Errorf("failed to update vendor categories: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Clear cache after update
	s.cacheMutex.Lock()
	s.vendorCache = nil
	s.cacheExpiry = time.Time{}
	s.cacheMutex.Unlock()

	slog.Info("Updated vendor categories",
		"from", fromCategory,
		"to", toCategory,
		"vendors_updated", rowsAffected)

	return nil
}

// GetVendorsByCategoryID returns all vendors with the specified category ID.
func (s *SQLiteStorage) GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error) {
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
	return s.GetVendorsByCategory(ctx, category.Name)
}

// UpdateVendorCategoriesByID updates all vendors from one category ID to another.
func (s *SQLiteStorage) UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
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
	return s.UpdateVendorCategories(ctx, fromCategory.Name, toCategory.Name)
}
