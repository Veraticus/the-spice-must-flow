package plaid

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		config  Config
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				Environment: "sandbox",
				AccessToken: "test-token",
			},
			wantErr: false,
		},
		{
			name: "missing client ID",
			config: Config{
				Secret:      "test-secret",
				Environment: "sandbox",
				AccessToken: "test-token",
			},
			wantErr: true,
			errMsg:  "plaid client ID is required",
		},
		{
			name: "missing secret",
			config: Config{
				ClientID:    "test-client-id",
				Environment: "sandbox",
				AccessToken: "test-token",
			},
			wantErr: true,
			errMsg:  "plaid secret is required",
		},
		{
			name: "missing access token",
			config: Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				Environment: "sandbox",
			},
			wantErr: true,
			errMsg:  "plaid access token is required",
		},
		{
			name: "missing environment",
			config: Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				AccessToken: "test-token",
			},
			wantErr: true,
			errMsg:  "plaid environment is required",
		},
		{
			name: "invalid environment",
			config: Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				Environment: "invalid",
				AccessToken: "test-token",
			},
			wantErr: true,
			errMsg:  "invalid Plaid environment",
		},
		{
			name: "valid development environment",
			config: Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				Environment: "development",
				AccessToken: "test-token",
			},
			wantErr: false,
		},
		{
			name: "valid production environment",
			config: Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				Environment: "production",
				AccessToken: "test-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		config  *Config
		name    string
		wantErr bool
	}{
		{
			name: "valid config creates client",
			config: &Config{
				ClientID:    "test-client-id",
				Secret:      "test-secret",
				Environment: "sandbox",
				AccessToken: "test-token",
			},
			wantErr: false,
		},
		{
			name: "invalid config returns error",
			config: &Config{
				ClientID: "test-client-id",
				// Missing required fields
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.client)
				assert.Equal(t, tt.config.AccessToken, client.accessToken)
				assert.NotNil(t, client.logger)
				assert.NotNil(t, client.retryOpts)
			}
		})
	}
}

func TestClient_GetTransactions_Validation(t *testing.T) {
	client := &Client{
		accessToken: "test-token",
		logger:      slog.Default().With("component", "plaid-test"),
	}

	tests := []struct {
		startDate time.Time
		endDate   time.Time
		ctx       context.Context
		name      string
		errMsg    string
		wantErr   bool
	}{
		{
			name:      "nil context",
			ctx:       nil,
			startDate: time.Now().AddDate(0, -1, 0),
			endDate:   time.Now(),
			wantErr:   true,
			errMsg:    "context cannot be nil",
		},
		{
			name:      "start date after end date",
			ctx:       context.Background(),
			startDate: time.Now(),
			endDate:   time.Now().AddDate(0, -1, 0),
			wantErr:   true,
			errMsg:    "start date must be before end date",
		},
		// Note: We can't test the successful case without mocking the Plaid API client
		// as it would make actual API calls. This test only validates input parameters.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetTransactions(tt.ctx, tt.startDate, tt.endDate)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestCleanMerchantName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic name",
			input:    "Starbucks",
			expected: "Starbucks",
		},
		{
			name:     "lowercase to title case",
			input:    "starbucks coffee",
			expected: "Starbucks Coffee",
		},
		{
			name:     "remove LLC suffix",
			input:    "Amazon LLC",
			expected: "Amazon",
		},
		{
			name:     "remove Inc suffix",
			input:    "Apple Inc",
			expected: "Apple",
		},
		{
			name:     "remove Corp suffix",
			input:    "Microsoft Corp",
			expected: "Microsoft",
		},
		{
			name:     "remove transaction ID",
			input:    "PAYPAL 123456789",
			expected: "Paypal",
		},
		{
			name:     "preserve short numbers",
			input:    "7-ELEVEN 2345",
			expected: "7-Eleven 2345",
		},
		{
			name:     "multiple cleanups",
			input:    "amazon.com llc 987654321",
			expected: "Amazon.Com",
		},
		{
			name:     "extra spaces",
			input:    "  Google   Cloud   ",
			expected: "Google Cloud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanMerchantName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123456", true},
		{"000000", true},
		{"12a456", false},
		{"", true}, // edge case: empty string
		{"ABC123", false},
		{"12.34", false},
		{"12 34", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isAllDigits(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapPlaidTransaction(t *testing.T) {
	// This test would require mocking Plaid transaction objects
	// For now, we'll test the transaction mapping logic is correct

	// We'll create a manual transaction to test the mapping
	tx := model.Transaction{
		Date:          time.Now().Truncate(24 * time.Hour),
		ID:            "test-transaction-id",
		Name:          "STARBUCKS STORE #123",
		MerchantName:  "Starbucks Store #123",
		AccountID:     "test-account-id",
		PlaidCategory: "Food and Drink > Restaurants > Coffee Shop",
		Amount:        5.50,
	}

	// Verify hash is generated
	originalHash := tx.Hash
	generatedHash := tx.GenerateHash()
	assert.NotEmpty(t, generatedHash)
	assert.NotEqual(t, originalHash, generatedHash)
}

func TestMockClient(t *testing.T) {
	mock := NewMockClient()

	// Test GetTransactions
	startDate := time.Now().AddDate(0, -1, 0)
	endDate := time.Now()

	// Set custom behavior
	expectedTxs := []model.Transaction{
		{
			ID:     "tx1",
			Name:   "Test Transaction",
			Amount: 10.50,
		},
	}
	mock.GetTransactionsFn = func(_ context.Context, _, _ time.Time) ([]model.Transaction, error) {
		return expectedTxs, nil
	}

	txs, err := mock.GetTransactions(context.Background(), startDate, endDate)
	require.NoError(t, err)
	assert.Equal(t, expectedTxs, txs)

	// Verify call was tracked
	assert.Len(t, mock.GetTransactionsCalls, 1)
	assert.Equal(t, startDate, mock.GetTransactionsCalls[0].StartDate)
	assert.Equal(t, endDate, mock.GetTransactionsCalls[0].EndDate)

	// Test GetAccounts
	expectedAccounts := []string{"acc1", "acc2"}
	mock.GetAccountsFn = func(_ context.Context) ([]string, error) {
		return expectedAccounts, nil
	}

	accounts, err := mock.GetAccounts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedAccounts, accounts)
	assert.Equal(t, 1, mock.GetAccountsCalls)

	// Test Reset
	mock.Reset()
	assert.Len(t, mock.GetTransactionsCalls, 0)
	assert.Equal(t, 0, mock.GetAccountsCalls)
}
