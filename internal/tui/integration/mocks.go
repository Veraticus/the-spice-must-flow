package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// MockStorage implements service.Storage for testing.
type MockStorage struct {
	errorOnSave     error
	errorOnLoad     error
	classifications map[string]model.Classification
	transactions    []model.Transaction
	categories      []model.Category
	checkPatterns   []model.CheckPattern
	saveCallCount   int
	delayOnLoad     time.Duration
	mu              sync.RWMutex
}

// NewMockStorage creates a new mock storage.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		transactions:    []model.Transaction{},
		categories:      []model.Category{},
		checkPatterns:   []model.CheckPattern{},
		classifications: make(map[string]model.Classification),
	}
}

// SetTransactions sets the transactions to return.
func (m *MockStorage) SetTransactions(txns []model.Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions = txns
}

// SetCategories sets the categories to return.
func (m *MockStorage) SetCategories(cats []model.Category) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.categories = cats
}

// SetCheckPatterns sets the check patterns to return.
func (m *MockStorage) SetCheckPatterns(patterns []model.CheckPattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkPatterns = patterns
}

// SetLoadError causes GetTransactionsToClassify to return an error.
func (m *MockStorage) SetLoadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnLoad = err
}

// SetLoadDelay adds a delay to loading operations.
func (m *MockStorage) SetLoadDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delayOnLoad = d
}

// SetSaveError causes SaveClassification to return an error.
func (m *MockStorage) SetSaveError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnSave = err
}

// GetTransactionsToClassify implements service.Storage.
func (m *MockStorage) GetTransactionsToClassify(ctx context.Context, _ *time.Time) ([]model.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.delayOnLoad > 0 {
		select {
		case <-time.After(m.delayOnLoad):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.errorOnLoad != nil {
		return nil, m.errorOnLoad
	}

	// Filter unclassified transactions
	var unclassified []model.Transaction
	for _, txn := range m.transactions {
		if len(txn.Category) == 0 {
			unclassified = append(unclassified, txn)
		}
	}

	return unclassified, nil
}

// GetCategories implements service.Storage.
func (m *MockStorage) GetCategories(_ context.Context) ([]model.Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.errorOnLoad != nil {
		return nil, m.errorOnLoad
	}

	return m.categories, nil
}

// SaveClassification implements service.Storage.
func (m *MockStorage) SaveClassification(_ context.Context, classification *model.Classification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.saveCallCount++

	if m.errorOnSave != nil {
		return m.errorOnSave
	}

	txnID := classification.Transaction.ID
	m.classifications[txnID] = *classification

	// Update transaction with classification
	for i, txn := range m.transactions {
		if txn.ID == txnID {
			m.transactions[i].Category = []string{classification.Category}
			break
		}
	}

	return nil
}

// GetSaveCallCount returns the number of times SaveClassification was called.
func (m *MockStorage) GetSaveCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveCallCount
}

// GetClassification returns a saved classification.
func (m *MockStorage) GetClassification(txnID string) (model.Classification, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.classifications[txnID]
	return c, ok
}

// DeleteClassification removes a classification (for undo).
func (m *MockStorage) DeleteClassification(_ context.Context, txnID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.classifications, txnID)
	return nil
}

// SaveTransactions saves transactions to the mock storage.
func (m *MockStorage) SaveTransactions(_ context.Context, transactions []model.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions = append(m.transactions, transactions...)
	return nil
}

// GetTransactionByID retrieves a transaction by its ID.
func (m *MockStorage) GetTransactionByID(_ context.Context, id string) (*model.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, txn := range m.transactions {
		if txn.ID == id {
			return &txn, nil
		}
	}
	return nil, fmt.Errorf("transaction not found")
}

// GetTransactionsByCategory returns transactions for a given category.
func (m *MockStorage) GetTransactionsByCategory(_ context.Context, _ string) ([]model.Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetTransactionsByCategoryID returns transactions for a given category ID.
func (m *MockStorage) GetTransactionsByCategoryID(_ context.Context, _ int) ([]model.Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

// UpdateTransactionCategories updates all transactions from one category to another.
func (m *MockStorage) UpdateTransactionCategories(_ context.Context, _, _ string) error {
	return fmt.Errorf("not implemented")
}

// UpdateTransactionCategoriesByID updates all transactions from one category ID to another.
func (m *MockStorage) UpdateTransactionCategoriesByID(_ context.Context, _, _ int) error {
	return fmt.Errorf("not implemented")
}

// GetTransactions returns all transactions matching the filter.
func (m *MockStorage) GetTransactions(_ context.Context, _ service.TransactionFilter) ([]model.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transactions, nil
}

// UpdateTransactionDirection updates the direction of a transaction.
func (m *MockStorage) UpdateTransactionDirection(_ context.Context, _ string, _ model.TransactionDirection) error {
	return fmt.Errorf("not implemented")
}

// GetIncomeByPeriod returns income transactions for a time period.
func (m *MockStorage) GetIncomeByPeriod(_ context.Context, _, _ time.Time) ([]model.Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetExpensesByPeriod returns expense transactions for a time period.
func (m *MockStorage) GetExpensesByPeriod(_ context.Context, _, _ time.Time) ([]model.Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetCashFlow returns cash flow summary for a time period.
func (m *MockStorage) GetCashFlow(_ context.Context, _, _ time.Time) (*service.CashFlowSummary, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetVendor retrieves a vendor by merchant name.
func (m *MockStorage) GetVendor(_ context.Context, _ string) (*model.Vendor, error) {
	return nil, fmt.Errorf("not implemented")
}

// SaveVendor saves a vendor to storage.
func (m *MockStorage) SaveVendor(_ context.Context, _ *model.Vendor) error {
	return fmt.Errorf("not implemented")
}

// GetAllVendors returns all vendors.
func (m *MockStorage) GetAllVendors(_ context.Context) ([]model.Vendor, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetVendorsByCategory returns vendors for a given category.
func (m *MockStorage) GetVendorsByCategory(_ context.Context, _ string) ([]model.Vendor, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetVendorsByCategoryID returns vendors for a given category ID.
func (m *MockStorage) GetVendorsByCategoryID(_ context.Context, _ int) ([]model.Vendor, error) {
	return nil, fmt.Errorf("not implemented")
}

// UpdateVendorCategories updates all vendors from one category to another.
func (m *MockStorage) UpdateVendorCategories(_ context.Context, _, _ string) error {
	return fmt.Errorf("not implemented")
}

// UpdateVendorCategoriesByID updates all vendors from one category ID to another.
func (m *MockStorage) UpdateVendorCategoriesByID(_ context.Context, _, _ int) error {
	return fmt.Errorf("not implemented")
}

// GetClassificationsByDateRange returns classifications for a date range.
func (m *MockStorage) GetClassificationsByDateRange(_ context.Context, _, _ time.Time) ([]model.Classification, error) {
	return nil, fmt.Errorf("not implemented")
}

// SaveProgress saves classification progress.
func (m *MockStorage) SaveProgress(_ context.Context, _ *model.ClassificationProgress) error {
	return fmt.Errorf("not implemented")
}

// GetLatestProgress returns the latest classification progress.
func (m *MockStorage) GetLatestProgress(_ context.Context) (*model.ClassificationProgress, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetCategoryByName retrieves a category by name.
func (m *MockStorage) GetCategoryByName(_ context.Context, name string) (*model.Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, cat := range m.categories {
		if cat.Name == name {
			return &cat, nil
		}
	}
	return nil, fmt.Errorf("category not found")
}

// GetCategoryByID retrieves a category by ID.
func (m *MockStorage) GetCategoryByID(_ context.Context, id int) (*model.Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, cat := range m.categories {
		if cat.ID == id {
			return &cat, nil
		}
	}
	return nil, fmt.Errorf("category not found")
}

// CreateCategory creates a new category.
func (m *MockStorage) CreateCategory(_ context.Context, name, _ string, _ model.CategoryType) (*model.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a new category
	newCat := &model.Category{
		ID:   len(m.categories) + 1,
		Name: name,
	}
	m.categories = append(m.categories, *newCat)
	return newCat, nil
}

// UpdateCategory updates an existing category.
func (m *MockStorage) UpdateCategory(_ context.Context, _ int, _, _ string) error {
	return fmt.Errorf("not implemented")
}

// DeleteCategory deletes a category by ID.
func (m *MockStorage) DeleteCategory(_ context.Context, _ int) error {
	return fmt.Errorf("not implemented")
}

// CreateCheckPattern creates a new check pattern.
func (m *MockStorage) CreateCheckPattern(_ context.Context, _ *model.CheckPattern) error {
	return fmt.Errorf("not implemented")
}

// GetCheckPattern retrieves a check pattern by ID.
func (m *MockStorage) GetCheckPattern(_ context.Context, _ int64) (*model.CheckPattern, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetActiveCheckPatterns returns all active check patterns.
func (m *MockStorage) GetActiveCheckPatterns(_ context.Context) ([]model.CheckPattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checkPatterns, nil
}

// GetMatchingCheckPatterns returns check patterns matching a transaction.
func (m *MockStorage) GetMatchingCheckPatterns(_ context.Context, _ model.Transaction) ([]model.CheckPattern, error) {
	return nil, fmt.Errorf("not implemented")
}

// UpdateCheckPattern updates an existing check pattern.
func (m *MockStorage) UpdateCheckPattern(_ context.Context, _ *model.CheckPattern) error {
	return fmt.Errorf("not implemented")
}

// DeleteCheckPattern deletes a check pattern by ID.
func (m *MockStorage) DeleteCheckPattern(_ context.Context, _ int64) error {
	return fmt.Errorf("not implemented")
}

// IncrementCheckPatternUseCount increments the use count of a check pattern.
func (m *MockStorage) IncrementCheckPatternUseCount(_ context.Context, _ int64) error {
	return fmt.Errorf("not implemented")
}

// Migrate runs database migrations.
func (m *MockStorage) Migrate(_ context.Context) error {
	return nil
}

// BeginTx starts a new transaction.
func (m *MockStorage) BeginTx(_ context.Context) (service.Transaction, error) {
	return &MockTransaction{storage: m}, nil
}

// Close closes the storage connection.
func (m *MockStorage) Close() error {
	return nil
}

// MockTransaction implements service.Transaction.
type MockTransaction struct {
	storage *MockStorage
}

// Commit commits the transaction.
func (t *MockTransaction) Commit() error {
	return nil
}

// Rollback rolls back the transaction.
func (t *MockTransaction) Rollback() error {
	return nil
}

// SaveTransactions saves transactions within the transaction.
func (t *MockTransaction) SaveTransactions(ctx context.Context, transactions []model.Transaction) error {
	return t.storage.SaveTransactions(ctx, transactions)
}

// GetTransactionsToClassify returns transactions needing classification.
func (t *MockTransaction) GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error) {
	return t.storage.GetTransactionsToClassify(ctx, fromDate)
}

// GetTransactionByID retrieves a transaction by ID.
func (t *MockTransaction) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	return t.storage.GetTransactionByID(ctx, id)
}

// GetTransactionsByCategory returns transactions for a category.
func (t *MockTransaction) GetTransactionsByCategory(ctx context.Context, category string) ([]model.Transaction, error) {
	return t.storage.GetTransactionsByCategory(ctx, category)
}

// GetTransactionsByCategoryID returns transactions for a category ID.
func (t *MockTransaction) GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error) {
	return t.storage.GetTransactionsByCategoryID(ctx, categoryID)
}

// UpdateTransactionCategories updates transaction categories.
func (t *MockTransaction) UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error {
	return t.storage.UpdateTransactionCategories(ctx, fromCategory, toCategory)
}

// UpdateTransactionCategoriesByID updates transaction categories by ID.
func (t *MockTransaction) UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	return t.storage.UpdateTransactionCategoriesByID(ctx, fromCategoryID, toCategoryID)
}

// GetTransactions returns transactions matching the filter.
func (t *MockTransaction) GetTransactions(ctx context.Context, filter service.TransactionFilter) ([]model.Transaction, error) {
	return t.storage.GetTransactions(ctx, filter)
}

// UpdateTransactionDirection updates the direction of a transaction.
func (t *MockTransaction) UpdateTransactionDirection(ctx context.Context, transactionID string, direction model.TransactionDirection) error {
	return t.storage.UpdateTransactionDirection(ctx, transactionID, direction)
}

// GetIncomeByPeriod returns income transactions for a time period.
func (t *MockTransaction) GetIncomeByPeriod(ctx context.Context, start, end time.Time) ([]model.Transaction, error) {
	return t.storage.GetIncomeByPeriod(ctx, start, end)
}

// GetExpensesByPeriod returns expense transactions for a time period.
func (t *MockTransaction) GetExpensesByPeriod(ctx context.Context, start, end time.Time) ([]model.Transaction, error) {
	return t.storage.GetExpensesByPeriod(ctx, start, end)
}

// GetCashFlow returns cash flow summary for a time period.
func (t *MockTransaction) GetCashFlow(ctx context.Context, start, end time.Time) (*service.CashFlowSummary, error) {
	return t.storage.GetCashFlow(ctx, start, end)
}

// GetVendor retrieves a vendor by merchant name.
func (t *MockTransaction) GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	return t.storage.GetVendor(ctx, merchantName)
}

// SaveVendor saves a vendor to storage.
func (t *MockTransaction) SaveVendor(ctx context.Context, vendor *model.Vendor) error {
	return t.storage.SaveVendor(ctx, vendor)
}

// GetAllVendors returns all vendors.
func (t *MockTransaction) GetAllVendors(ctx context.Context) ([]model.Vendor, error) {
	return t.storage.GetAllVendors(ctx)
}

// GetVendorsByCategory returns vendors for a given category.
func (t *MockTransaction) GetVendorsByCategory(ctx context.Context, category string) ([]model.Vendor, error) {
	return t.storage.GetVendorsByCategory(ctx, category)
}

// GetVendorsByCategoryID returns vendors for a given category ID.
func (t *MockTransaction) GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error) {
	return t.storage.GetVendorsByCategoryID(ctx, categoryID)
}

// UpdateVendorCategories updates all vendors from one category to another.
func (t *MockTransaction) UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error {
	return t.storage.UpdateVendorCategories(ctx, fromCategory, toCategory)
}

// UpdateVendorCategoriesByID updates all vendors from one category ID to another.
func (t *MockTransaction) UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	return t.storage.UpdateVendorCategoriesByID(ctx, fromCategoryID, toCategoryID)
}

// SaveClassification saves a classification.
func (t *MockTransaction) SaveClassification(ctx context.Context, classification *model.Classification) error {
	return t.storage.SaveClassification(ctx, classification)
}

// GetClassificationsByDateRange returns classifications for a date range.
func (t *MockTransaction) GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
	return t.storage.GetClassificationsByDateRange(ctx, start, end)
}

// SaveProgress saves classification progress.
func (t *MockTransaction) SaveProgress(ctx context.Context, progress *model.ClassificationProgress) error {
	return t.storage.SaveProgress(ctx, progress)
}

// GetLatestProgress returns the latest classification progress.
func (t *MockTransaction) GetLatestProgress(ctx context.Context) (*model.ClassificationProgress, error) {
	return t.storage.GetLatestProgress(ctx)
}

// GetCategories returns all categories.
func (t *MockTransaction) GetCategories(ctx context.Context) ([]model.Category, error) {
	return t.storage.GetCategories(ctx)
}

// GetCategoryByName retrieves a category by name.
func (t *MockTransaction) GetCategoryByName(ctx context.Context, name string) (*model.Category, error) {
	return t.storage.GetCategoryByName(ctx, name)
}

// GetCategoryByID retrieves a category by ID.
func (t *MockTransaction) GetCategoryByID(ctx context.Context, id int) (*model.Category, error) {
	return t.storage.GetCategoryByID(ctx, id)
}

// CreateCategory creates a new category.
func (t *MockTransaction) CreateCategory(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error) {
	return t.storage.CreateCategory(ctx, name, description, categoryType)
}

// UpdateCategory updates an existing category.
func (t *MockTransaction) UpdateCategory(ctx context.Context, id int, name, description string) error {
	return t.storage.UpdateCategory(ctx, id, name, description)
}

// DeleteCategory deletes a category by ID.
func (t *MockTransaction) DeleteCategory(ctx context.Context, id int) error {
	return t.storage.DeleteCategory(ctx, id)
}

// CreateCheckPattern creates a new check pattern.
func (t *MockTransaction) CreateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	return t.storage.CreateCheckPattern(ctx, pattern)
}

// GetCheckPattern retrieves a check pattern by ID.
func (t *MockTransaction) GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error) {
	return t.storage.GetCheckPattern(ctx, id)
}

// GetActiveCheckPatterns returns all active check patterns.
func (t *MockTransaction) GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	return t.storage.GetActiveCheckPatterns(ctx)
}

// GetMatchingCheckPatterns returns check patterns matching a transaction.
func (t *MockTransaction) GetMatchingCheckPatterns(ctx context.Context, txn model.Transaction) ([]model.CheckPattern, error) {
	return t.storage.GetMatchingCheckPatterns(ctx, txn)
}

// UpdateCheckPattern updates an existing check pattern.
func (t *MockTransaction) UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	return t.storage.UpdateCheckPattern(ctx, pattern)
}

// DeleteCheckPattern deletes a check pattern by ID.
func (t *MockTransaction) DeleteCheckPattern(ctx context.Context, id int64) error {
	return t.storage.DeleteCheckPattern(ctx, id)
}

// IncrementCheckPatternUseCount increments the use count of a check pattern.
func (t *MockTransaction) IncrementCheckPatternUseCount(ctx context.Context, id int64) error {
	return t.storage.IncrementCheckPatternUseCount(ctx, id)
}

// Migrate runs database migrations.
func (t *MockTransaction) Migrate(ctx context.Context) error {
	return t.storage.Migrate(ctx)
}

// BeginTx starts a new transaction.
func (t *MockTransaction) BeginTx(ctx context.Context) (service.Transaction, error) {
	return t.storage.BeginTx(ctx)
}

// Close closes the transaction.
func (t *MockTransaction) Close() error {
	return t.storage.Close()
}

// MockClassifier implements engine.Classifier for testing.
type MockClassifier struct {
	rankings          map[string]model.CategoryRankings
	errorOnClassify   error
	delayOnClassify   time.Duration
	classifyCallCount int
	mu                sync.RWMutex
}

// NewMockClassifier creates a new mock classifier.
func NewMockClassifier() *MockClassifier {
	return &MockClassifier{
		rankings: make(map[string]model.CategoryRankings),
	}
}

// SetRankings sets the rankings to return for a transaction.
func (m *MockClassifier) SetRankings(txnID string, rankings model.CategoryRankings) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rankings[txnID] = rankings
}

// SetDefaultRankings sets rankings for any transaction.
func (m *MockClassifier) SetDefaultRankings(rankings model.CategoryRankings) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rankings["*"] = rankings
}

// SetClassifyError causes ClassifyTransaction to return an error.
func (m *MockClassifier) SetClassifyError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnClassify = err
}

// SetClassifyDelay adds a delay to classification.
func (m *MockClassifier) SetClassifyDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delayOnClassify = d
}

// SuggestCategory implements engine.Classifier.
func (m *MockClassifier) SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (category string, confidence float64, isNew bool, description string, err error) {
	m.mu.Lock()
	m.classifyCallCount++
	delay := m.delayOnClassify
	classifyErr := m.errorOnClassify
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return "", 0, false, "", ctx.Err()
		}
	}

	if classifyErr != nil {
		return "", 0, false, "", classifyErr
	}

	// Use first category from rankings if available
	m.mu.RLock()
	defer m.mu.RUnlock()

	if rankings, ok := m.rankings[transaction.ID]; ok && len(rankings) > 0 {
		return rankings[0].Category, rankings[0].Score, rankings[0].IsNew, rankings[0].Description, nil
	}

	if rankings, ok := m.rankings["*"]; ok && len(rankings) > 0 {
		return rankings[0].Category, rankings[0].Score, rankings[0].IsNew, rankings[0].Description, nil
	}

	// Default to first category
	if len(categories) > 0 {
		return categories[0], 0.8, false, "", nil
	}

	return "", 0, false, "", fmt.Errorf("no categories available")
}

// SuggestCategoryRankings implements engine.Classifier.
func (m *MockClassifier) SuggestCategoryRankings(ctx context.Context, transaction model.Transaction, categories []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	m.mu.Lock()
	m.classifyCallCount++
	delay := m.delayOnClassify
	err := m.errorOnClassify
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for specific transaction rankings
	if rankings, ok := m.rankings[transaction.ID]; ok {
		return rankings, nil
	}

	// Check for default rankings
	if rankings, ok := m.rankings["*"]; ok {
		return rankings, nil
	}

	// Generate default rankings based on categories
	if len(categories) > 0 {
		var rankings model.CategoryRankings

		// Create simple rankings
		for i, cat := range categories {
			if i >= 5 {
				break // Top 5 only
			}

			confidence := 0.9 - (float64(i) * 0.15)
			if confidence < 0.1 {
				confidence = 0.1
			}

			rankings = append(rankings, model.CategoryRanking{
				Category:    cat.Name,
				Description: "",
				Score:       confidence,
				IsNew:       false,
			})
		}

		return rankings, nil
	}

	return nil, fmt.Errorf("no categories provided")
}

// BatchSuggestCategories implements engine.Classifier.
func (m *MockClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, 0, len(transactions))

	for _, txn := range transactions {
		cat, conf, isNew, desc, err := m.SuggestCategory(ctx, txn, categories)
		if err != nil {
			return nil, err
		}

		suggestions = append(suggestions, service.LLMSuggestion{
			TransactionID:       txn.ID,
			Category:            cat,
			CategoryDescription: desc,
			Confidence:          conf,
			IsNew:               isNew,
		})
	}

	return suggestions, nil
}

// GenerateCategoryDescription implements engine.Classifier.
func (m *MockClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (description string, confidence float64, err error) {
	return fmt.Sprintf("Mock description for %s", categoryName), 0.9, nil
}

// SuggestTransactionDirection implements engine.Classifier.
func (m *MockClassifier) SuggestTransactionDirection(_ context.Context, transaction model.Transaction) (direction model.TransactionDirection, confidence float64, reasoning string, err error) {
	if transaction.Amount > 0 {
		return model.DirectionIncome, 0.95, "Positive amount indicates income", nil
	}
	return model.DirectionExpense, 0.95, "Negative amount indicates expense", nil
}

// GetClassifyCallCount returns the number of times ClassifyTransaction was called.
func (m *MockClassifier) GetClassifyCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.classifyCallCount
}
