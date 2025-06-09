package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteStorage implements the Storage interface using SQLite.
type SQLiteStorage struct {
	db          *sql.DB
	vendorCache map[string]*model.Vendor
	cacheExpiry time.Time
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
		vendorCache: make(map[string]*model.Vendor),
	}, nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
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

func (t *sqliteTransaction) SaveProgress(ctx context.Context, progress *model.ClassificationProgress) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateProgress(progress); err != nil {
		return err
	}
	return t.storage.saveProgressTx(ctx, t.tx, progress)
}

func (t *sqliteTransaction) GetLatestProgress(ctx context.Context) (*model.ClassificationProgress, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	return t.storage.getLatestProgressTx(ctx, t.tx)
}

func (t *sqliteTransaction) Migrate(_ context.Context) error {
	// Migrations should not be run within a transaction
	return fmt.Errorf("migrations cannot be run within a transaction")
}

func (t *sqliteTransaction) BeginTx(_ context.Context) (service.Transaction, error) {
	// Nested transactions not supported
	return nil, fmt.Errorf("nested transactions not supported")
}
