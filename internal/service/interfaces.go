// Package service defines the interfaces for all application services.
package service

import (
	"context"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// PlaidClient defines the contract for fetching data from Plaid.
type PlaidClient interface {
	GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error)
	GetAccounts(ctx context.Context) ([]string, error)
}

// Storage defines the contract for our persistence layer.
type Storage interface {
	// Transaction operations
	SaveTransactions(ctx context.Context, transactions []model.Transaction) error
	GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error)
	GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error)

	// Vendor operations
	GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error)
	SaveVendor(ctx context.Context, vendor *model.Vendor) error
	GetAllVendors(ctx context.Context) ([]model.Vendor, error)

	// Classification operations
	SaveClassification(ctx context.Context, classification *model.Classification) error
	GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error)

	// Progress tracking
	SaveProgress(ctx context.Context, progress *model.ClassificationProgress) error
	GetLatestProgress(ctx context.Context) (*model.ClassificationProgress, error)

	// Database management
	Migrate(ctx context.Context) error
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction.
type Transaction interface {
	Commit() error
	Rollback() error
	// Include all Storage methods for use within transaction
	Storage
}

// LLMClassifier defines the contract for AI-based categorization.
type LLMClassifier interface {
	SuggestCategory(ctx context.Context, transaction model.Transaction) (category string, confidence float64, err error)
	BatchSuggestCategories(ctx context.Context, transactions []model.Transaction) ([]LLMSuggestion, error)
}

// LLMSuggestion represents a single classification suggestion.
type LLMSuggestion struct {
	TransactionID string
	Category      string
	Confidence    float64
}

// UserPrompter defines the contract for user interaction.
type UserPrompter interface {
	ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error)
	BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error)
	GetCompletionStats() CompletionStats
}

// CompletionStats shows the results of a classification run.
type CompletionStats struct {
	TotalTransactions int
	AutoClassified    int
	UserClassified    int
	NewVendorRules    int
	Duration          time.Duration
}

// ReportWriter defines the contract for output generation.
type ReportWriter interface {
	WriteReport(ctx context.Context, classifications []model.Classification, summary ReportSummary) error
	ExportToSheets(ctx context.Context, sheetID string, data []model.Classification) error
}

// ReportSummary contains aggregate information for the report.
type ReportSummary struct {
	DateRange    DateRange
	ByCategory   map[string]CategorySummary
	ClassifiedBy map[model.ClassificationStatus]int
	TotalAmount  float64
}

// DateRange represents a time period with start and end dates.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// CategorySummary contains aggregated statistics for a category.
type CategorySummary struct {
	Count  int
	Amount float64
}

// Retryable defines a common interface for retryable operations.
type Retryable interface {
	IsRetryable() bool
	RetryAfter() time.Duration
}

// RetryOptions configures retry behavior for operations.
type RetryOptions struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}
