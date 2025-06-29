package analysis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// Mock implementations for testing.
type mockStorage struct {
	service.Storage
	mock.Mock
}

type mockLLMClient struct {
	mock.Mock
}

func (m *mockLLMClient) AnalyzeTransactions(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *mockLLMClient) ValidateAndCorrectResponse(ctx context.Context, correctionPrompt string) (string, error) {
	args := m.Called(ctx, correctionPrompt)
	return args.String(0), args.Error(1)
}

type mockSessionStore struct {
	mock.Mock
}

func (m *mockSessionStore) Create(ctx context.Context, session *Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *mockSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if session, ok := args.Get(0).(*Session); ok {
		return session, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSessionStore) Update(ctx context.Context, session *Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

type mockReportStore struct {
	mock.Mock
}

func (m *mockReportStore) SaveReport(ctx context.Context, report *Report) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *mockReportStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if report, ok := args.Get(0).(*Report); ok {
		return report, args.Error(1)
	}
	return nil, args.Error(1)
}

type mockReportValidator struct {
	mock.Mock
}

func (m *mockReportValidator) Validate(data []byte) (*Report, error) {
	args := m.Called(data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if report, ok := args.Get(0).(*Report); ok {
		return report, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockReportValidator) ExtractError(data []byte, err error) (string, int, int) {
	args := m.Called(data, err)
	return args.String(0), args.Int(1), args.Int(2)
}

type mockFixApplier struct {
	mock.Mock
}

func (m *mockFixApplier) ApplyPatternFixes(ctx context.Context, patterns []SuggestedPattern) error {
	args := m.Called(ctx, patterns)
	return args.Error(0)
}

func (m *mockFixApplier) ApplyCategoryFixes(ctx context.Context, fixes []Fix) error {
	args := m.Called(ctx, fixes)
	return args.Error(0)
}

func (m *mockFixApplier) ApplyRecategorizations(ctx context.Context, issues []Issue) error {
	args := m.Called(ctx, issues)
	return args.Error(0)
}

type mockPromptBuilder struct {
	mock.Mock
}

func (m *mockPromptBuilder) BuildAnalysisPrompt(data PromptData) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}

func (m *mockPromptBuilder) BuildCorrectionPrompt(data CorrectionPromptData) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}

type mockReportFormatter struct {
	mock.Mock
}

func (m *mockReportFormatter) FormatSummary(report *Report) string {
	args := m.Called(report)
	return args.String(0)
}

func (m *mockReportFormatter) FormatIssue(issue Issue) string {
	args := m.Called(issue)
	return args.String(0)
}

func (m *mockReportFormatter) FormatInteractive(report *Report) string {
	args := m.Called(report)
	return args.String(0)
}

func TestDeps_Validate(t *testing.T) {
	// Create complete set of mocks
	validDeps := Deps{
		Storage:       &mockStorage{},
		LLMClient:     &mockLLMClient{},
		SessionStore:  &mockSessionStore{},
		ReportStore:   &mockReportStore{},
		Validator:     &mockReportValidator{},
		FixApplier:    &mockFixApplier{},
		PromptBuilder: &mockPromptBuilder{},
		Formatter:     &mockReportFormatter{},
	}

	tests := []struct {
		modifyFn func(*Deps)
		name     string
		errMsg   string
		wantErr  bool
	}{
		{
			name:     "valid dependencies",
			modifyFn: func(_ *Deps) {},
			wantErr:  false,
		},
		{
			name: "missing storage",
			modifyFn: func(d *Deps) {
				d.Storage = nil
			},
			wantErr: true,
			errMsg:  "storage dependency is required",
		},
		{
			name: "missing LLM client",
			modifyFn: func(d *Deps) {
				d.LLMClient = nil
			},
			wantErr: true,
			errMsg:  "LLM client dependency is required",
		},
		{
			name: "missing session store",
			modifyFn: func(d *Deps) {
				d.SessionStore = nil
			},
			wantErr: true,
			errMsg:  "session store dependency is required",
		},
		{
			name: "missing report store",
			modifyFn: func(d *Deps) {
				d.ReportStore = nil
			},
			wantErr: true,
			errMsg:  "report store dependency is required",
		},
		{
			name: "missing validator",
			modifyFn: func(d *Deps) {
				d.Validator = nil
			},
			wantErr: true,
			errMsg:  "validator dependency is required",
		},
		{
			name: "missing fix applier",
			modifyFn: func(d *Deps) {
				d.FixApplier = nil
			},
			wantErr: true,
			errMsg:  "fix applier dependency is required",
		},
		{
			name: "missing prompt builder",
			modifyFn: func(d *Deps) {
				d.PromptBuilder = nil
			},
			wantErr: true,
			errMsg:  "prompt builder dependency is required",
		},
		{
			name: "missing formatter",
			modifyFn: func(d *Deps) {
				d.Formatter = nil
			},
			wantErr: true,
			errMsg:  "formatter dependency is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := validDeps
			tt.modifyFn(&deps)
			err := deps.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewEngine(t *testing.T) {
	validDeps := Deps{
		Storage:       &mockStorage{},
		LLMClient:     &mockLLMClient{},
		SessionStore:  &mockSessionStore{},
		ReportStore:   &mockReportStore{},
		Validator:     &mockReportValidator{},
		FixApplier:    &mockFixApplier{},
		PromptBuilder: &mockPromptBuilder{},
		Formatter:     &mockReportFormatter{},
	}

	tests := []struct {
		deps    Deps
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name:    "valid dependencies",
			deps:    validDeps,
			wantErr: false,
		},
		{
			name: "invalid dependencies",
			deps: Deps{
				// Missing required dependencies
			},
			wantErr: true,
			errMsg:  "invalid dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(tt.deps)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, engine)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, engine)
				assert.Equal(t, tt.deps, engine.deps)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 50, config.MaxIssuesPerReport)
	assert.Equal(t, 3600, config.SessionTimeout)
	assert.False(t, config.EnableAutoFix)
}

func TestNewEngineWithConfig(t *testing.T) {
	validDeps := Deps{
		Storage:       &mockStorage{},
		LLMClient:     &mockLLMClient{},
		SessionStore:  &mockSessionStore{},
		ReportStore:   &mockReportStore{},
		Validator:     &mockReportValidator{},
		FixApplier:    &mockFixApplier{},
		PromptBuilder: &mockPromptBuilder{},
		Formatter:     &mockReportFormatter{},
	}

	customConfig := &Config{
		MaxRetries:         5,
		MaxIssuesPerReport: 100,
		SessionTimeout:     7200,
		EnableAutoFix:      true,
	}

	tests := []struct {
		deps    Deps
		config  *Config
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name:    "valid dependencies with custom config",
			deps:    validDeps,
			config:  customConfig,
			wantErr: false,
		},
		{
			name:    "valid dependencies with nil config",
			deps:    validDeps,
			config:  nil,
			wantErr: false,
		},
		{
			name: "invalid dependencies",
			deps: Deps{
				// Missing required dependencies
			},
			config:  customConfig,
			wantErr: true,
			errMsg:  "invalid dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngineWithConfig(tt.deps, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, engine)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, engine)
				assert.Equal(t, tt.deps, engine.deps)
			}
		})
	}
}
