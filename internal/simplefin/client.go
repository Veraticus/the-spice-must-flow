package simplefin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// SimpleFINClient implements the TransactionFetcher interface for SimpleFIN
type SimpleFINClient struct {
	accessURL  string
	httpClient *http.Client
}

// SimpleFIN API response types
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
	Posted      int64  `json:"posted"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	Payee       string `json:"payee"`
	Pending     bool   `json:"pending"`
}

// NewClient creates a new SimpleFIN client, using saved auth if available
func NewClient(token string) (*SimpleFINClient, error) {
	// Load or claim auth
	auth, err := LoadOrClaimAuth(token)
	if err != nil {
		return nil, fmt.Errorf("failed to load/claim auth: %w", err)
	}
	
	return &SimpleFINClient{
		accessURL: auth.AccessURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// claimToken exchanges a claim token for an access URL
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
	req, err := http.NewRequest("POST", claimURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create claim request: %w", err)
	}
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to claim access URL: %w", err)
	}
	defer resp.Body.Close()
	
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

// GetTransactions fetches transactions from SimpleFIN
func (c *SimpleFINClient) GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error) {
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
	
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SimpleFIN API error: %d - %s", resp.StatusCode, string(body))
	}

	var accountSet accountSet
	if err := json.NewDecoder(resp.Body).Decode(&accountSet); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert SimpleFIN transactions to our model
	var transactions []model.Transaction
	for _, account := range accountSet.Accounts {
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

			// Create our transaction model
			modelTx := model.Transaction{
				ID:           fmt.Sprintf("%s_%s", account.ID, tx.ID),
				Date:         date,
				Name:         tx.Description,
				MerchantName: normalizeMerchant(tx.Payee),
				Amount:       amount,
				AccountID:    account.ID,
				// SimpleFIN doesn't provide categories
				PlaidCategory: "",
			}

			// Generate hash for deduplication
			modelTx.Hash = modelTx.GenerateHash()

			transactions = append(transactions, modelTx)
		}
	}

	return transactions, nil
}

// GetAccounts returns the list of account IDs
func (c *SimpleFINClient) GetAccounts(ctx context.Context) ([]string, error) {
	// SimpleFIN uses the access URL with /accounts endpoint
	accountsURL := c.accessURL + "/accounts"
	req, err := http.NewRequestWithContext(ctx, "GET", accountsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SimpleFIN API error: %d - %s", resp.StatusCode, string(body))
	}

	var accountSet accountSet
	if err := json.NewDecoder(resp.Body).Decode(&accountSet); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var accountIDs []string
	for _, account := range accountSet.Accounts {
		accountIDs = append(accountIDs, account.ID)
	}

	return accountIDs, nil
}

// parseAmount converts SimpleFIN amount string (in cents) to float64 dollars
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

// normalizeMerchant performs basic merchant name normalization
func normalizeMerchant(raw string) string {
	// This is a simple implementation - we'll enhance it later
	merchant := strings.TrimSpace(raw)
	
	// Remove common suffixes
	merchant = strings.TrimSuffix(merchant, " LLC")
	merchant = strings.TrimSuffix(merchant, " INC")
	merchant = strings.TrimSuffix(merchant, " CORP")
	
	// Title case
	merchant = strings.Title(strings.ToLower(merchant))
	
	return merchant
}

