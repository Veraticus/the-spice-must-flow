package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// Mock implementations for engine tests.
type engineMockStorage struct {
	service.Storage
	mock.Mock
}

func (m *engineMockStorage) GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
	args := m.Called(ctx, start, end)
	if classifications, ok := args.Get(0).([]model.Classification); ok {
		return classifications, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) GetCategories(ctx context.Context) ([]model.Category, error) {
	args := m.Called(ctx)
	if categories, ok := args.Get(0).([]model.Category); ok {
		return categories, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error) {
	args := m.Called(ctx)
	if rules, ok := args.Get(0).([]model.PatternRule); ok {
		return rules, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	args := m.Called(ctx)
	if patterns, ok := args.Get(0).([]model.CheckPattern); ok {
		return patterns, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) BeginTx(ctx context.Context) (service.Transaction, error) {
	args := m.Called(ctx)
	if tx, ok := args.Get(0).(service.Transaction); ok {
		return tx, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) SaveTransaction(ctx context.Context, txn model.Transaction) error {
	args := m.Called(ctx, txn)
	return args.Error(0)
}

func (m *engineMockStorage) SaveClassification(ctx context.Context, classification *model.Classification) error {
	args := m.Called(ctx, classification)
	return args.Error(0)
}

func (m *engineMockStorage) GetTransaction(ctx context.Context, id string) (*model.Transaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if txn, ok := args.Get(0).(*model.Transaction); ok {
		return txn, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) UpdateTransaction(ctx context.Context, txn model.Transaction) error {
	args := m.Called(ctx, txn)
	return args.Error(0)
}

func (m *engineMockStorage) SaveCategory(ctx context.Context, category model.Category) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *engineMockStorage) GetCategory(ctx context.Context, id string) (*model.Category, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if cat, ok := args.Get(0).(*model.Category); ok {
		return cat, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) UpdateCategory(ctx context.Context, id int, name, description string) error {
	args := m.Called(ctx, id, name, description)
	return args.Error(0)
}

func (m *engineMockStorage) DeleteCategory(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *engineMockStorage) GetDefaultCategoryID(ctx context.Context, categoryType model.CategoryType) (string, error) {
	args := m.Called(ctx, categoryType)
	return args.String(0), args.Error(1)
}

func (m *engineMockStorage) UpdatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *engineMockStorage) DeletePatternRule(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *engineMockStorage) GetPatternRule(ctx context.Context, id int) (*model.PatternRule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if rule, ok := args.Get(0).(*model.PatternRule); ok {
		return rule, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

func (m *engineMockStorage) DeleteCheckPattern(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *engineMockStorage) GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if pattern, ok := args.Get(0).(*model.CheckPattern); ok {
		return pattern, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) SaveVendor(ctx context.Context, vendor *model.Vendor) error {
	args := m.Called(ctx, vendor)
	return args.Error(0)
}

func (m *engineMockStorage) ListVendors(ctx context.Context) ([]model.Vendor, error) {
	args := m.Called(ctx)
	if vendors, ok := args.Get(0).([]model.Vendor); ok {
		return vendors, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) GetVendor(ctx context.Context, id string) (*model.Vendor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if vendor, ok := args.Get(0).(*model.Vendor); ok {
		return vendor, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) UpdateVendor(ctx context.Context, vendor model.Vendor) error {
	args := m.Called(ctx, vendor)
	return args.Error(0)
}

func (m *engineMockStorage) DeleteVendor(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *engineMockStorage) CreateCategoryRanking(ctx context.Context, cr model.CategoryRanking) error {
	args := m.Called(ctx, cr)
	return args.Error(0)
}

func (m *engineMockStorage) GetCategoryRankings(ctx context.Context, vendorName string) ([]model.CategoryRanking, error) {
	args := m.Called(ctx, vendorName)
	if v, ok := args.Get(0).([]model.CategoryRanking); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) IncreaseRanking(ctx context.Context, vendorName, categoryID string) error {
	args := m.Called(ctx, vendorName, categoryID)
	return args.Error(0)
}

func (m *engineMockStorage) Migrate(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *engineMockStorage) GetSchemaVersion(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *engineMockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *engineMockStorage) ClearAllClassifications(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *engineMockStorage) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	args := m.Called(ctx, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*model.Category); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockStorage) CreateCategoryWithType(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error) {
	args := m.Called(ctx, name, description, categoryType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*model.Category); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

type engineMockLLMClient struct {
	mock.Mock
}

func (m *engineMockLLMClient) AnalyzeTransactions(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *engineMockLLMClient) ValidateAndCorrectResponse(ctx context.Context, correctionPrompt string) (string, error) {
	args := m.Called(ctx, correctionPrompt)
	return args.String(0), args.Error(1)
}

func (m *engineMockLLMClient) AnalyzeTransactionsWithFile(ctx context.Context, prompt string, transactionData map[string]interface{}) (string, error) {
	args := m.Called(ctx, prompt, transactionData)
	return args.String(0), args.Error(1)
}

type engineMockSessionStore struct {
	mock.Mock
}

func (m *engineMockSessionStore) Create(ctx context.Context, session *Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *engineMockSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*Session); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockSessionStore) Update(ctx context.Context, session *Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

type engineMockReportStore struct {
	mock.Mock
}

func (m *engineMockReportStore) SaveReport(ctx context.Context, report *Report) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *engineMockReportStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*Report); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

type engineMockValidator struct {
	mock.Mock
}

func (m *engineMockValidator) Validate(data []byte) (*Report, error) {
	args := m.Called(data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*Report); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *engineMockValidator) ExtractError(data []byte, err error) (section string, line int, column int) {
	args := m.Called(data, err)
	return args.String(0), args.Int(1), args.Int(2)
}

type engineMockFixApplier struct {
	mock.Mock
}

func (m *engineMockFixApplier) ApplyPatternFixes(ctx context.Context, patterns []SuggestedPattern) error {
	args := m.Called(ctx, patterns)
	return args.Error(0)
}

func (m *engineMockFixApplier) ApplyCategoryFixes(ctx context.Context, fixes []Fix) error {
	args := m.Called(ctx, fixes)
	return args.Error(0)
}

func (m *engineMockFixApplier) ApplyRecategorizations(ctx context.Context, issues []Issue) error {
	args := m.Called(ctx, issues)
	return args.Error(0)
}

type engineMockReportFormatter struct {
	mock.Mock
}

func (m *engineMockReportFormatter) FormatSummary(report *Report) string {
	args := m.Called(report)
	return args.String(0)
}

func (m *engineMockReportFormatter) FormatIssue(issue Issue) string {
	args := m.Called(issue)
	return args.String(0)
}

func (m *engineMockReportFormatter) FormatInteractive(report *Report) string {
	args := m.Called(report)
	return args.String(0)
}

func TestEngine_Analyze(t *testing.T) {
	ctx := context.Background()
	startDate := time.Now().AddDate(0, -1, 0)
	endDate := time.Now()

	tests := []struct {
		setupMocks  func(*engineMockStorage, *engineMockLLMClient, *engineMockSessionStore, *engineMockReportStore, *engineMockValidator, *engineMockFixApplier)
		validate    func(*testing.T, *Report)
		name        string
		errContains string
		opts        Options
		wantErr     bool
	}{
		{
			name: "successful analysis",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
				Focus:     FocusAll,
			},
			setupMocks: func(storage *engineMockStorage, llm *engineMockLLMClient, sessions *engineMockSessionStore, reports *engineMockReportStore, validator *engineMockValidator, _ *engineMockFixApplier) {
				// Session management
				sessions.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				// Load data
				categoryID := "cat1"
				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{
					{
						Transaction: model.Transaction{ID: "1", Amount: 100.00, Name: "Test", Category: []string{categoryID}},
					},
				}, nil)
				storage.On("GetCategories", mock.Anything).Return([]model.Category{
					{ID: 1, Name: "Groceries"},
				}, nil)
				storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
				storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

				// LLM analysis
				validJSON := `{
					"coherence_score": 85,
					"issues": [],
					"insights": ["Good categorization"],
					"suggested_patterns": []
				}`
				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(validJSON, nil)

				// Validation
				report := &Report{
					CoherenceScore: 85,
					Issues:         []Issue{},
					Insights:       []string{"Good categorization"},
				}
				validator.On("Validate", []byte(validJSON)).Return(report, nil)

				// Save report
				reports.On("SaveReport", mock.Anything, mock.AnythingOfType("*analysis.Report")).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, report *Report) {
				t.Helper()
				assert.Equal(t, float64(85), report.CoherenceScore)
				assert.Empty(t, report.Issues)
			},
		},
		{
			name: "validation recovery",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
			},
			setupMocks: func(storage *engineMockStorage, llm *engineMockLLMClient, sessions *engineMockSessionStore, reports *engineMockReportStore, validator *engineMockValidator, _ *engineMockFixApplier) {
				// Session management
				sessions.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				// Load data
				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{
					{
						Transaction: model.Transaction{ID: "1", Amount: 100.00},
					},
				}, nil)
				storage.On("GetCategories", mock.Anything).Return([]model.Category{}, nil)
				storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
				storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

				// First attempt fails validation
				invalidJSON := `{"coherence_score": "invalid"}`
				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(invalidJSON, nil).Once()

				validator.On("Validate", []byte(invalidJSON)).Return(nil, errors.New("invalid JSON")).Once()
				validator.On("ExtractError", []byte(invalidJSON), mock.AnythingOfType("*errors.errorString")).Return("coherence_score", 1, 20)

				// Second attempt succeeds
				validJSON := `{"coherence_score": 75, "issues": [], "insights": ["Fixed"]}`
				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(validJSON, nil).Once()

				report := &Report{
					CoherenceScore: 75,
					Insights:       []string{"Fixed"},
				}
				validator.On("Validate", []byte(validJSON)).Return(report, nil).Once()

				reports.On("SaveReport", mock.Anything, mock.AnythingOfType("*analysis.Report")).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, report *Report) {
				t.Helper()
				assert.Equal(t, float64(75), report.CoherenceScore)
			},
		},
		{
			name: "all validation attempts fail",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
			},
			setupMocks: func(storage *engineMockStorage, llm *engineMockLLMClient, sessions *engineMockSessionStore, _ *engineMockReportStore, validator *engineMockValidator, _ *engineMockFixApplier) {
				sessions.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{}, nil)
				storage.On("GetCategories", mock.Anything).Return([]model.Category{}, nil)
				storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
				storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

				// All attempts fail
				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(`{"invalid": true}`, nil)

				validator.On("Validate", mock.Anything).Return(nil, errors.New("validation failed"))
				validator.On("ExtractError", mock.Anything, mock.Anything).Return("", 0, 0)
			},
			wantErr:     true,
			errContains: "analysis failed after 3 attempts",
		},
		{
			name: "with auto-apply fixes",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
				AutoApply: true,
			},
			setupMocks: func(storage *engineMockStorage, llm *engineMockLLMClient, sessions *engineMockSessionStore, reports *engineMockReportStore, validator *engineMockValidator, fixer *engineMockFixApplier) {
				sessions.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{}, nil)
				storage.On("GetCategories", mock.Anything).Return([]model.Category{}, nil)
				storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
				storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

				validJSON := `{
					"coherence_score": 90,
					"issues": [{
						"type": "miscategorized",
						"severity": "high",
						"description": "Wrong category",
						"fix": {
							"type": "update_category",
							"description": "Fix category"
						}
					}],
					"insights": ["Found issues"],
					"suggested_patterns": [{
						"pattern": {
							"merchant_pattern": "GROCERY*",
							"default_category": "cat1"
						}
					}]
				}`

				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(validJSON, nil)

				report := &Report{
					CoherenceScore: 90,
					Issues: []Issue{
						{
							Type:        IssueTypeMiscategorized,
							Severity:    SeverityHigh,
							Description: "Wrong category",
							Fix: &Fix{
								Type:        "update_category",
								Description: "Fix category",
							},
						},
					},
					SuggestedPatterns: []SuggestedPattern{
						{
							Pattern: model.PatternRule{
								MerchantPattern: "GROCERY*",
								DefaultCategory: "cat1",
							},
						},
					},
				}
				validator.On("Validate", []byte(validJSON)).Return(report, nil)

				reports.On("SaveReport", mock.Anything, mock.AnythingOfType("*analysis.Report")).Return(nil)

				// Fix application
				fixer.On("ApplyPatternFixes", mock.Anything, mock.AnythingOfType("[]analysis.SuggestedPattern")).Return(nil)
				fixer.On("ApplyCategoryFixes", mock.Anything, mock.AnythingOfType("[]analysis.Fix")).Return(nil)
				fixer.On("ApplyRecategorizations", mock.Anything, mock.AnythingOfType("[]analysis.Issue")).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "transaction loading failure",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
			},
			setupMocks: func(storage *engineMockStorage, _ *engineMockLLMClient, sessions *engineMockSessionStore, _ *engineMockReportStore, _ *engineMockValidator, _ *engineMockFixApplier) {
				sessions.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{}, errors.New("database error"))
			},
			wantErr:     true,
			errContains: "failed to load transactions",
		},
		{
			name: "continue existing session",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
				SessionID: "existing-session",
			},
			setupMocks: func(storage *engineMockStorage, llm *engineMockLLMClient, sessions *engineMockSessionStore, reports *engineMockReportStore, validator *engineMockValidator, _ *engineMockFixApplier) {
				// Return existing session
				existingSession := &Session{
					ID:     "existing-session",
					Status: StatusInProgress,
				}
				sessions.On("Get", mock.Anything, "existing-session").Return(existingSession, nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{}, nil)
				storage.On("GetCategories", mock.Anything).Return([]model.Category{}, nil)
				storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
				storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

				validJSON := `{"coherence_score": 80, "issues": [], "insights": ["Good"]}`
				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(validJSON, nil)

				report := &Report{CoherenceScore: 80}
				validator.On("Validate", []byte(validJSON)).Return(report, nil)
				reports.On("SaveReport", mock.Anything, mock.AnythingOfType("*analysis.Report")).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "with progress callback",
			opts: Options{
				StartDate: startDate,
				EndDate:   endDate,
				ProgressFunc: func(_ string, _ int) {
					// Progress tracking
				},
			},
			setupMocks: func(storage *engineMockStorage, llm *engineMockLLMClient, sessions *engineMockSessionStore, reports *engineMockReportStore, validator *engineMockValidator, _ *engineMockFixApplier) {
				sessions.On("Create", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)
				sessions.On("Update", mock.Anything, mock.AnythingOfType("*analysis.Session")).Return(nil)

				storage.On("GetClassificationsByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return([]model.Classification{}, nil)
				storage.On("GetCategories", mock.Anything).Return([]model.Category{}, nil)
				storage.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
				storage.On("GetActiveCheckPatterns", mock.Anything).Return([]model.CheckPattern{}, nil)

				validJSON := `{"coherence_score": 95, "issues": [], "insights": ["Excellent"]}`
				llm.On("AnalyzeTransactionsWithFile", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return(validJSON, nil)

				report := &Report{CoherenceScore: 95}
				validator.On("Validate", []byte(validJSON)).Return(report, nil)
				reports.On("SaveReport", mock.Anything, mock.AnythingOfType("*analysis.Report")).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, report *Report) {
				t.Helper()
				assert.Equal(t, float64(95), report.CoherenceScore)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			storage := new(engineMockStorage)
			llmClient := new(engineMockLLMClient)
			sessionStore := new(engineMockSessionStore)
			reportStore := new(engineMockReportStore)
			validator := new(engineMockValidator)
			fixApplier := new(engineMockFixApplier)

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(storage, llmClient, sessionStore, reportStore, validator, fixApplier)
			}

			// Create prompt builder
			promptBuilder, err := NewTemplatePromptBuilder()
			assert.NoError(t, err)

			// Create engine
			deps := Deps{
				Storage:       storage,
				LLMClient:     llmClient,
				SessionStore:  sessionStore,
				ReportStore:   reportStore,
				Validator:     validator,
				FixApplier:    fixApplier,
				PromptBuilder: promptBuilder,
				Formatter:     new(engineMockReportFormatter),
			}

			engine, err := NewEngine(deps)
			assert.NoError(t, err)

			// Run analysis
			report, err := engine.Analyze(ctx, tt.opts)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, report)

				// Additional validation
				if tt.validate != nil {
					tt.validate(t, report)
				}
			}

			// Verify mock expectations
			storage.AssertExpectations(t)
			llmClient.AssertExpectations(t)
			sessionStore.AssertExpectations(t)
			reportStore.AssertExpectations(t)
			validator.AssertExpectations(t)
			fixApplier.AssertExpectations(t)
		})
	}
}

func TestEngine_analyzeVendors(t *testing.T) {
	ctx := context.Background()
	engine := &Engine{}

	transactions := []model.Transaction{
		{MerchantName: "Grocery Store", Category: []string{"cat1"}},
		{MerchantName: "Grocery Store", Category: []string{"cat1"}},
		{MerchantName: "Grocery Store", Category: []string{"cat2"}},
		{MerchantName: "Gas Station", Category: []string{"cat2"}},
		{MerchantName: "Gas Station", Category: []string{"cat2"}},
		{MerchantName: "", Category: []string{"cat1"}},     // Empty vendor
		{MerchantName: "Restaurant", Category: []string{}}, // No category
	}

	vendors, err := engine.analyzeVendors(ctx, transactions)
	assert.NoError(t, err)

	// Should have 2 vendors (Grocery Store and Gas Station)
	assert.Len(t, vendors, 2)

	// Check vendor details
	vendorMap := make(map[string]RecentVendor)
	for _, v := range vendors {
		vendorMap[v.Name] = v
	}

	assert.Equal(t, "cat1", vendorMap["Grocery Store"].Category)
	assert.Equal(t, 2, vendorMap["Grocery Store"].Occurrences)

	assert.Equal(t, "cat2", vendorMap["Gas Station"].Category)
	assert.Equal(t, 2, vendorMap["Gas Station"].Occurrences)
}
