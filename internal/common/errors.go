// Package common provides shared utilities and types used across the application.
package common

import (
	"context"
	"errors"
	"fmt"
)

// Common application errors.
var (
	// Database errors.
	ErrNotFound          = errors.New("not found")
	ErrDuplicateEntry    = errors.New("duplicate entry")
	ErrDatabaseCorrupted = errors.New("database corrupted")

	// Plaid errors.
	ErrPlaidConnection = errors.New("plaid connection failed")
	ErrPlaidRateLimit  = errors.New("plaid rate limit exceeded")
	ErrInvalidAccount  = errors.New("invalid account")

	// Classification errors.
	ErrNoTransactions       = errors.New("no transactions to classify")
	ErrClassificationFailed = errors.New("classification failed")

	// Configuration errors.
	ErrMissingConfig = errors.New("missing configuration")
	ErrInvalidConfig = errors.New("invalid configuration")
)

// UserError represents an error that should be shown to the user.
type UserError struct {
	Err         error
	UserMessage string
}

func (e *UserError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.UserMessage, e.Err)
	}
	return e.UserMessage
}

func (e *UserError) Unwrap() error {
	return e.Err
}

// NewUserError creates a new user-friendly error.
func NewUserError(userMessage string, err error) error {
	return &UserError{
		UserMessage: userMessage,
		Err:         err,
	}
}

// IsRetryable determines if an error should trigger a retry.
func IsRetryable(err error) bool {
	// Check for specific retryable errors
	if errors.Is(err, ErrRateLimit) ||
		errors.Is(err, ErrPlaidRateLimit) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return true
	}

	// Check for retryable error type
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.Retryable
	}

	return false
}
