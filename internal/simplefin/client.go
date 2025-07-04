package simplefin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Client implements the TransactionFetcher interface for SimpleFIN.
type Client struct {
	httpClient *http.Client
	accessURL  string
}

// SimpleFIN API response types.
type accountSet struct {
	Accounts []account `json:"accounts"`
}

type account struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Currency     string        `json:"currency"`
	Balance      string        `json:"balance"`
	Transactions []transaction `json:"transactions"`
}

type transaction struct {
	ID          string `json:"id"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	Payee       string `json:"payee"`
	Posted      int64  `json:"posted"`
	Pending     bool   `json:"pending"`
}

// NewClient creates a new SimpleFIN client, using saved auth if available.
func NewClient(token string) (*Client, error) {
	// Load or claim auth
	auth, err := LoadOrClaimAuth(token)
	if err != nil {
		return nil, fmt.Errorf("failed to load/claim auth: %w", err)
	}

	return &Client{
		accessURL: auth.AccessURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// claimToken exchanges a claim token for an access URL.
func claimToken(token string) (string, error) {
	// SimpleFIN tokens are base64-encoded claim URLs
	// Decode the token to get the claim URL
	decodedBytes, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		// Try standard encoding if URL encoding fails
		decodedBytes, err = base64.StdEncoding.DecodeString(token)
		if err != nil {
			return "", fmt.Errorf("failed to decode SimpleFIN token: %w", err)
		}
	}

	claimURL := string(decodedBytes)

	// Validate it's a proper URL
	if !strings.HasPrefix(claimURL, "http://") && !strings.HasPrefix(claimURL, "https://") {
		return "", fmt.Errorf("decoded token is not a valid URL: %s", claimURL)
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Claim the access URL by POSTing to the claim URL
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claimURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create claim request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to claim access URL: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to claim SimpleFIN access: %d - %s", resp.StatusCode, string(body))
	}

	// Read the access URL from the response
	accessURLBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read access URL: %w", err)
	}

	accessURL := strings.TrimSpace(string(accessURLBytes))
	if !strings.HasPrefix(accessURL, "http://") && !strings.HasPrefix(accessURL, "https://") {
		return "", fmt.Errorf("invalid access URL received: %s", accessURL)
	}

	return accessURL, nil
}

// GetTransactions fetches transactions from SimpleFIN.
func (c *Client) GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error) {
	// SimpleFIN uses the access URL with /accounts endpoint
	baseURL := c.accessURL + "/accounts"

	// Parse URL to add query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Add query parameters for date range
	q := u.Query()
	q.Set("start-date", fmt.Sprintf("%d", startDate.Unix()))
	// Note: end-date in SimpleFIN is exclusive, so we add 1 day
	q.Set("end-date", fmt.Sprintf("%d", endDate.AddDate(0, 0, 1).Unix()))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	slog.Debug("Requesting SimpleFIN transactions",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
		"url_params", u.RawQuery)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SimpleFIN API error: %d - %s", resp.StatusCode, string(body))
	}

	var accounts accountSet
	if err := json.NewDecoder(resp.Body).Decode(&accounts); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert SimpleFIN transactions to our model
	var transactions []model.Transaction
	for _, account := range accounts.Accounts {
		for _, tx := range account.Transactions {
			// Skip pending transactions
			if tx.Pending {
				continue
			}

			// Convert Unix timestamp to time.Time
			date := time.Unix(tx.Posted, 0)

			// Skip transactions outside our date range
			if date.Before(startDate) || date.After(endDate) {
				continue
			}

			// Convert amount from cents string to float64
			amount, err := parseAmount(tx.Amount)
			if err != nil {
				return nil, fmt.Errorf("failed to parse amount %s: %w", tx.Amount, err)
			}

			// SimpleFIN doesn't provide categories
			var categories []string

			// Try to infer transaction type from description or payee
			transactionType := inferTransactionType(tx.Description, tx.Payee)

			// Create our transaction model
			modelTx := model.Transaction{
				ID:           fmt.Sprintf("%s_%s", account.ID, tx.ID),
				Date:         date,
				Name:         tx.Description,
				MerchantName: normalizeMerchant(tx.Payee),
				Amount:       amount,
				AccountID:    account.ID,
				Category:     categories,
				Type:         transactionType,
			}

			// Generate hash for deduplication
			modelTx.Hash = modelTx.GenerateHash()

			transactions = append(transactions, modelTx)
		}
	}

	return transactions, nil
}

// GetAccounts returns the list of account IDs.
func (c *Client) GetAccounts(ctx context.Context) ([]string, error) {
	// SimpleFIN uses the access URL with /accounts endpoint
	accountsURL := c.accessURL + "/accounts"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, accountsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SimpleFIN API error: %d - %s", resp.StatusCode, string(body))
	}

	var accounts accountSet
	if err := json.NewDecoder(resp.Body).Decode(&accounts); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	accountIDs := make([]string, 0, len(accounts.Accounts))
	for _, account := range accounts.Accounts {
		accountIDs = append(accountIDs, account.ID)
	}

	return accountIDs, nil
}

// Close closes the SimpleFIN client (no-op for now).
func (c *Client) Close() error {
	return nil
}

// parseAmount converts SimpleFIN amount string (in cents) to float64 dollars.
func parseAmount(amountStr string) (float64, error) {
	// SimpleFIN amounts are in cents as strings
	// Negative amounts represent debits
	var cents int64
	if _, err := fmt.Sscanf(amountStr, "%d", &cents); err != nil {
		return 0, err
	}

	// Convert to dollars and make debits positive for our use case
	amount := float64(cents) / 100.0
	if amount < 0 {
		amount = -amount
	}

	return amount, nil
}

// normalizeMerchant performs basic merchant name normalization.
func normalizeMerchant(raw string) string {
	// This is a simple implementation - we'll enhance it later
	merchant := strings.TrimSpace(raw)

	// Remove common suffixes
	merchant = strings.TrimSuffix(merchant, " LLC")
	merchant = strings.TrimSuffix(merchant, " INC")
	merchant = strings.TrimSuffix(merchant, " CORP")

	// Title case using simple approach
	words := strings.Fields(strings.ToLower(merchant))
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}
	merchant = strings.Join(words, " ")

	return merchant
}

// inferTransactionType tries to guess transaction type from description/payee.
func inferTransactionType(description, payee string) string {
	combined := strings.ToLower(description + " " + payee)

	// Check for common patterns
	switch {
	case strings.Contains(combined, "check #") || strings.Contains(combined, "check paid"):
		return "CHECK"
	case strings.Contains(combined, "atm") || strings.Contains(combined, "cash withdrawal"):
		return "ATM"
	case strings.Contains(combined, "direct deposit") || strings.Contains(combined, "payroll"):
		return "DIRECTDEP"
	case strings.Contains(combined, "wire transfer") || strings.Contains(combined, "wire from"):
		return "XFER"
	case strings.Contains(combined, "online payment") || strings.Contains(combined, "web payment"):
		return "ONLINE"
	case strings.Contains(combined, "interest paid"):
		return "INT"
	case strings.Contains(combined, "fee") || strings.Contains(combined, "service charge"):
		return "FEE"
	case strings.Contains(combined, "venmo") || strings.Contains(combined, "zelle") || strings.Contains(combined, "paypal"):
		return "PAYMENT"
	default:
		// Default to DEBIT for purchases
		return "DEBIT"
	}
}
