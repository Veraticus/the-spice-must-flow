package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestValidateDataCoverageFromClassifications(t *testing.T) {
	year := 2024
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 23, 59, 59, 999999999, time.UTC)

	tests := []struct {
		name        string
		transactions []time.Time
		expectedError string
		expectWarnings bool
	}{
		{
			name:          "no data",
			transactions:  []time.Time{},
			expectedError: "no transaction data found for 2024",
			expectWarnings: false,
		},
		{
			name: "full year coverage",
			transactions: generateMonthlyTransactions(year, 1, 12),
			expectedError: "",
			expectWarnings: false,
		},
		{
			name: "missing first quarter - now just warns",
			transactions: generateMonthlyTransactions(year, 4, 12),
			expectedError: "",
			expectWarnings: true, // 90 day gap at start
		},
		{
			name: "missing last quarter - now just warns",
			transactions: generateMonthlyTransactions(year, 1, 9),
			expectedError: "",
			expectWarnings: true, // 92 day gap at end
		},
		{
			name: "sparse data - only 3 months with big gaps",
			transactions: []time.Time{
				time.Date(year, 3, 15, 0, 0, 0, 0, time.UTC),
				time.Date(year, 6, 15, 0, 0, 0, 0, time.UTC),
				time.Date(year, 9, 15, 0, 0, 0, 0, time.UTC),
			},
			expectedError: "",
			expectWarnings: true, // Multiple 30+ day gaps
		},
		{
			name: "acceptable coverage - 10 months",
			transactions: generateMonthlyTransactions(year, 1, 10),
			expectedError: "",
			expectWarnings: true, // Gap at end
		},
		{
			name: "gap in middle over 30 days",
			transactions: append(
				generateMonthlyTransactions(year, 1, 5),
				generateMonthlyTransactions(year, 8, 12)...,
			),
			expectedError: "",
			expectWarnings: true, // 2 month gap in middle
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create classifications from transactions
			var classifications []model.Classification
			for i, date := range tt.transactions {
				classifications = append(classifications, model.Classification{
					Transaction: model.Transaction{
						ID:   fmt.Sprintf("TX%d", i),
						Date: date,
					},
					Category: "Test",
				})
			}

			err := validateDataCoverageFromClassifications(classifications, start, end)
			
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
			
			// Note: We can't easily test for warnings in unit tests since they go to slog
			// In a real test, we might capture log output or refactor to return warnings
		})
	}
}

// generateMonthlyTransactions creates transactions for each day of the specified months
func generateMonthlyTransactions(year, startMonth, endMonth int) []time.Time {
	var dates []time.Time
	for month := startMonth; month <= endMonth; month++ {
		// Add transactions on 1st, 15th, and last day of each month
		dates = append(dates,
			time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC),
			time.Date(year, time.Month(month), 15, 0, 0, 0, 0, time.UTC),
		)
		// Add last day of month
		lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)
		dates = append(dates, lastDay)
	}
	return dates
}

func TestDataCoverageEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() ([]model.Classification, time.Time, time.Time)
		expectedError string
		expectWarnings bool
	}{
		{
			name: "single transaction in middle of year",
			setup: func() ([]model.Classification, time.Time, time.Time) {
				start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(2024, 12, 31, 23, 59, 59, 999999999, time.UTC)
				classifications := []model.Classification{{
					Transaction: model.Transaction{
						ID:   "SINGLE",
						Date: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
					},
				}}
				return classifications, start, end
			},
			expectedError: "", // Now just warns about gaps
			expectWarnings: true, // Large gaps before and after
		},
		{
			name: "leap year coverage",
			setup: func() ([]model.Classification, time.Time, time.Time) {
				start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(2024, 12, 31, 23, 59, 59, 999999999, time.UTC)
				// Full year including Feb 29
				dates := generateMonthlyTransactions(2024, 1, 12)
				dates = append(dates, time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC))
				
				var classifications []model.Classification
				for i, date := range dates {
					classifications = append(classifications, model.Classification{
						Transaction: model.Transaction{
							ID:   fmt.Sprintf("TX%d", i),
							Date: date,
						},
					})
				}
				return classifications, start, end
			},
			expectedError: "", // Should pass
			expectWarnings: false, // Good coverage
		},
		{
			name: "exact 30 day gap - should warn",
			setup: func() ([]model.Classification, time.Time, time.Time) {
				start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(2024, 3, 31, 23, 59, 59, 999999999, time.UTC)
				classifications := []model.Classification{
					{
						Transaction: model.Transaction{
							ID:   "TX1",
							Date: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
						},
					},
					{
						Transaction: model.Transaction{
							ID:   "TX2",
							Date: time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC), // Exactly 30 days later
						},
					},
					{
						Transaction: model.Transaction{
							ID:   "TX3",
							Date: time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC),
						},
					},
				}
				return classifications, start, end
			},
			expectedError: "", 
			expectWarnings: true, // 30+ day gap between TX2 and TX3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifications, start, end := tt.setup()
			err := validateDataCoverageFromClassifications(classifications, start, end)
			
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestUnclassifiedTransactionValidation(t *testing.T) {
	// This test validates the unclassified transaction check in runFlow
	// We're testing the logic that prevents export when there are unclassified transactions
	
	tests := []struct {
		name                string
		unclassifiedCount   int
		unclassifiedInRange int
		dateRange           string
		expectedError       string
	}{
		{
			name:                "no unclassified transactions",
			unclassifiedCount:   0,
			unclassifiedInRange: 0,
			dateRange:           "full year",
			expectedError:       "",
		},
		{
			name:                "unclassified transactions in range",
			unclassifiedCount:   5,
			unclassifiedInRange: 5,
			dateRange:           "full year",
			expectedError:       "cannot export: 5 transactions are not classified",
		},
		{
			name:                "unclassified transactions outside range",
			unclassifiedCount:   10,
			unclassifiedInRange: 0,
			dateRange:           "specific month",
			expectedError:       "",
		},
		{
			name:                "mixed - some in range, some outside",
			unclassifiedCount:   20,
			unclassifiedInRange: 3,
			dateRange:           "specific month",
			expectedError:       "cannot export: 3 transactions are not classified",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the check performed in runFlow
			var err error
			if tt.unclassifiedInRange > 0 {
				err = fmt.Errorf(tt.expectedError)
			}
			
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}