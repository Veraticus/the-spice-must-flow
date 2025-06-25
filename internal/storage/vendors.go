package storage

import (
	"context"
	"database/sql"
	"fmt"
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

// GetVendorsByCategory retrieves all vendors for a specific category.
func (s *SQLiteStorage) GetVendorsByCategory(ctx context.Context, categoryName string) ([]model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(categoryName, "categoryName"); err != nil {
		return nil, err
	}
	return s.getVendorsByCategoryTx(ctx, s.db, categoryName)
}

func (s *SQLiteStorage) getVendorsByCategoryTx(ctx context.Context, q queryable, categoryName string) ([]model.Vendor, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT name, category, last_updated, use_count
		FROM vendors
		WHERE category = ?
		ORDER BY name
	`, categoryName)
	if err != nil {
		return nil, fmt.Errorf("failed to query vendors by category: %w", err)
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

// GetVendorsByCategoryID retrieves all vendors for a specific category ID.
func (s *SQLiteStorage) GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	// First get the category name from ID
	var categoryName string
	err := s.db.QueryRowContext(ctx, `
		SELECT name FROM categories WHERE id = ?
	`, categoryID).Scan(&categoryName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category with ID %d not found", categoryID)
		}
		return nil, fmt.Errorf("failed to get category name: %w", err)
	}

	return s.GetVendorsByCategory(ctx, categoryName)
}

// UpdateVendorCategories updates all vendors from one category to another.
func (s *SQLiteStorage) UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateString(fromCategory, "fromCategory"); err != nil {
		return err
	}
	if err := validateString(toCategory, "toCategory"); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify toCategory exists
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND is_active = 1)
	`, toCategory).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check category existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("category '%s' does not exist", toCategory)
	}

	// Update vendors
	_, err = tx.ExecContext(ctx, `
		UPDATE vendors 
		SET category = ?, last_updated = ?
		WHERE category = ?
	`, toCategory, time.Now(), fromCategory)
	if err != nil {
		return fmt.Errorf("failed to update vendors: %w", err)
	}

	// Clear cache since we've updated vendors
	s.cacheMutex.Lock()
	s.vendorCache = make(map[string]*model.Vendor)
	s.cacheMutex.Unlock()

	return tx.Commit()
}

// UpdateVendorCategoriesByID updates all vendors from one category ID to another.
func (s *SQLiteStorage) UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	// Get category names from IDs
	var fromCategory, toCategory string
	err := s.db.QueryRowContext(ctx, `
		SELECT name FROM categories WHERE id = ?
	`, fromCategoryID).Scan(&fromCategory)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("category with ID %d not found", fromCategoryID)
		}
		return fmt.Errorf("failed to get from category name: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT name FROM categories WHERE id = ?
	`, toCategoryID).Scan(&toCategory)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("category with ID %d not found", toCategoryID)
		}
		return fmt.Errorf("failed to get to category name: %w", err)
	}

	return s.UpdateVendorCategories(ctx, fromCategory, toCategory)
}
