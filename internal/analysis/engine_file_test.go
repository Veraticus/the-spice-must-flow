package analysis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

func TestEngine_FileBasedAnalysisDetection(t *testing.T) {
	tests := []struct {
		name               string
		transactionCount   int
		expectFileBased    bool
		expectedSampleSize int
	}{
		{
			name:               "small dataset uses file-based",
			transactionCount:   100,
			expectFileBased:    true,
			expectedSampleSize: 0, // Always 0 since we never include transactions in prompt
		},
		{
			name:               "medium dataset uses file-based",
			transactionCount:   5000,
			expectFileBased:    true,
			expectedSampleSize: 0,
		},
		{
			name:               "large dataset uses file-based",
			transactionCount:   10000,
			expectFileBased:    true,
			expectedSampleSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test transactions
			transactions := make([]model.Transaction, tt.transactionCount)
			for i := 0; i < tt.transactionCount; i++ {
				transactions[i] = model.Transaction{
					ID:     fmt.Sprintf("txn-%d", i),
					Name:   fmt.Sprintf("Test Transaction %d", i),
					Amount: float64(i + 1),
					Date:   time.Now(),
					Type:   "DEBIT",
				}
			}

			// Create mock dependencies
			mockDeps := createFileTestDependencies()

			// Capture the prompt data used in analysis
			var capturedPromptData PromptData
			mockDeps.PromptBuilder = &fileTestPromptBuilder{
				buildFunc: func(data PromptData) (string, error) {
					capturedPromptData = data
					return "test prompt", nil
				},
			}

			// Mock the storage to return our test transactions
			mockDeps.Storage = &fileTestStorage{
				getClassificationsByDateRangeFunc: func(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
					// Return classifications that reference our transactions
					classifications := make([]model.Classification, len(transactions))
					for i, txn := range transactions {
						classifications[i] = model.Classification{
							Transaction: txn,
						}
					}
					return classifications, nil
				},
				getCategoriesFunc: func(ctx context.Context) ([]model.Category, error) {
					return []model.Category{{Name: "Test", Type: "expense"}}, nil
				},
				getActivePatternRulesFunc: func(ctx context.Context) ([]model.PatternRule, error) {
					return []model.PatternRule{}, nil
				},
				getActiveCheckPatternsFunc: func(ctx context.Context) ([]model.CheckPattern, error) {
					return []model.CheckPattern{}, nil
				},
			}

			// Create engine
			engine, err := NewEngine(mockDeps)
			require.NoError(t, err)

			// Create a mock LLM client that tracks calls
			mockLLMClient := &fileTestLLMClient{
				analyzeTransactionsFunc: func(ctx context.Context, prompt string) (string, error) {
					return `{"coherenceScore": 85}`, nil
				},
				analyzeTransactionsWithFileFunc: func(ctx context.Context, prompt string, data map[string]interface{}) (string, error) {
					// Verify we got the right number of transactions
					if txns, ok := data["transactions"].([]map[string]interface{}); ok {
						assert.Len(t, txns, tt.transactionCount)
					}
					return `{"coherenceScore": 85}`, nil
				},
			}
			mockDeps.LLMClient = mockLLMClient

			// Run analysis
			opts := Options{
				StartDate: time.Now().AddDate(0, -1, 0),
				EndDate:   time.Now(),
			}

			_, err = engine.Analyze(context.Background(), opts)
			require.NoError(t, err)

			// Verify the correct approach was used
			if tt.expectFileBased {
				assert.NotNil(t, capturedPromptData.FileBasedData)
				assert.True(t, capturedPromptData.FileBasedData.UseFileBasedPrompt)
				assert.Equal(t, tt.transactionCount, capturedPromptData.FileBasedData.TransactionCount)
				assert.Empty(t, capturedPromptData.Transactions)
			} else {
				assert.Nil(t, capturedPromptData.FileBasedData)
				assert.Len(t, capturedPromptData.Transactions, tt.expectedSampleSize)
			}

			// Verify sample size is set correctly
			assert.Equal(t, tt.expectedSampleSize, capturedPromptData.SampleSize)
		})
	}
}

func TestEngine_TransactionCategoriesInFileExport(t *testing.T) {
	// Create prompt builder
	promptBuilder, err := NewTemplatePromptBuilder()
	require.NoError(t, err)

	// Create mocks
	storage := new(engineMockStorage)
	llmClient := new(engineMockLLMClient)
	sessionStore := new(engineMockSessionStore)
	reportStore := new(engineMockReportStore)
	validator := new(engineMockValidator)
	fixApplier := new(engineMockFixApplier)
	formatter := new(engineMockReportFormatter)

	// Setup storage mock to return classifications with categories
	storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{
		{
			Transaction: model.Transaction{
				ID:           "txn-1",
				Name:         "Test Transaction 1",
				MerchantName: "Test Merchant",
				Amount:       100.00,
				Date:         time.Now(),
				Category:     []string{"Original Import Category"}, // This should be ignored
			},
			Category:   "Groceries", // This is the actual classification category
			Status:     model.StatusClassifiedByAI,
			Confidence: 0.95,
		},
		{
			Transaction: model.Transaction{
				ID:           "txn-2",
				Name:         "Test Transaction 2",
				MerchantName: "Another Merchant",
				Amount:       50.00,
				Date:         time.Now(),
				Category:     []string{}, // No original category
			},
			Category:   "Entertainment", // Classification category
			Status:     model.StatusUserModified,
			Confidence: 1.0,
		},
	}, nil)

	storage.On("GetCategories", mock.Anything).Return([]model.Category{
		{Name: "Groceries", Type: model.CategoryTypeExpense},
		{Name: "Entertainment", Type: model.CategoryTypeExpense},
	}, nil)
	storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
	storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

	// Setup session store
	sessionStore.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
	sessionStore.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

	// Capture the data sent to LLM
	var capturedTransactionData map[string]interface{}
	llmClient.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).
		Run(func(args mock.Arguments) {
			capturedTransactionData = args.Get(2).(map[string]interface{})
		}).
		Return(`{"coherence_score": 90, "issues": []}`, nil)

	// Setup validator
	report := &Report{CoherenceScore: 90, Issues: []Issue{}}
	validator.On("Validate", mock.AnythingOfType("[]uint8")).Return(report, nil)

	// Setup report store
	reportStore.On("SaveReport", mock.Anything, mock.AnythingOfType("*analysis.Report")).Return(nil)

	// Create engine
	deps := Deps{
		Storage:       storage,
		LLMClient:     llmClient,
		SessionStore:  sessionStore,
		ReportStore:   reportStore,
		Validator:     validator,
		FixApplier:    fixApplier,
		PromptBuilder: promptBuilder,
		Formatter:     formatter,
	}

	engine, err := NewEngine(deps)
	require.NoError(t, err)

	// Run analysis
	opts := Options{
		StartDate: time.Now().AddDate(0, -1, 0),
		EndDate:   time.Now(),
	}

	_, err = engine.Analyze(context.Background(), opts)
	require.NoError(t, err)

	// Verify transaction data was captured
	require.NotNil(t, capturedTransactionData)

	// Extract transactions from captured data
	transactionsRaw, ok := capturedTransactionData["transactions"]
	require.True(t, ok, "transactions key should exist in data")

	transactions, ok := transactionsRaw.([]map[string]interface{})
	require.True(t, ok, "transactions should be a slice of maps")
	require.Len(t, transactions, 2)

	// Verify transactions have correct categories (order may vary)
	txnMap := make(map[string]string)
	for _, txn := range transactions {
		id, ok := txn["ID"].(string)
		require.True(t, ok, "ID should be a string")
		category, ok := txn["Category"].(string)
		require.True(t, ok, "Category should be a string")
		txnMap[id] = category
	}

	assert.Equal(t, "Groceries", txnMap["txn-1"])
	assert.Equal(t, "Entertainment", txnMap["txn-2"])

	// Verify all mocks expectations were met
	storage.AssertExpectations(t)
	llmClient.AssertExpectations(t)
	sessionStore.AssertExpectations(t)
	reportStore.AssertExpectations(t)
	validator.AssertExpectations(t)
}

// Mock implementations for file-based testing

type fileTestPromptBuilder struct {
	buildFunc func(PromptData) (string, error)
}

func (m *fileTestPromptBuilder) BuildAnalysisPrompt(data PromptData) (string, error) {
	if m.buildFunc != nil {
		return m.buildFunc(data)
	}
	return "test prompt", nil
}

func (m *fileTestPromptBuilder) BuildCorrectionPrompt(data CorrectionPromptData) (string, error) {
	return "correction prompt", nil
}

type fileTestLLMClient struct {
	analyzeTransactionsFunc         func(context.Context, string) (string, error)
	analyzeTransactionsWithFileFunc func(context.Context, string, map[string]interface{}) (string, error)
}

func (m *fileTestLLMClient) AnalyzeTransactions(ctx context.Context, prompt string) (string, error) {
	if m.analyzeTransactionsFunc != nil {
		return m.analyzeTransactionsFunc(ctx, prompt)
	}
	return `{"coherenceScore": 85}`, nil
}

func (m *fileTestLLMClient) AnalyzeTransactionsWithFile(ctx context.Context, prompt string, data map[string]interface{}) (string, error) {
	if m.analyzeTransactionsWithFileFunc != nil {
		return m.analyzeTransactionsWithFileFunc(ctx, prompt, data)
	}
	return `{"coherenceScore": 85}`, nil
}

func (m *fileTestLLMClient) ValidateAndCorrectResponse(ctx context.Context, prompt string) (string, error) {
	return `{"coherenceScore": 85}`, nil
}

// Helper to create mock dependencies for file tests.
func createFileTestDependencies() Deps {
	return Deps{
		Storage:       &fileTestStorage{},
		LLMClient:     &fileTestLLMClient{},
		PromptBuilder: &fileTestPromptBuilder{},
		Validator:     &fileTestValidator{},
		SessionStore:  &fileTestSessionStore{},
		ReportStore:   &fileTestReportStore{},
		FixApplier:    &fileTestFixer{},
		Formatter:     &fileTestFormatter{},
	}
}

type fileTestStorage struct {
	getClassificationsByDateRangeFunc func(context.Context, time.Time, time.Time) ([]model.Classification, error)
	getCategoriesFunc                 func(context.Context) ([]model.Category, error)
	getActivePatternRulesFunc         func(context.Context) ([]model.PatternRule, error)
	getActiveCheckPatternsFunc        func(context.Context) ([]model.CheckPattern, error)
}

func (m *fileTestStorage) GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
	if m.getClassificationsByDateRangeFunc != nil {
		return m.getClassificationsByDateRangeFunc(ctx, start, end)
	}
	return []model.Classification{}, nil
}

func (m *fileTestStorage) GetCategories(ctx context.Context) ([]model.Category, error) {
	if m.getCategoriesFunc != nil {
		return m.getCategoriesFunc(ctx)
	}
	return []model.Category{}, nil
}

func (m *fileTestStorage) GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error) {
	if m.getActivePatternRulesFunc != nil {
		return m.getActivePatternRulesFunc(ctx)
	}
	return []model.PatternRule{}, nil
}

func (m *fileTestStorage) GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	if m.getActiveCheckPatternsFunc != nil {
		return m.getActiveCheckPatternsFunc(ctx)
	}
	return []model.CheckPattern{}, nil
}

// Stub implementations for the rest of the Storage interface.
func (m *fileTestStorage) SaveTransactions(ctx context.Context, transactions []model.Transaction) error {
	return nil
}
func (m *fileTestStorage) GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error) {
	return nil, nil
}
func (m *fileTestStorage) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	return nil, nil
}
func (m *fileTestStorage) GetTransactionsByCategory(ctx context.Context, categoryName string) ([]model.Transaction, error) {
	return nil, nil
}
func (m *fileTestStorage) GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error) {
	return nil, nil
}
func (m *fileTestStorage) UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error {
	return nil
}
func (m *fileTestStorage) UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	return nil
}
func (m *fileTestStorage) GetTransactionCount(ctx context.Context) (int, error) { return 0, nil }
func (m *fileTestStorage) GetTransactionCountByCategory(ctx context.Context, categoryName string) (int, error) {
	return 0, nil
}
func (m *fileTestStorage) GetEarliestTransactionDate(ctx context.Context) (time.Time, error) {
	return time.Now(), nil
}
func (m *fileTestStorage) GetLatestTransactionDate(ctx context.Context) (time.Time, error) {
	return time.Now(), nil
}
func (m *fileTestStorage) GetCategorySummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	return nil, nil
}
func (m *fileTestStorage) GetMerchantSummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	return nil, nil
}
func (m *fileTestStorage) GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	return nil, nil
}
func (m *fileTestStorage) SaveVendor(ctx context.Context, vendor *model.Vendor) error  { return nil }
func (m *fileTestStorage) DeleteVendor(ctx context.Context, merchantName string) error { return nil }
func (m *fileTestStorage) GetAllVendors(ctx context.Context) ([]model.Vendor, error)   { return nil, nil }
func (m *fileTestStorage) GetVendorsByCategory(ctx context.Context, categoryName string) ([]model.Vendor, error) {
	return nil, nil
}
func (m *fileTestStorage) GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error) {
	return nil, nil
}
func (m *fileTestStorage) GetVendorsBySource(ctx context.Context, source model.VendorSource) ([]model.Vendor, error) {
	return nil, nil
}
func (m *fileTestStorage) DeleteVendorsBySource(ctx context.Context, source model.VendorSource) error {
	return nil
}
func (m *fileTestStorage) UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error {
	return nil
}
func (m *fileTestStorage) UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	return nil
}
func (m *fileTestStorage) FindVendorMatch(ctx context.Context, merchantName string) (*model.Vendor, error) {
	return nil, nil
}
func (m *fileTestStorage) SaveClassification(ctx context.Context, classification *model.Classification) error {
	return nil
}
func (m *fileTestStorage) GetClassificationsByConfidence(ctx context.Context, maxConfidence float64, excludeUserModified bool) ([]model.Classification, error) {
	return nil, nil
}
func (m *fileTestStorage) ClearAllClassifications(ctx context.Context) error { return nil }
func (m *fileTestStorage) GetCategoryByName(ctx context.Context, name string) (*model.Category, error) {
	return nil, nil
}
func (m *fileTestStorage) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	return nil, nil
}
func (m *fileTestStorage) CreateCategoryWithType(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error) {
	return nil, nil
}
func (m *fileTestStorage) UpdateCategory(ctx context.Context, id int, name, description string) error {
	return nil
}
func (m *fileTestStorage) DeleteCategory(ctx context.Context, id int) error { return nil }
func (m *fileTestStorage) CreateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	return nil
}
func (m *fileTestStorage) GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error) {
	return nil, nil
}
func (m *fileTestStorage) GetMatchingCheckPatterns(ctx context.Context, txn model.Transaction) ([]model.CheckPattern, error) {
	return nil, nil
}
func (m *fileTestStorage) UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	return nil
}
func (m *fileTestStorage) DeleteCheckPattern(ctx context.Context, id int64) error { return nil }
func (m *fileTestStorage) IncrementCheckPatternUseCount(ctx context.Context, id int64) error {
	return nil
}
func (m *fileTestStorage) CreatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	return nil
}
func (m *fileTestStorage) GetPatternRule(ctx context.Context, id int) (*model.PatternRule, error) {
	return nil, nil
}
func (m *fileTestStorage) UpdatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	return nil
}
func (m *fileTestStorage) DeletePatternRule(ctx context.Context, id int) error            { return nil }
func (m *fileTestStorage) IncrementPatternRuleUseCount(ctx context.Context, id int) error { return nil }
func (m *fileTestStorage) GetPatternRulesByCategory(ctx context.Context, category string) ([]model.PatternRule, error) {
	return nil, nil
}
func (m *fileTestStorage) Migrate(ctx context.Context) error                        { return nil }
func (m *fileTestStorage) BeginTx(ctx context.Context) (service.Transaction, error) { return nil, nil }
func (m *fileTestStorage) Close() error                                             { return nil }

type fileTestValidator struct{}

func (m *fileTestValidator) Validate(data []byte) (*Report, error) {
	return &Report{CoherenceScore: 85}, nil
}

func (m *fileTestValidator) ExtractError(data []byte, err error) (string, int, int) {
	return "", 0, 0
}

type fileTestSessionStore struct{}

func (m *fileTestSessionStore) Create(ctx context.Context, session *Session) error {
	return nil
}

func (m *fileTestSessionStore) Get(ctx context.Context, id string) (*Session, error) {
	return &Session{ID: id, Status: StatusPending}, nil
}

func (m *fileTestSessionStore) Update(ctx context.Context, session *Session) error {
	return nil
}

type fileTestReportStore struct{}

func (m *fileTestReportStore) SaveReport(ctx context.Context, report *Report) error {
	return nil
}

func (m *fileTestReportStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	return nil, nil
}

type fileTestFixer struct{}

func (m *fileTestFixer) ApplyPatternFixes(ctx context.Context, patterns []SuggestedPattern) error {
	return nil
}

func (m *fileTestFixer) ApplyCategoryFixes(ctx context.Context, fixes []Fix) error {
	return nil
}

func (m *fileTestFixer) ApplyRecategorizations(ctx context.Context, issues []Issue) error {
	return nil
}

type fileTestFormatter struct{}

func (m *fileTestFormatter) FormatSummary(report *Report) string {
	return "test summary"
}

func (m *fileTestFormatter) FormatIssue(issue Issue) string {
	return "test issue"
}

func (m *fileTestFormatter) FormatInteractive(report *Report) string {
	return "test interactive"
}
