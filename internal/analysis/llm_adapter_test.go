package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLLMClient implements llm.Client for testing.
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) Classify(ctx context.Context, prompt string) (llm.ClassificationResponse, error) {
	args := m.Called(ctx, prompt)
	if v, ok := args.Get(0).(llm.ClassificationResponse); ok {
		return v, args.Error(1)
	}
	return llm.ClassificationResponse{}, args.Error(1)
}

func (m *MockLLMClient) ClassifyWithRankings(ctx context.Context, prompt string) (llm.RankingResponse, error) {
	args := m.Called(ctx, prompt)
	if v, ok := args.Get(0).(llm.RankingResponse); ok {
		return v, args.Error(1)
	}
	return llm.RankingResponse{}, args.Error(1)
}

func (m *MockLLMClient) ClassifyMerchantBatch(ctx context.Context, prompt string) (llm.MerchantBatchResponse, error) {
	args := m.Called(ctx, prompt)
	if v, ok := args.Get(0).(llm.MerchantBatchResponse); ok {
		return v, args.Error(1)
	}
	return llm.MerchantBatchResponse{}, args.Error(1)
}

func (m *MockLLMClient) GenerateDescription(ctx context.Context, prompt string) (llm.DescriptionResponse, error) {
	args := m.Called(ctx, prompt)
	if v, ok := args.Get(0).(llm.DescriptionResponse); ok {
		return v, args.Error(1)
	}
	return llm.DescriptionResponse{}, args.Error(1)
}

func (m *MockLLMClient) Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error) {
	args := m.Called(ctx, prompt, systemPrompt)
	return args.String(0), args.Error(1)
}

// MockReportValidator implements ReportValidator for testing.
type MockReportValidator struct {
	mock.Mock
}

func (m *MockReportValidator) Validate(data []byte) (*Report, error) {
	args := m.Called(data)
	if v, ok := args.Get(0).(*Report); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockReportValidator) ExtractError(data []byte, err error) (string, int, int) {
	args := m.Called(data, err)
	return args.String(0), args.Int(1), args.Int(2)
}

func TestNewLLMAnalysisAdapter(t *testing.T) {
	mockClient := &MockLLMClient{}
	adapter := NewLLMAnalysisAdapter(mockClient)

	assert.NotNil(t, adapter)
	assert.Equal(t, mockClient, adapter.client)
	assert.Equal(t, 3, adapter.retryOptions.MaxAttempts)
	assert.Equal(t, 1*time.Second, adapter.retryOptions.InitialDelay)
	assert.Equal(t, 30*time.Second, adapter.retryOptions.MaxDelay)
	assert.Equal(t, 2.0, adapter.retryOptions.Multiplier)
}

func TestNewLLMAnalysisAdapterWithRetry(t *testing.T) {
	mockClient := &MockLLMClient{}
	customRetry := service.RetryOptions{
		MaxAttempts:  5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   1.5,
	}

	adapter := NewLLMAnalysisAdapterWithRetry(mockClient, customRetry)

	assert.NotNil(t, adapter)
	assert.Equal(t, mockClient, adapter.client)
	assert.Equal(t, customRetry, adapter.retryOptions)
}

func TestAnalyzeTransactions(t *testing.T) {
	ctx := context.Background()

	validReport := &Report{
		CoherenceScore: 85,
		Insights:       []string{"Test summary"},
		Issues: []Issue{
			{
				Severity:    SeverityHigh,
				Type:        IssueTypeMiscategorized,
				Description: "Test issue",
			},
		},
		CategorySummary: map[string]CategoryStat{
			"Test": {TransactionCount: 100},
		},
	}

	validJSON, _ := json.Marshal(validReport)

	tests := []struct {
		setupMock   func(*MockLLMClient)
		name        string
		wantJSON    string
		errContains string
		wantErr     bool
	}{
		{
			name: "successful analysis",
			setupMock: func(m *MockLLMClient) {
				m.On("Analyze", ctx, "test prompt", mock.AnythingOfType("string")).Return(
					string(validJSON), nil,
				).Once()
			},
			wantJSON: string(validJSON),
			wantErr:  false,
		},
		{
			name: "retry on temporary error",
			setupMock: func(m *MockLLMClient) {
				// First call fails with temporary error
				m.On("Analyze", ctx, "test prompt", mock.AnythingOfType("string")).Return(
					"", errors.New("connection timeout"),
				).Once()

				// Second call succeeds
				m.On("Analyze", ctx, "test prompt", mock.AnythingOfType("string")).Return(
					string(validJSON), nil,
				).Once()
			},
			wantJSON: string(validJSON),
			wantErr:  false,
		},
		{
			name: "non-retryable error",
			setupMock: func(m *MockLLMClient) {
				m.On("Analyze", ctx, "test prompt", mock.AnythingOfType("string")).Return(
					"", errors.New("invalid API key"),
				).Once()
			},
			wantJSON:    "",
			wantErr:     true,
			errContains: "invalid API key",
		},
		{
			name: "max retries exceeded",
			setupMock: func(m *MockLLMClient) {
				m.On("Analyze", ctx, "test prompt", mock.AnythingOfType("string")).Return(
					"", errors.New("rate limit exceeded"),
				).Times(3)
			},
			wantJSON:    "",
			wantErr:     true,
			errContains: "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockLLMClient{}
			tt.setupMock(mockClient)

			adapter := NewLLMAnalysisAdapterWithRetry(mockClient, service.RetryOptions{
				MaxAttempts:  3,
				InitialDelay: 10 * time.Millisecond, // Short delay for tests
				MaxDelay:     50 * time.Millisecond,
				Multiplier:   2.0,
			})

			resultJSON, err := adapter.AnalyzeTransactions(ctx, "test prompt")

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantJSON, resultJSON)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestAnalyzeWithFallback(t *testing.T) {
	ctx := context.Background()

	validReport := &Report{
		CoherenceScore: 90,
		Insights:       []string{"Valid report"},
		Issues:         []Issue{},
		CategorySummary: map[string]CategoryStat{
			"Test": {TransactionCount: 50},
		},
	}

	validJSON, _ := json.Marshal(validReport)

	tests := []struct {
		setupMocks  func(*MockLLMClient, *MockReportValidator)
		wantReport  *Report
		name        string
		errContains string
		promptData  PromptData
		wantErr     bool
	}{
		{
			name: "successful on first attempt",
			setupMocks: func(mockLLM *MockLLMClient, validator *MockReportValidator) {
				mockLLM.On("Analyze", ctx, mock.Anything, mock.Anything).Return(
					string(validJSON),
					nil,
				).Once()

				validator.On("Validate", validJSON).Return(validReport, nil).Once()
			},
			promptData: PromptData{
				Transactions: []model.Transaction{
					{ID: "test", Amount: 10.0},
				},
				Categories: []model.Category{
					{Name: "Test"},
				},
				DateRange:  DateRange{Start: time.Now(), End: time.Now()},
				TotalCount: 1,
				SampleSize: 1,
				FileBasedData: &FileBasedPromptData{
					FilePath:           "/tmp/test-transactions.json",
					TransactionCount:   1,
					UseFileBasedPrompt: true,
				},
			},
			wantReport: validReport,
			wantErr:    false,
		},
		{
			name: "successful after correction",
			setupMocks: func(mockLLM *MockLLMClient, validator *MockReportValidator) {
				// First attempt returns invalid JSON
				invalidJSON := `{"coherence_score": 85, "summary": "test"`
				mockLLM.On("Analyze", ctx, mock.Anything, mock.Anything).Return(
					invalidJSON,
					nil,
				).Once()

				// Validation fails
				validator.On("Validate", []byte(invalidJSON)).Return(
					(*Report)(nil),
					errors.New("unexpected EOF"),
				).Once()

				// Extract bad section
				validator.On("ExtractError", []byte(invalidJSON), mock.Anything).Return(
					`"summary": "test"`,
					1,
					20,
				).Once()

				// Correction attempt succeeds
				mockLLM.On("Analyze", ctx, mock.Anything, mock.Anything).Return(
					string(validJSON),
					nil,
				).Once()

				// Validation succeeds
				validator.On("Validate", validJSON).Return(validReport, nil).Once()
			},
			promptData: PromptData{
				Transactions: []model.Transaction{
					{ID: "test", Amount: 10.0},
				},
				Categories: []model.Category{
					{Name: "Test"},
				},
				DateRange:  DateRange{Start: time.Now(), End: time.Now()},
				TotalCount: 1,
				SampleSize: 1,
				FileBasedData: &FileBasedPromptData{
					FilePath:           "/tmp/test-transactions.json",
					TransactionCount:   1,
					UseFileBasedPrompt: true,
				},
			},
			wantReport: validReport,
			wantErr:    false,
		},
		{
			name: "correction also fails",
			setupMocks: func(mockLLM *MockLLMClient, _ *MockReportValidator) {
				// First attempt fails
				mockLLM.On("Analyze", ctx, mock.Anything, mock.Anything).Return(
					"",
					errors.New("network error"),
				).Twice() // Once for initial, once for correction
			},
			promptData: PromptData{
				Transactions: []model.Transaction{
					{ID: "test", Amount: 10.0},
				},
				Categories: []model.Category{
					{Name: "Test"},
				},
				DateRange:  DateRange{Start: time.Now(), End: time.Now()},
				TotalCount: 1,
				SampleSize: 1,
				FileBasedData: &FileBasedPromptData{
					FilePath:           "/tmp/test-transactions.json",
					TransactionCount:   1,
					UseFileBasedPrompt: true,
				},
			},
			wantReport:  nil,
			wantErr:     true,
			errContains: "correction attempt failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &MockLLMClient{}
			mockValidator := &MockReportValidator{}
			tt.setupMocks(mockLLM, mockValidator)

			adapter := NewLLMAnalysisAdapterWithRetry(mockLLM, service.RetryOptions{
				MaxAttempts:  1, // No retries for cleaner test behavior
				InitialDelay: 10 * time.Millisecond,
				MaxDelay:     50 * time.Millisecond,
				Multiplier:   2.0,
			})

			promptBuilder, err := NewTemplatePromptBuilder()
			assert.NoError(t, err)

			report, err := adapter.AnalyzeWithFallback(ctx, promptBuilder, tt.promptData, mockValidator)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantReport, report)
			}

			mockLLM.AssertExpectations(t)
			mockValidator.AssertExpectations(t)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err  error
		name string
		want bool
	}{
		{
			name: "rate limit error",
			err:  common.ErrRateLimit,
			want: true,
		},
		{
			name: "timeout error",
			err:  errors.New("connection timeout"),
			want: true,
		},
		{
			name: "connection error",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "429 error",
			err:  errors.New("HTTP 429 Too Many Requests"),
			want: true,
		},
		{
			name: "503 error",
			err:  errors.New("HTTP 503 Service Unavailable"),
			want: true,
		},
		{
			name: "504 error",
			err:  errors.New("HTTP 504 Gateway Timeout"),
			want: true,
		},
		{
			name: "non-retryable error",
			err:  errors.New("invalid API key"),
			want: false,
		},
		{
			name: "parse error",
			err:  errors.New("failed to parse response"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}
