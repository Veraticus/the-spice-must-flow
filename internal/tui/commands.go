// Package tui provides a terminal user interface for transaction classification.
package tui

import (
	"context"
	crypto_rand "crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

// tickCmd returns a command that sends a tick message every second for elapsed time updates.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// loadTransactions loads transactions from storage.
func (m Model) loadTransactions() tea.Cmd {
	return func() tea.Msg {
		if m.storage == nil {
			return dataLoadedMsg{
				dataType: "transactions",
				err:      fmt.Errorf("storage not configured"),
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		transactions, err := m.storage.GetTransactionsToClassify(ctx, nil)
		if err != nil {
			return dataLoadedMsg{
				dataType: "transactions",
				err:      err,
			}
		}

		return transactionsLoadedMsg{
			transactions: transactions,
			err:          nil,
		}
	}
}

// loadCategories loads categories from storage.
func (m Model) loadCategories() tea.Cmd {
	return func() tea.Msg {
		if m.storage == nil {
			return dataLoadedMsg{
				dataType: "categories",
				err:      fmt.Errorf("storage not configured"),
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		categories, err := m.storage.GetCategories(ctx)
		if err != nil {
			return dataLoadedMsg{
				dataType: "categories",
				err:      err,
			}
		}

		return categoriesLoadedMsg{
			categories: categories,
			err:        nil,
		}
	}
}

// loadCheckPatterns loads check patterns from storage.
func (m Model) loadCheckPatterns() tea.Cmd {
	return func() tea.Msg {
		if m.storage == nil {
			return dataLoadedMsg{
				dataType: "checkpatterns",
				err:      fmt.Errorf("storage not configured"),
			}
		}

		// Check patterns would be loaded here if the method exists
		// For now, return empty patterns
		patterns := []model.CheckPattern{}

		return checkPatternsLoadedMsg{
			patterns: patterns,
			err:      nil,
		}
	}
}

// generateTestData generates fake data for testing.
func (m Model) generateTestData() tea.Cmd {
	return func() tea.Msg {
		// Generate test transactions
		transactions := generateTestTransactions(m.config.TestData.TransactionCount)

		// Generate test categories
		categories := generateTestCategories()

		// Return as loaded messages
		go func() {
			time.Sleep(100 * time.Millisecond)
			m.transactions = transactions
			m.categories = categories
		}()

		return transactionsLoadedMsg{
			transactions: transactions,
		}
	}
}

// generateTestTransactions creates realistic test transactions.
func generateTestTransactions(count int) []model.Transaction {

	merchants := []struct {
		name     string
		category string
		channel  string
		minAmt   float64
		maxAmt   float64
	}{
		{"Whole Foods Market", "Groceries", "in store", 20, 200},
		{"Shell Oil 12345", "Transportation", "in store", 30, 80},
		{"Netflix.com", "Entertainment", "online", 15.99, 15.99},
		{"Amazon.com", "Shopping", "online", 10, 500},
		{"Starbucks", "Dining Out", "in store", 3, 12},
		{"Target", "Shopping", "in store", 15, 300},
		{"Uber", "Transportation", "online", 8, 45},
		{"Spotify", "Entertainment", "online", 9.99, 9.99},
		{"CVS Pharmacy", "Healthcare", "in store", 10, 150},
		{"Home Depot", "Home Supplies", "in store", 25, 400},
		{"Chipotle", "Dining Out", "in store", 8, 25},
		{"Delta Airlines", "Travel", "online", 150, 800},
		{"Planet Fitness", "Fitness", "online", 10, 10},
		{"Trader Joe's", "Groceries", "in store", 25, 150},
		{"Exxon Mobil", "Transportation", "in store", 35, 75},
	}

	var transactions []model.Transaction
	baseDate := time.Now()

	for i := 0; i < count; i++ {
		merchantIdx, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(int64(len(merchants))))
		merchant := merchants[merchantIdx.Int64()]

		// Generate random amount between min and max
		var amount float64
		if merchant.minAmt == merchant.maxAmt {
			// Fixed price (like subscriptions)
			amount = merchant.minAmt
		} else {
			// Random amount in range
			rangeBig := big.NewInt(int64((merchant.maxAmt - merchant.minAmt) * 100))
			randomOffset, _ := crypto_rand.Int(crypto_rand.Reader, rangeBig)
			amount = merchant.minAmt + float64(randomOffset.Int64())/100.0
		}

		// Add some variation to merchant names
		merchantName := merchant.name
		variationChance, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(10))
		if variationChance.Int64() < 3 { // 30% chance
			variation, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(9999))
			merchantName = fmt.Sprintf("%s #%04d", merchant.name, variation.Int64())
		}

		// Determine transaction type
		txnType := "debit"
		typeChance, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(10))
		if typeChance.Int64() < 1 { // 10% chance
			txnType = "credit"
		}

		// Most transactions are expenses
		direction := model.DirectionExpense
		dirChance, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(20))
		if dirChance.Int64() < 1 { // 5% chance
			direction = model.DirectionIncome
			// Random income between 0 and 5000
			incomeAmount, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(500000))
			amount = float64(incomeAmount.Int64()) / 100.0
		}

		transaction := model.Transaction{
			ID:           fmt.Sprintf("test_txn_%d", i),
			Hash:         fmt.Sprintf("hash_%d", i),
			AccountID:    "test_account_001",
			Date:         baseDate.AddDate(0, 0, -(i / 3)),
			Name:         fmt.Sprintf("%s %s", merchantName, baseDate.AddDate(0, 0, -(i/3)).Format("01/02")),
			MerchantName: merchantName,
			Amount:       roundToTwoDecimals(amount),
			Type:         txnType,
			Direction:    direction,
		}

		// Some transactions already classified
		classifiedChance, _ := crypto_rand.Int(crypto_rand.Reader, big.NewInt(10))
		if classifiedChance.Int64() < 3 { // 30% chance
			// Add category to the Category slice
			transaction.Category = []string{merchant.category}
		}

		transactions = append(transactions, transaction)
	}

	return transactions
}

// generateTestCategories creates test categories.
func generateTestCategories() []model.Category {
	categoryNames := []string{
		"Groceries",
		"Dining Out",
		"Transportation",
		"Entertainment",
		"Shopping",
		"Healthcare",
		"Utilities",
		"Home Supplies",
		"Education",
		"Travel",
		"Fitness",
		"Personal Care",
		"Gifts",
		"Subscriptions",
		"Insurance",
		"Taxes",
		"Investments",
		"Charity",
		"Pets",
		"Other",
	}

	categories := make([]model.Category, 0, len(categoryNames))
	for i, name := range categoryNames {
		categories = append(categories, model.Category{
			ID:          i + 1,
			Name:        name,
			Description: fmt.Sprintf("Expenses related to %s", name),
			CreatedAt:   time.Now().Add(-time.Duration(i) * 24 * time.Hour),
		})
	}

	return categories
}

// Helper functions

func roundToTwoDecimals(f float64) float64 {
	return float64(int(f*100)) / 100
}
