// Package service defines the interfaces for all application services.
package service

import (
	"context"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Storage defines the contract for our persistence layer.
type Storage interface {
	// Transaction operations
	SaveTransactions(ctx context.Context, transactions []model.Transaction) error
	GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error)
	GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error)
	GetTransactionsByCategory(ctx context.Context, categoryName string) ([]model.Transaction, error)
	GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error)
	UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error
	UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error
	GetTransactionCount(ctx context.Context) (int, error)
	GetTransactionCountByCategory(ctx context.Context, categoryName string) (int, error)
	GetEarliestTransactionDate(ctx context.Context) (time.Time, error)
	GetLatestTransactionDate(ctx context.Context) (time.Time, error)
	GetCategorySummary(ctx context.Context, start, end time.Time) (map[string]float64, error)
	GetMerchantSummary(ctx context.Context, start, end time.Time) (map[string]float64, error)

	// Vendor operations
	GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error)
	SaveVendor(ctx context.Context, vendor *model.Vendor) error
	DeleteVendor(ctx context.Context, merchantName string) error
	GetAllVendors(ctx context.Context) ([]model.Vendor, error)
	GetVendorsByCategory(ctx context.Context, categoryName string) ([]model.Vendor, error)
	GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error)
	GetVendorsBySource(ctx context.Context, source model.VendorSource) ([]model.Vendor, error)
	DeleteVendorsBySource(ctx context.Context, source model.VendorSource) error
	UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error
	UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error
	FindVendorMatch(ctx context.Context, merchantName string) (*model.Vendor, error)

	// Classification operations
	SaveClassification(ctx context.Context, classification *model.Classification) error
	GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error)
	GetClassificationsByConfidence(ctx context.Context, maxConfidence float64, excludeUserModified bool) ([]model.Classification, error)
	ClearAllClassifications(ctx context.Context) error

	// Category operations
	GetCategories(ctx context.Context) ([]model.Category, error)
	GetCategoryByName(ctx context.Context, name string) (*model.Category, error)
	CreateCategory(ctx context.Context, name, description string) (*model.Category, error)
	CreateCategoryWithType(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error)
	UpdateCategory(ctx context.Context, id int, name, description string) error
	UpdateCategoryBusinessPercent(ctx context.Context, id int, businessPercent int) error
	DeleteCategory(ctx context.Context, id int) error

	// Check pattern operations
	CreateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error
	GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error)
	GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error)
	GetMatchingCheckPatterns(ctx context.Context, txn model.Transaction) ([]model.CheckPattern, error)
	UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error
	DeleteCheckPattern(ctx context.Context, id int64) error
	IncrementCheckPatternUseCount(ctx context.Context, id int64) error

	// Pattern rule operations
	CreatePatternRule(ctx context.Context, rule *model.PatternRule) error
	GetPatternRule(ctx context.Context, id int) (*model.PatternRule, error)
	GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error)
	UpdatePatternRule(ctx context.Context, rule *model.PatternRule) error
	DeletePatternRule(ctx context.Context, id int) error
	IncrementPatternRuleUseCount(ctx context.Context, id int) error
	GetPatternRulesByCategory(ctx context.Context, category string) ([]model.PatternRule, error)

	// Database management
	Migrate(ctx context.Context) error
	BeginTx(ctx context.Context) (Transaction, error)
	Close() error
}

// Transaction represents a database transaction.
type Transaction interface {
	Commit() error
	Rollback() error
	// Include all Storage methods for use within transaction
	Storage
}

// LLMSuggestion represents a single classification suggestion.
type LLMSuggestion struct {
	TransactionID       string
	Category            string
	CategoryDescription string
	Confidence          float64
	IsNew               bool
}

// CompletionStats shows the results of a classification run.
type CompletionStats struct {
	TotalTransactions int
	AutoClassified    int
	UserClassified    int
	NewVendorRules    int
	Duration          time.Duration
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

// RetryOptions configures retry behavior for operations.
type RetryOptions struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// ReportWriter defines the contract for output generation.
