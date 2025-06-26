package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteStorage implements the Storage interface using SQLite.
type SQLiteStorage struct {
	cacheExpiry time.Time
	db          *sql.DB
	vendorCache map[string]*model.Vendor
	dbPath      string
	cacheMutex  sync.RWMutex
}

// NewSQLiteStorage creates a new SQLite storage instance.
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	// Validate input
	if err := validateString(dbPath, "dbPath"); err != nil {
		return nil, err
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite doesn't benefit from multiple connections
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SQLiteStorage{
		db:          db,
		dbPath:      dbPath,
		vendorCache: make(map[string]*model.Vendor),
	}, nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// NewCheckpointManager creates a new checkpoint manager for this storage instance.
func (s *SQLiteStorage) NewCheckpointManager() (*CheckpointManager, error) {
	return NewCheckpointManager(s.db, s.dbPath)
}

// BeginTx starts a new database transaction.
func (s *SQLiteStorage) BeginTx(ctx context.Context) (service.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &sqliteTransaction{
		tx:      tx,
		storage: s,
	}, nil
}

// sqliteTransaction wraps sql.Tx to implement service.Transaction.
type sqliteTransaction struct {
	tx      *sql.Tx
	storage *SQLiteStorage
}

func (t *sqliteTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *sqliteTransaction) Rollback() error {
	return t.tx.Rollback()
}

// Transaction methods delegate to the main storage with the transaction.
func (t *sqliteTransaction) SaveTransactions(ctx context.Context, transactions []model.Transaction) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateTransactions(transactions); err != nil {
		return err
	}
	return t.storage.saveTransactionsTx(ctx, t.tx, transactions)
}

func (t *sqliteTransaction) GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return t.storage.getTransactionsToClassifyTx(ctx, t.tx, fromDate)
}

func (t *sqliteTransaction) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(id, "id"); err != nil {
		return nil, err
	}
	return t.storage.getTransactionByIDTx(ctx, t.tx, id)
}

func (t *sqliteTransaction) GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(merchantName, "merchantName"); err != nil {
		return nil, err
	}
	return t.storage.getVendorTx(ctx, t.tx, merchantName)
}

func (t *sqliteTransaction) SaveVendor(ctx context.Context, vendor *model.Vendor) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateVendor(vendor); err != nil {
		return err
	}
	return t.storage.saveVendorTx(ctx, t.tx, vendor)
}

func (t *sqliteTransaction) GetAllVendors(ctx context.Context) ([]model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return t.storage.getAllVendorsTx(ctx, t.tx)
}

func (t *sqliteTransaction) SaveClassification(ctx context.Context, classification *model.Classification) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateClassification(classification); err != nil {
		return err
	}
	return t.storage.saveClassificationTx(ctx, t.tx, classification)
}

func (t *sqliteTransaction) GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, fmt.Errorf("%w: end date %v is before start date %v", ErrInvalidDateRange, end, start)
	}
	return t.storage.getClassificationsByDateRangeTx(ctx, t.tx, start, end)
}

func (t *sqliteTransaction) Migrate(_ context.Context) error {
	// Migrations should not be run within a transaction
	return fmt.Errorf("migrations cannot be run within a transaction")
}

func (t *sqliteTransaction) GetTransactionsByCategory(ctx context.Context, categoryName string) ([]model.Transaction, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(categoryName, "categoryName"); err != nil {
		return nil, err
	}
	return t.storage.getTransactionsByCategoryTx(ctx, t.tx, categoryName)
}

func (t *sqliteTransaction) GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error) {
	return t.storage.GetTransactionsByCategoryID(ctx, categoryID)
}

func (t *sqliteTransaction) UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error {
	return t.storage.UpdateTransactionCategories(ctx, fromCategory, toCategory)
}

func (t *sqliteTransaction) UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	return t.storage.UpdateTransactionCategoriesByID(ctx, fromCategoryID, toCategoryID)
}

func (t *sqliteTransaction) GetTransactionCount(ctx context.Context) (int, error) {
	return t.storage.GetTransactionCount(ctx)
}

func (t *sqliteTransaction) GetTransactionCountByCategory(ctx context.Context, categoryName string) (int, error) {
	return t.storage.GetTransactionCountByCategory(ctx, categoryName)
}

func (t *sqliteTransaction) GetEarliestTransactionDate(ctx context.Context) (time.Time, error) {
	return t.storage.GetEarliestTransactionDate(ctx)
}

func (t *sqliteTransaction) GetLatestTransactionDate(ctx context.Context) (time.Time, error) {
	return t.storage.GetLatestTransactionDate(ctx)
}

func (t *sqliteTransaction) GetCategorySummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	return t.storage.GetCategorySummary(ctx, start, end)
}

func (t *sqliteTransaction) GetMerchantSummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	return t.storage.GetMerchantSummary(ctx, start, end)
}

func (t *sqliteTransaction) GetVendorsByCategory(ctx context.Context, categoryName string) ([]model.Vendor, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateString(categoryName, "categoryName"); err != nil {
		return nil, err
	}
	return t.storage.getVendorsByCategoryTx(ctx, t.tx, categoryName)
}

func (t *sqliteTransaction) GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error) {
	return t.storage.GetVendorsByCategoryID(ctx, categoryID)
}

func (t *sqliteTransaction) UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error {
	return t.storage.UpdateVendorCategories(ctx, fromCategory, toCategory)
}

func (t *sqliteTransaction) UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	return t.storage.UpdateVendorCategoriesByID(ctx, fromCategoryID, toCategoryID)
}

func (t *sqliteTransaction) BeginTx(_ context.Context) (service.Transaction, error) {
	// Nested transactions not supported
	return nil, fmt.Errorf("nested transactions not supported")
}

func (t *sqliteTransaction) Close() error {
	// Transactions should be committed or rolled back, not closed
	return fmt.Errorf("transactions must be committed or rolled back, not closed")
}
