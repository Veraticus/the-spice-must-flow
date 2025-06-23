package tui

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

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
	rand.Seed(time.Now().UnixNano())

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
		merchant := merchants[rand.Intn(len(merchants))]
		amount := merchant.minAmt + rand.Float64()*(merchant.maxAmt-merchant.minAmt)

		// Add some variation to merchant names
		merchantName := merchant.name
		if rand.Float64() < 0.3 {
			merchantName = fmt.Sprintf("%s #%04d", merchant.name, rand.Intn(9999))
		}

		// Determine transaction type
		txnType := "debit"
		if rand.Float64() < 0.1 {
			txnType = "credit"
		}

		// Most transactions are expenses
		direction := model.DirectionExpense
		if rand.Float64() < 0.05 {
			direction = model.DirectionIncome
			amount = rand.Float64() * 5000 // Random income
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
		if rand.Float64() < 0.3 {
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

	var categories []model.Category
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
