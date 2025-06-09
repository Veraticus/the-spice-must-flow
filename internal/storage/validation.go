// Package storage provides the data persistence layer for the spice application.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// Validation errors.
var (
	ErrNilContext            = errors.New("context cannot be nil")
	ErrEmptyString           = errors.New("string parameter cannot be empty")
	ErrNilParameter          = errors.New("parameter cannot be nil")
	ErrEmptySlice            = errors.New("slice cannot be empty")
	ErrInvalidDateRange      = errors.New("start date must be before end date")
	ErrInvalidStatus         = errors.New("invalid classification status")
	ErrInvalidTransaction    = errors.New("invalid transaction")
	ErrInvalidVendor         = errors.New("invalid vendor")
	ErrInvalidClassification = errors.New("invalid classification")
)

// validateContext ensures the context is not nil.
func validateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	return nil
}

// validateString ensures a string parameter is not empty.
func validateString(s string, paramName string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%w: %s", ErrEmptyString, paramName)
	}
	return nil
}

// validateTransactions validates a slice of transactions.
func validateTransactions(transactions []model.Transaction) error {
	if transactions == nil {
		return fmt.Errorf("%w: transactions", ErrNilParameter)
	}
	if len(transactions) == 0 {
		return fmt.Errorf("%w: transactions", ErrEmptySlice)
	}

	for i, txn := range transactions {
		if err := validateTransaction(&txn); err != nil {
			return fmt.Errorf("transaction at index %d: %w", i, err)
		}
	}
	return nil
}

// validateTransaction validates a single transaction.
func validateTransaction(txn *model.Transaction) error {
	if txn == nil {
		return fmt.Errorf("%w: transaction", ErrNilParameter)
	}
	if txn.ID == "" {
		return fmt.Errorf("%w: missing ID", ErrInvalidTransaction)
	}
	if txn.Date.IsZero() {
		return fmt.Errorf("%w: missing date", ErrInvalidTransaction)
	}
	if txn.Name == "" {
		return fmt.Errorf("%w: missing name", ErrInvalidTransaction)
	}
	if txn.AccountID == "" {
		return fmt.Errorf("%w: missing account ID", ErrInvalidTransaction)
	}
	return nil
}

// validateVendor validates a vendor.
func validateVendor(vendor *model.Vendor) error {
	if vendor == nil {
		return fmt.Errorf("%w: vendor", ErrNilParameter)
	}
	if strings.TrimSpace(vendor.Name) == "" {
		return fmt.Errorf("%w: missing name", ErrInvalidVendor)
	}
	if strings.TrimSpace(vendor.Category) == "" {
		return fmt.Errorf("%w: missing category", ErrInvalidVendor)
	}
	return nil
}

// validateClassification validates a classification.
func validateClassification(classification *model.Classification) error {
	if classification == nil {
		return fmt.Errorf("%w: classification", ErrNilParameter)
	}
	if err := validateTransaction(&classification.Transaction); err != nil {
		return fmt.Errorf("classification transaction: %w", err)
	}
	if strings.TrimSpace(classification.Category) == "" {
		return fmt.Errorf("%w: missing category", ErrInvalidClassification)
	}

	// Validate status
	switch classification.Status {
	case model.StatusUnclassified,
		model.StatusClassifiedByRule,
		model.StatusClassifiedByAI,
		model.StatusUserModified:
		// Valid status
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStatus, classification.Status)
	}

	// Validate confidence is between 0 and 1
	if classification.Confidence < 0 || classification.Confidence > 1 {
		return fmt.Errorf("%w: confidence must be between 0 and 1", ErrInvalidClassification)
	}

	return nil
}

// validateProgress validates classification progress.
func validateProgress(progress *model.ClassificationProgress) error {
	if progress == nil {
		return fmt.Errorf("%w: progress", ErrNilParameter)
	}
	// Progress fields are optional, but dates should not be zero if set
	return nil
}
