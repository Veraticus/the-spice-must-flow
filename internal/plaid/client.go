// Package plaid provides a client for interacting with the Plaid API.
package plaid

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/common"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/plaid/plaid-go/v20/plaid"
)

// Config holds Plaid API configuration.
type Config struct {
	ClientID    string
	Secret      string
	Environment string // sandbox, development, or production
	AccessToken string
}

// Validate ensures all required fields are present.
func (c *Config) Validate() error {
	if c.ClientID == "" {
		return fmt.Errorf("plaid client ID is required")
	}
	if c.Secret == "" {
		return fmt.Errorf("plaid secret is required")
	}
	if c.AccessToken == "" {
		return fmt.Errorf("plaid access token is required")
	}
	if c.Environment == "" {
		return fmt.Errorf("plaid environment is required")
	}

	validEnvs := map[string]bool{
		"sandbox":     true,
		"development": true,
		"production":  true,
	}
	if !validEnvs[c.Environment] {
		return fmt.Errorf("invalid Plaid environment: must be sandbox, development, or production")
	}

	return nil
}

// Client implements the PlaidClient interface.
type Client struct {
	client      *plaid.APIClient
	logger      *slog.Logger
	retryOpts   *service.RetryOptions
	accessToken string
}

// NewClient creates a new Plaid client with the given configuration.
func NewClient(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Configure Plaid client based on environment
	configuration := plaid.NewConfiguration()
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", cfg.ClientID)
	configuration.AddDefaultHeader("PLAID-SECRET", cfg.Secret)

	switch cfg.Environment {
	case "sandbox":
		configuration.UseEnvironment(plaid.Sandbox)
	case "development":
		configuration.UseEnvironment(plaid.Development)
	case "production":
		configuration.UseEnvironment(plaid.Production)
	}

	client := plaid.NewAPIClient(configuration)

	return &Client{
		client:      client,
		accessToken: cfg.AccessToken,
		logger:      slog.Default().With("component", "plaid"),
		retryOpts: &service.RetryOptions{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		},
	}, nil
}

// GetTransactions fetches transactions from Plaid within the specified date range.
func (c *Client) GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if startDate.After(endDate) {
		return nil, fmt.Errorf("start date must be before end date")
	}

	c.logger.Info("Fetching transactions from Plaid",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"))

	var allTransactions []plaid.Transaction
	offset := int32(0)
	const pageSize = int32(500) // Plaid's max page size

	// Fetch all transactions with pagination
	for {
		var plaidTransactions []plaid.Transaction

		retryErr := common.WithRetry(ctx, func() error {
			request := plaid.NewTransactionsGetRequest(
				c.accessToken,
				startDate.Format("2006-01-02"),
				endDate.Format("2006-01-02"),
			)
			// Set options for pagination
			options := plaid.TransactionsGetRequestOptions{
				Count:  plaid.PtrInt32(pageSize),
				Offset: plaid.PtrInt32(offset),
			}
			request.SetOptions(options)

			resp, _, err := c.client.PlaidApi.TransactionsGet(ctx).TransactionsGetRequest(*request).Execute()
			if err != nil {
				if plaidError := extractPlaidError(err); plaidError != nil {
					// Check for rate limit error
					if plaidError.ErrorCode == "RATE_LIMIT_EXCEEDED" {
						c.logger.Warn("Rate limit hit, will retry", "error", plaidError.ErrorMessage)
						return &common.RetryableError{Err: err, Retryable: true}
					}
					return fmt.Errorf("plaid API error: %s - %s", plaidError.ErrorCode, plaidError.ErrorMessage)
				}
				return fmt.Errorf("failed to fetch transactions: %w", err)
			}

			plaidTransactions = resp.GetTransactions()
			totalTransactions := resp.GetTotalTransactions()

			c.logger.Debug("Fetched transaction batch",
				"count", len(plaidTransactions),
				"offset", offset,
				"total", totalTransactions)

			return nil
		}, *c.retryOpts)

		if retryErr != nil {
			return nil, retryErr
		}

		allTransactions = append(allTransactions, plaidTransactions...)

		// Check if we've fetched all transactions
		if len(plaidTransactions) < int(pageSize) {
			break
		}

		offset += pageSize
	}

	c.logger.Info("Fetched all transactions", "count", len(allTransactions))

	// Convert Plaid transactions to our model
	transactions := make([]model.Transaction, 0, len(allTransactions))
	for _, pt := range allTransactions {
		tx := c.mapPlaidTransaction(pt)
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetAccounts fetches account IDs from Plaid.
func (c *Client) GetAccounts(ctx context.Context) ([]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	c.logger.Info("Fetching accounts from Plaid")

	var accounts []plaid.AccountBase
	retryErr := common.WithRetry(ctx, func() error {
		request := plaid.NewAccountsGetRequest(c.accessToken)
		resp, _, err := c.client.PlaidApi.AccountsGet(ctx).AccountsGetRequest(*request).Execute()
		if err != nil {
			if plaidError := extractPlaidError(err); plaidError != nil {
				if plaidError.ErrorCode == "RATE_LIMIT_EXCEEDED" {
					c.logger.Warn("Rate limit hit, will retry", "error", plaidError.ErrorMessage)
					return &common.RetryableError{Err: err, Retryable: true}
				}
				return fmt.Errorf("plaid API error: %s - %s", plaidError.ErrorCode, plaidError.ErrorMessage)
			}
			return fmt.Errorf("failed to fetch accounts: %w", err)
		}

		accounts = resp.GetAccounts()
		return nil
	}, *c.retryOpts)

	if retryErr != nil {
		return nil, retryErr
	}

	c.logger.Info("Fetched accounts", "count", len(accounts))

	// Extract account IDs
	accountIDs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.GetAccountId())
	}

	return accountIDs, nil
}

// mapPlaidTransaction converts a Plaid transaction to our internal model.
func (c *Client) mapPlaidTransaction(pt plaid.Transaction) model.Transaction {
	// Parse the date
	date, err := time.Parse("2006-01-02", pt.GetDate())
	if err != nil {
		c.logger.Error("Failed to parse transaction date", "date", pt.GetDate(), "error", err)
		date = time.Now() // Fallback to current date
	}

	// Get merchant name, falling back to name if not available
	merchantName := pt.GetMerchantName()
	if merchantName == "" {
		merchantName = pt.GetName()
	}

	// Clean up the merchant name
	merchantName = cleanMerchantName(merchantName)

	// Get categories - Plaid provides a hierarchy
	var plaidCategory string
	if categories := pt.GetCategory(); len(categories) > 0 {
		plaidCategory = strings.Join(categories, " > ")
	}

	tx := model.Transaction{
		Date:          date,
		ID:            pt.GetTransactionId(),
		Name:          pt.GetName(),
		MerchantName:  merchantName,
		AccountID:     pt.GetAccountId(),
		PlaidCategory: plaidCategory,
		Amount:        pt.GetAmount(),
	}

	// Generate hash for deduplication
	tx.Hash = tx.GenerateHash()

	return tx
}

// cleanMerchantName standardizes merchant names by removing common suffixes and normalizing format.
func cleanMerchantName(name string) string {
	// Convert to title case manually to avoid deprecated strings.Title
	words := strings.Fields(strings.ToLower(name))
	for i, word := range words {
		if word != "" {
			// Handle special cases
			runes := []rune(word)
			for j := 0; j < len(runes); j++ {
				if j == 0 || (j > 0 && !isLetter(runes[j-1])) {
					runes[j] = toUpper(runes[j])
				}
			}
			words[i] = string(runes)
		}
	}
	name = strings.Join(words, " ")

	// Handle common patterns like "MERCHANT 123456789" first
	// Use strings.Fields to split by any whitespace and rejoin with single spaces
	parts := strings.Fields(name)
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// If the last part is all digits and longer than 5 chars, it's probably a transaction ID
		if len(lastPart) > 5 && isAllDigits(lastPart) {
			parts = parts[:len(parts)-1]
		}
	}

	// Reconstruct name without transaction ID
	name = strings.Join(parts, " ")

	// Remove common payment processor suffixes
	suffixes := []string{
		" Llc",
		" Inc",
		" Corp",
		" Corporation",
		" Company",
		" Co",
		" Ltd",
		" Limited",
	}

	// Keep removing suffixes until none are found (handles multiple suffixes)
	changed := true
	for changed {
		changed = false
		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				name = strings.TrimSuffix(name, suffix)
				changed = true
			}
		}
	}

	// Final trim
	return strings.TrimSpace(name)
}

// isAllDigits checks if a string contains only digits.
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isLetter checks if a rune is a letter.
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// toUpper converts a rune to uppercase.
func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// extractPlaidError attempts to extract a Plaid error from a generic error.
func extractPlaidError(err error) *plaid.PlaidError {
	plaidErr, convErr := plaid.ToPlaidError(err)
	if convErr != nil {
		return nil
	}
	return &plaidErr
}

// Ensure Client implements PlaidClient interface.
var _ TransactionFetcher = (*Client)(nil)
