package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// TestStorageValidation tests that validation is applied at the storage layer.
func TestStorageValidation(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()

	t.Run("nil context validation", func(t *testing.T) {
		// Test all methods with nil context
		// These tests intentionally pass nil to verify validation
		//nolint:staticcheck
		txns := []model.Transaction{{ID: "test", Date: time.Now(), Name: "Test", AccountID: "acc1"}}

		if err := store.SaveTransactions(nil, txns); err == nil || !strings.Contains(err.Error(), "context cannot be nil") { //nolint:staticcheck
			t.Errorf("SaveTransactions should fail with nil context, got: %v", err)
		}

		if _, err := store.GetTransactionsToClassify(nil, nil); err == nil || !strings.Contains(err.Error(), "context cannot be nil") { //nolint:staticcheck
			t.Errorf("GetTransactionsToClassify should fail with nil context, got: %v", err)
		}

		if _, err := store.GetTransactionByID(nil, "id"); err == nil || !strings.Contains(err.Error(), "context cannot be nil") { //nolint:staticcheck
			t.Errorf("GetTransactionByID should fail with nil context, got: %v", err)
		}

		if _, err := store.BeginTx(nil); err == nil || !strings.Contains(err.Error(), "context cannot be nil") { //nolint:staticcheck
			t.Errorf("BeginTx should fail with nil context, got: %v", err)
		}

		if err := store.Migrate(nil); err == nil || !strings.Contains(err.Error(), "context cannot be nil") { //nolint:staticcheck
			t.Errorf("Migrate should fail with nil context, got: %v", err)
		}
	})

	t.Run("empty string validation", func(t *testing.T) {
		ctx := context.Background()

		// GetTransactionByID with empty ID
		if _, err := store.GetTransactionByID(ctx, ""); err == nil || !strings.Contains(err.Error(), "string parameter cannot be empty") {
			t.Errorf("GetTransactionByID should fail with empty ID, got: %v", err)
		}

		// GetVendor with empty name
		if _, err := store.GetVendor(ctx, ""); err == nil || !strings.Contains(err.Error(), "string parameter cannot be empty") {
			t.Errorf("GetVendor should fail with empty merchantName, got: %v", err)
		}

		// DeleteVendor with empty name
		if err := store.DeleteVendor(ctx, "   "); err == nil || !strings.Contains(err.Error(), "string parameter cannot be empty") {
			t.Errorf("DeleteVendor should fail with whitespace merchantName, got: %v", err)
		}
	})

	t.Run("nil parameter validation", func(t *testing.T) {
		ctx := context.Background()

		// SaveVendor with nil vendor
		if err := store.SaveVendor(ctx, nil); err == nil || !strings.Contains(err.Error(), "parameter cannot be nil") {
			t.Errorf("SaveVendor should fail with nil vendor, got: %v", err)
		}

		// SaveClassification with nil classification
		if err := store.SaveClassification(ctx, nil); err == nil || !strings.Contains(err.Error(), "parameter cannot be nil") {
			t.Errorf("SaveClassification should fail with nil classification, got: %v", err)
		}

		// SaveProgress with nil progress
		if err := store.SaveProgress(ctx, nil); err == nil || !strings.Contains(err.Error(), "parameter cannot be nil") {
			t.Errorf("SaveProgress should fail with nil progress, got: %v", err)
		}
	})

	t.Run("invalid transaction validation", func(t *testing.T) {
		ctx := context.Background()

		// Transaction with missing fields
		invalidTxns := []model.Transaction{
			{Date: time.Now(), Name: "Test", AccountID: "acc1"}, // Missing ID
		}

		if err := store.SaveTransactions(ctx, invalidTxns); err == nil || !strings.Contains(err.Error(), "missing ID") {
			t.Errorf("SaveTransactions should fail with missing ID, got: %v", err)
		}

		// Transaction with zero date
		invalidTxns = []model.Transaction{
			{ID: "test", Name: "Test", AccountID: "acc1"}, // Zero date
		}

		if err := store.SaveTransactions(ctx, invalidTxns); err == nil || !strings.Contains(err.Error(), "missing date") {
			t.Errorf("SaveTransactions should fail with zero date, got: %v", err)
		}
	})

	t.Run("invalid vendor validation", func(t *testing.T) {
		ctx := context.Background()

		// Vendor with empty name
		vendor := &model.Vendor{Name: "   ", Category: "Food"}
		if err := store.SaveVendor(ctx, vendor); err == nil || !strings.Contains(err.Error(), "missing name") {
			t.Errorf("SaveVendor should fail with whitespace name, got: %v", err)
		}

		// Vendor with empty category
		vendor = &model.Vendor{Name: "Test", Category: ""}
		if err := store.SaveVendor(ctx, vendor); err == nil || !strings.Contains(err.Error(), "missing category") {
			t.Errorf("SaveVendor should fail with empty category, got: %v", err)
		}
	})

	t.Run("invalid classification validation", func(t *testing.T) {
		ctx := context.Background()

		// First save a valid transaction
		validTxn := model.Transaction{
			ID:        "test-txn",
			Date:      time.Now(),
			Name:      "Test Transaction",
			AccountID: "acc1",
		}
		validTxn.Hash = validTxn.GenerateHash()
		if err := store.SaveTransactions(ctx, []model.Transaction{validTxn}); err != nil {
			t.Fatalf("Failed to save test transaction: %v", err)
		}

		// Classification with invalid status
		classification := &model.Classification{
			Transaction: validTxn,
			Category:    "Food",
			Status:      "INVALID_STATUS",
			Confidence:  0.5,
		}

		if err := store.SaveClassification(ctx, classification); err == nil || !strings.Contains(err.Error(), "invalid classification status") {
			t.Errorf("SaveClassification should fail with invalid status, got: %v", err)
		}

		// Classification with invalid confidence
		classification = &model.Classification{
			Transaction: validTxn,
			Category:    "Food",
			Status:      model.StatusUserModified,
			Confidence:  1.5, // > 1
		}

		if err := store.SaveClassification(ctx, classification); err == nil || !strings.Contains(err.Error(), "confidence must be between 0 and 1") {
			t.Errorf("SaveClassification should fail with confidence > 1, got: %v", err)
		}
	})

	t.Run("date range validation", func(t *testing.T) {
		ctx := context.Background()

		// End date before start date
		start := time.Now()
		end := start.Add(-24 * time.Hour)

		if _, err := store.GetClassificationsByDateRange(ctx, start, end); err == nil || !strings.Contains(err.Error(), "start date must be before end date") {
			t.Errorf("GetClassificationsByDateRange should fail when end is before start, got: %v", err)
		}
	})
}

// TestTransactionValidation tests transaction-specific validation within a transaction.
func TestTransactionValidation(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Begin a transaction
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	t.Run("transaction methods validate inputs", func(t *testing.T) {
		// Test nil context
		if err := tx.SaveTransactions(context.Background(), []model.Transaction{}); err == nil || !strings.Contains(err.Error(), "slice cannot be empty") {
			t.Errorf("Transaction.SaveTransactions should validate empty slice, got: %v", err)
		}

		// Test empty transactions
		if err := tx.SaveTransactions(ctx, []model.Transaction{}); err == nil || !strings.Contains(err.Error(), "slice cannot be empty") {
			t.Errorf("Transaction.SaveTransactions should validate empty slice, got: %v", err)
		}

		// Test invalid vendor
		if err := tx.SaveVendor(ctx, &model.Vendor{Name: "", Category: "Food"}); err == nil || !strings.Contains(err.Error(), "missing name") {
			t.Errorf("Transaction.SaveVendor should validate vendor, got: %v", err)
		}
	})
}
