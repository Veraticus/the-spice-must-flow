package engine

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassificationEngine_ClassifyTransactions(t *testing.T) {
	tests := []struct {
		setupStorage      func(*testing.T) service.Storage
		setupProgress     *model.ClassificationProgress
		name              string
		setupTransactions []model.Transaction
		setupVendors      []model.Vendor
		expectedStats     service.CompletionStats
		llmAutoAccept     bool
		wantErr           bool
	}{
		{
			name: "classify new transactions with no existing vendors",
			setupStorage: func(t *testing.T) service.Storage {
				t.Helper()
				db, err := storage.NewSQLiteStorage(":memory:")
				require.NoError(t, err)
				require.NoError(t, db.Migrate(context.Background()))
				return db
			},
			setupTransactions: []model.Transaction{
				{
					ID:           "1",
					Hash:         "hash1",
					Date:         time.Now().AddDate(0, 0, -1),
					Name:         "STARBUCKS STORE #123",
					MerchantName: "Starbucks",
					Amount:       5.75,
					AccountID:    "acc1",
				},
				{
					ID:           "2",
					Hash:         "hash2",
					Date:         time.Now().AddDate(0, 0, -2),
					Name:         "STARBUCKS STORE #456",
					MerchantName: "Starbucks",
					Amount:       6.25,
					AccountID:    "acc1",
				},
			},
			llmAutoAccept: true,
			expectedStats: service.CompletionStats{
				TotalTransactions: 2,
				AutoClassified:    2,
				UserClassified:    0,
				NewVendorRules:    1,
			},
			wantErr: false,
		},
		{
			name: "apply existing vendor rules",
			setupStorage: func(t *testing.T) service.Storage {
				t.Helper()
				db, err := storage.NewSQLiteStorage(":memory:")
				require.NoError(t, err)
				require.NoError(t, db.Migrate(context.Background()))
				return db
			},
			setupTransactions: []model.Transaction{
				{
					ID:           "3",
					Hash:         "hash3",
					Date:         time.Now().AddDate(0, 0, -1),
					Name:         "WHOLE FOODS MARKET",
					MerchantName: "Whole Foods Market",
					Amount:       125.50,
					AccountID:    "acc1",
				},
			},
			setupVendors: []model.Vendor{
				{
					Name:        "Whole Foods Market",
					Category:    "Groceries",
					LastUpdated: time.Now().AddDate(0, -1, 0),
					UseCount:    10,
				},
			},
			llmAutoAccept: true,
			expectedStats: service.CompletionStats{
				TotalTransactions: 0, // Applied by rule, not processed by prompter
				AutoClassified:    0,
				UserClassified:    0,
				NewVendorRules:    0,
			},
			wantErr: false,
		},
		{
			name: "resume from saved progress",
			setupStorage: func(t *testing.T) service.Storage {
				t.Helper()
				db, err := storage.NewSQLiteStorage(":memory:")
				require.NoError(t, err)
				require.NoError(t, db.Migrate(context.Background()))
				return db
			},
			setupTransactions: []model.Transaction{
				{
					ID:           "4",
					Hash:         "hash4",
					Date:         time.Now().AddDate(0, 0, -3),
					Name:         "OLD TRANSACTION",
					MerchantName: "Old Merchant",
					Amount:       50.00,
					AccountID:    "acc1",
				},
				{
					ID:           "5",
					Hash:         "hash5",
					Date:         time.Now().AddDate(0, 0, -1),
					Name:         "NEW TRANSACTION",
					MerchantName: "New Merchant",
					Amount:       75.00,
					AccountID:    "acc1",
				},
			},
			setupProgress: &model.ClassificationProgress{
				LastProcessedID:   "4",
				LastProcessedDate: time.Now().AddDate(0, 0, -2),
				TotalProcessed:    1,
				StartedAt:         time.Now().AddDate(0, 0, -1),
			},
			llmAutoAccept: true,
			expectedStats: service.CompletionStats{
				TotalTransactions: 1, // Only new transaction
				AutoClassified:    1,
				UserClassified:    0,
				NewVendorRules:    1,
			},
			wantErr: false,
		},
		{
			name: "high variance merchant detection",
			setupStorage: func(t *testing.T) service.Storage {
				t.Helper()
				db, err := storage.NewSQLiteStorage(":memory:")
				require.NoError(t, err)
				require.NoError(t, db.Migrate(context.Background()))
				return db
			},
			setupTransactions: []model.Transaction{
				{
					ID:           "6",
					Hash:         "hash6",
					Date:         time.Now().AddDate(0, 0, -1),
					Name:         "AMAZON.COM",
					MerchantName: "Amazon",
					Amount:       5.99,
					AccountID:    "acc1",
				},
				{
					ID:           "7",
					Hash:         "hash7",
					Date:         time.Now().AddDate(0, 0, -2),
					Name:         "AMAZON.COM",
					MerchantName: "Amazon",
					Amount:       15.99,
					AccountID:    "acc1",
				},
				{
					ID:           "8",
					Hash:         "hash8",
					Date:         time.Now().AddDate(0, 0, -3),
					Name:         "AMAZON.COM",
					MerchantName: "Amazon",
					Amount:       25.99,
					AccountID:    "acc1",
				},
				{
					ID:           "9",
					Hash:         "hash9",
					Date:         time.Now().AddDate(0, 0, -4),
					Name:         "AMAZON.COM",
					MerchantName: "Amazon",
					Amount:       99.99,
					AccountID:    "acc1",
				},
				{
					ID:           "10",
					Hash:         "hash10",
					Date:         time.Now().AddDate(0, 0, -5),
					Name:         "AMAZON.COM",
					MerchantName: "Amazon",
					Amount:       499.99,
					AccountID:    "acc1",
				},
			},
			llmAutoAccept: true,
			expectedStats: service.CompletionStats{
				TotalTransactions: 5,
				AutoClassified:    5, // Individual review but auto-accepted
				UserClassified:    0,
				NewVendorRules:    0, // No vendor rule for high-variance
			},
			wantErr: false,
		},
		{
			name: "empty transaction list",
			setupStorage: func(t *testing.T) service.Storage {
				t.Helper()
				db, err := storage.NewSQLiteStorage(":memory:")
				require.NoError(t, err)
				require.NoError(t, db.Migrate(context.Background()))
				return db
			},
			setupTransactions: []model.Transaction{},
			llmAutoAccept:     true,
			expectedStats: service.CompletionStats{
				TotalTransactions: 0,
				AutoClassified:    0,
				UserClassified:    0,
				NewVendorRules:    0,
			},
			wantErr: false,
		},
		{
			name: "auto-classification with high confidence",
			setupStorage: func(t *testing.T) service.Storage {
				t.Helper()
				db, err := storage.NewSQLiteStorage(":memory:")
				require.NoError(t, err)
				require.NoError(t, db.Migrate(context.Background()))
				return db
			},
			setupTransactions: []model.Transaction{
				{
					ID:           "11",
					Hash:         "hash11",
					Date:         time.Now().AddDate(0, 0, -1),
					Name:         "WHOLE FOODS MARKET #123",
					MerchantName: "Whole Foods Market",
					Amount:       89.45,
					AccountID:    "acc1",
				},
				{
					ID:           "12",
					Hash:         "hash12",
					Date:         time.Now().AddDate(0, 0, -2),
					Name:         "WHOLE FOODS MARKET #456",
					MerchantName: "Whole Foods Market",
					Amount:       125.67,
					AccountID:    "acc1",
				},
			},
			llmAutoAccept: true,
			// With auto-classification, prompter is never called
			expectedStats: service.CompletionStats{
				TotalTransactions: 0,
				AutoClassified:    0,
				UserClassified:    0,
				NewVendorRules:    0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup storage
			storage := tt.setupStorage(t)
			defer func() {
				// Close storage if it has a Close method
				if closer, ok := storage.(interface{ Close() error }); ok {
					_ = closer.Close()
				}
			}()

			// Create default categories required by the engine
			defaultCategories := []string{"Groceries", "Transportation", "Entertainment", "Shopping", "Dining"}
			for _, cat := range defaultCategories {
				_, err := storage.CreateCategory(ctx, cat, "Test category: "+cat)
				require.NoError(t, err)
			}

			// Insert test data
			if len(tt.setupTransactions) > 0 {
				err := storage.SaveTransactions(ctx, tt.setupTransactions)
				require.NoError(t, err)
			}

			for _, vendor := range tt.setupVendors {
				err := storage.SaveVendor(ctx, &vendor)
				require.NoError(t, err)
			}

			if tt.setupProgress != nil {
				err := storage.SaveProgress(ctx, tt.setupProgress)
				require.NoError(t, err)
			}

			// Create mocks
			llm := NewMockClassifier()
			prompter := NewMockPrompter(tt.llmAutoAccept)

			// Create engine
			engine := New(storage, llm, prompter)

			// Run classification
			err := engine.ClassifyTransactions(ctx, nil)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify statistics
			stats := prompter.GetCompletionStats()
			assert.Equal(t, tt.expectedStats.TotalTransactions, stats.TotalTransactions)
			assert.Equal(t, tt.expectedStats.AutoClassified, stats.AutoClassified)
			assert.Equal(t, tt.expectedStats.UserClassified, stats.UserClassified)
			assert.Equal(t, tt.expectedStats.NewVendorRules, stats.NewVendorRules)
		})
	}
}

func TestClassificationEngine_GroupByMerchant(t *testing.T) {
	engine := &ClassificationEngine{}

	transactions := []model.Transaction{
		{ID: "1", MerchantName: "Starbucks", Name: "STARBUCKS #123"},
		{ID: "2", MerchantName: "Starbucks", Name: "STARBUCKS #456"},
		{ID: "3", MerchantName: "", Name: "WHOLE FOODS"}, // No merchant name
		{ID: "4", MerchantName: "Amazon", Name: "AMAZON.COM"},
		{ID: "5", MerchantName: "Amazon", Name: "AMAZON PRIME"},
	}

	groups := engine.groupByMerchant(transactions)

	assert.Len(t, groups, 3)
	assert.Len(t, groups["Starbucks"], 2)
	assert.Len(t, groups["Amazon"], 2)
	assert.Len(t, groups["WHOLE FOODS"], 1) // Falls back to name
}

func TestClassificationEngine_SortMerchantsByVolume(t *testing.T) {
	engine := &ClassificationEngine{}

	groups := map[string][]model.Transaction{
		"LowVolume":    {{ID: "1"}},
		"MediumVolume": {{ID: "2"}, {ID: "3"}},
		"HighVolume":   {{ID: "4"}, {ID: "5"}, {ID: "6"}},
	}

	sorted := engine.sortMerchantsByVolume(groups)

	assert.Equal(t, []string{"HighVolume", "MediumVolume", "LowVolume"}, sorted)
}

func TestClassificationEngine_HasHighVariance(t *testing.T) {
	engine := &ClassificationEngine{}

	tests := []struct {
		name     string
		amounts  []float64
		expected bool
	}{
		{
			name:     "low variance",
			amounts:  []float64{5.00, 6.00, 7.00, 8.00, 9.00},
			expected: false,
		},
		{
			name:     "high variance",
			amounts:  []float64{5.00, 10.00, 15.00, 25.00, 100.00},
			expected: true,
		},
		{
			name:     "not enough transactions",
			amounts:  []float64{5.00, 500.00},
			expected: false,
		},
		{
			name:     "negative amounts",
			amounts:  []float64{-5.00, -10.00, -15.00, -25.00, -100.00},
			expected: true,
		},
		{
			name:     "zero minimum",
			amounts:  []float64{0.00, 50.00, 100.00, 150.00, 200.00},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactions := make([]model.Transaction, len(tt.amounts))
			for i, amount := range tt.amounts {
				transactions[i] = model.Transaction{
					ID:     string(rune(i)),
					Amount: amount,
				}
			}

			result := engine.hasHighVariance(transactions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClassificationEngine_VendorRetrieval(t *testing.T) {
	ctx := context.Background()

	// Create in-memory storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	// Create required category
	_, err = db.CreateCategory(ctx, "Test Category", "Test category for vendor")
	require.NoError(t, err)

	// Add vendor to storage
	vendor := &model.Vendor{
		Name:        "Test Vendor",
		Category:    "Test Category",
		LastUpdated: time.Now(),
		UseCount:    5,
	}
	require.NoError(t, db.SaveVendor(ctx, vendor))

	// Create engine
	engine := New(db, NewMockClassifier(), NewMockPrompter(true))

	// Test vendor retrieval (storage layer handles caching)
	retrievedVendor, err := engine.getVendor(ctx, "Test Vendor")
	require.NoError(t, err)
	assert.Equal(t, vendor.Name, retrievedVendor.Name)
	assert.Equal(t, vendor.Category, retrievedVendor.Category)

	// Test vendor not found
	notFound, err := engine.getVendor(ctx, "Nonexistent Vendor")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrNoRows))
	assert.Nil(t, notFound)
}

func TestClassificationEngine_ApplyVendorRule(t *testing.T) {
	engine := &ClassificationEngine{}

	vendor := &model.Vendor{
		Name:     "Starbucks",
		Category: "Coffee & Dining",
	}

	transactions := []model.Transaction{
		{ID: "1", MerchantName: "Starbucks", Amount: 5.75},
		{ID: "2", MerchantName: "Starbucks", Amount: 6.25},
	}

	classifications := engine.applyVendorRule(transactions, vendor)

	assert.Len(t, classifications, 2)
	for _, c := range classifications {
		assert.Equal(t, "Coffee & Dining", c.Category)
		assert.Equal(t, model.StatusClassifiedByRule, c.Status)
		assert.Equal(t, 1.0, c.Confidence)
		assert.False(t, c.ClassifiedAt.IsZero())
	}
}

func TestClassificationEngine_ContextCancellation(t *testing.T) {
	// Create storage with test data
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	// Create default categories
	ctx := context.Background()
	defaultCategories := []string{"Groceries", "Transportation", "Entertainment"}
	for _, cat := range defaultCategories {
		_, catErr := db.CreateCategory(ctx, cat, "Test category: "+cat)
		require.NoError(t, catErr)
	}

	// Add many transactions to ensure we can cancel mid-process
	transactions := make([]model.Transaction, 100)
	for i := 0; i < 100; i++ {
		transactions[i] = model.Transaction{
			ID:           string(rune(i)),
			Hash:         string(rune(i)),
			Date:         time.Now().AddDate(0, 0, -i),
			Name:         "MERCHANT",
			MerchantName: "Merchant",
			Amount:       float64(i),
			AccountID:    "acc1",
		}
	}
	require.NoError(t, db.SaveTransactions(context.Background(), transactions))

	// Create engine with slow prompter
	llm := NewMockClassifier()
	prompter := &slowMockPrompter{
		MockPrompter: NewMockPrompter(true),
		delay:        100 * time.Millisecond,
	}
	engine := New(db, llm, prompter)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start classification in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- engine.ClassifyTransactions(ctx, nil)
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Check error
	err = <-errChan
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))

	// Verify progress was saved or not needed
	progress, err := db.GetLatestProgress(context.Background())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Failed to get progress: %v", err)
	}
	// Progress may not be saved if canceled too quickly
	if progress != nil {
		assert.Greater(t, progress.TotalProcessed, 0)
	}
}

// slowMockPrompter adds delay to simulate slow user interaction.
type slowMockPrompter struct {
	*MockPrompter
	delay time.Duration
}

func (s *slowMockPrompter) BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.delay):
		return s.MockPrompter.BatchConfirmClassifications(ctx, pending)
	}
}

func TestClassificationEngine_RetryLogic(t *testing.T) {
	ctx := context.Background()

	// Create storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	// Create default categories
	defaultCategories := []string{"Groceries", "Transportation", "Test Category"}
	for _, cat := range defaultCategories {
		_, catErr := db.CreateCategory(ctx, cat, "Test category: "+cat)
		require.NoError(t, catErr)
	}

	// Add test transaction
	transaction := model.Transaction{
		ID:           "1",
		Hash:         "hash1",
		Date:         time.Now(),
		Name:         "TEST",
		MerchantName: "Test Merchant",
		Amount:       10.00,
		AccountID:    "acc1",
	}
	require.NoError(t, db.SaveTransactions(ctx, []model.Transaction{transaction}))

	// Create failing LLM classifier
	llm := &failingClassifier{
		failCount: 2, // Fail first 2 attempts
	}
	prompter := NewMockPrompter(true)

	// Create engine
	engine := New(db, llm, prompter)

	// Run classification - should succeed after retries
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Verify retry attempts
	assert.Equal(t, 3, llm.attempts) // 2 failures + 1 success
}

// failingClassifier simulates transient failures.
type failingClassifier struct {
	failCount int
	attempts  int
	mu        sync.Mutex
}

func (f *failingClassifier) SuggestCategory(_ context.Context, _ model.Transaction, _ []string) (string, float64, bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.attempts++
	if f.attempts <= f.failCount {
		return "", 0, false, "", errors.New("temporary failure")
	}

	return "Test Category", 0.85, false, "", nil
}

func (f *failingClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, len(transactions))
	for i, txn := range transactions {
		category, confidence, isNew, description, err := f.SuggestCategory(ctx, txn, categories)
		if err != nil {
			return nil, err
		}
		suggestions[i] = service.LLMSuggestion{
			TransactionID:       txn.ID,
			Category:            category,
			Confidence:          confidence,
			IsNew:               isNew,
			CategoryDescription: description,
		}
	}
	return suggestions, nil
}

func (f *failingClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (string, float64, error) {
	return "Test description for " + categoryName, 0.95, nil
}

func (f *failingClassifier) SuggestCategoryRankings(_ context.Context, transaction model.Transaction, _ []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	// Call SuggestCategory to maintain the same behavior
	cat, conf, isNew, desc, err := f.SuggestCategory(context.Background(), transaction, nil)
	if err != nil {
		return nil, err
	}

	// Return single ranking based on SuggestCategory result
	return model.CategoryRankings{
		{
			Category:    cat,
			Score:       conf,
			IsNew:       isNew,
			Description: desc,
		},
	}, nil
}

func (f *failingClassifier) SuggestCategoryBatch(_ context.Context, _ []llm.MerchantBatchRequest, _ []model.Category) (map[string]model.CategoryRankings, error) {
	return make(map[string]model.CategoryRankings), nil
}

// TestNewCategoryFlow tests the flow when AI suggests a new category.
func TestNewCategoryFlow(t *testing.T) {
	ctx := context.Background()

	// Setup storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed to close database: %v", closeErr)
		}
	}()

	// Create initial categories (the new category will be suggested by AI)
	initialCategories := []string{"Entertainment", "Shopping", "Dining"}
	for _, cat := range initialCategories {
		_, createErr := db.CreateCategory(ctx, cat, "Test category: "+cat)
		require.NoError(t, createErr)
	}

	// Setup transactions
	txns := []model.Transaction{
		{
			ID:           "1",
			Hash:         "hash1",
			Date:         time.Now(),
			Name:         "PELOTON SUBSCRIPTION",
			MerchantName: "Peloton",
			Amount:       39.99,
			AccountID:    "acc1",
		},
	}
	err = db.SaveTransactions(ctx, txns)
	require.NoError(t, err)

	// Create classifier that suggests a new category with low confidence
	llm := &newCategoryClassifier{
		suggestedCategory: "Fitness & Health",
		confidence:        0.75, // Below 0.9 threshold
		isNew:             true,
	}

	// Create prompter that accepts the new category
	prompter := &newCategoryPrompter{
		acceptNewCategory: true,
		acceptedCategory:  "Fitness & Health",
	}

	// Create engine
	engine := New(db, llm, prompter)

	// Run classification
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Verify the new category was created
	categories, err := db.GetCategories(ctx)
	require.NoError(t, err)

	// Find the new category
	var found bool
	for _, cat := range categories {
		if cat.Name == "Fitness & Health" {
			found = true
			break
		}
	}
	assert.True(t, found, "New category 'Fitness & Health' should have been created")

	// Verify the transaction was classified
	classifications, err := db.GetClassificationsByDateRange(ctx, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, classifications, 1)
	assert.Equal(t, "Fitness & Health", classifications[0].Category)
}

// newCategoryClassifier simulates AI suggesting a new category.
type newCategoryClassifier struct {
	suggestedCategory string
	confidence        float64
	isNew             bool
}

func (n *newCategoryClassifier) SuggestCategory(_ context.Context, _ model.Transaction, _ []string) (string, float64, bool, string, error) {
	description := ""
	if n.isNew {
		description = "Description for " + n.suggestedCategory
	}
	return n.suggestedCategory, n.confidence, n.isNew, description, nil
}

func (n *newCategoryClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, len(transactions))
	for i, txn := range transactions {
		category, confidence, isNew, description, err := n.SuggestCategory(ctx, txn, categories)
		if err != nil {
			return nil, err
		}
		suggestions[i] = service.LLMSuggestion{
			TransactionID:       txn.ID,
			Category:            category,
			Confidence:          confidence,
			IsNew:               isNew,
			CategoryDescription: description,
		}
	}
	return suggestions, nil
}

func (n *newCategoryClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (string, float64, error) {
	return "Generated description for " + categoryName, 0.95, nil
}

func (n *newCategoryClassifier) SuggestCategoryRankings(_ context.Context, _ model.Transaction, _ []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	// Return the new category suggestion as a ranking
	description := ""
	if n.isNew {
		description = "Description for " + n.suggestedCategory
	}
	return model.CategoryRankings{
		{
			Category:    n.suggestedCategory,
			Score:       n.confidence,
			IsNew:       n.isNew,
			Description: description,
		},
	}, nil
}

func (n *newCategoryClassifier) SuggestCategoryBatch(_ context.Context, _ []llm.MerchantBatchRequest, _ []model.Category) (map[string]model.CategoryRankings, error) {
	return make(map[string]model.CategoryRankings), nil
}

// newCategoryPrompter simulates user accepting a new category.
type newCategoryPrompter struct {
	acceptedCategory  string
	acceptNewCategory bool
}

func (n *newCategoryPrompter) ConfirmClassification(_ context.Context, pending model.PendingClassification) (model.Classification, error) {
	if pending.IsNewCategory && n.acceptNewCategory {
		return model.Classification{
			Transaction:  pending.Transaction,
			Category:     n.acceptedCategory,
			Status:       model.StatusClassifiedByAI,
			Confidence:   pending.Confidence,
			ClassifiedAt: time.Now(),
		}, nil
	}
	return model.Classification{}, errors.New("not accepting new category")
}

func (n *newCategoryPrompter) BatchConfirmClassifications(_ context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	classifications := make([]model.Classification, len(pending))
	for i, p := range pending {
		c, err := n.ConfirmClassification(context.Background(), p)
		if err != nil {
			return nil, err
		}
		classifications[i] = c
	}
	return classifications, nil
}

func (n *newCategoryPrompter) GetCompletionStats() service.CompletionStats {
	return service.CompletionStats{}
}
