package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// MockPrompter is a test implementation of the Prompter interface.
// It auto-accepts suggestions for testing purposes.
type MockPrompter struct {
	startTime            time.Time
	customResponses      map[string]string
	rejectMerchants      map[string]bool
	confirmCalls         []MockConfirmCall
	batchConfirmCalls    []MockBatchConfirmCall
	batchClassifications []model.Classification // For preset batch responses
	totalTransactions    int
	autoClassified       int
	userClassified       int
	newVendorRules       int
	mu                   sync.Mutex
	autoAccept           bool
}

// MockConfirmCall records details of a single confirmation request.
type MockConfirmCall struct {
	Error          error
	Classification model.Classification
	Pending        model.PendingClassification
}

// MockBatchConfirmCall records details of a batch confirmation request.
type MockBatchConfirmCall struct {
	Error           error
	Pending         []model.PendingClassification
	Classifications []model.Classification
}

// NewMockPrompter creates a new mock user prompter.
func NewMockPrompter(autoAccept bool) *MockPrompter {
	return &MockPrompter{
		confirmCalls:      make([]MockConfirmCall, 0),
		batchConfirmCalls: make([]MockBatchConfirmCall, 0),
		autoAccept:        autoAccept,
		customResponses:   make(map[string]string),
		rejectMerchants:   make(map[string]bool),
		startTime:         time.Now(),
	}
}

// ConfirmClassification confirms a single classification.
func (m *MockPrompter) ConfirmClassification(_ context.Context, pending model.PendingClassification) (model.Classification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	merchantName := pending.Transaction.MerchantName
	if merchantName == "" {
		merchantName = pending.Transaction.Name
	}

	// Check if we should reject this merchant
	if m.rejectMerchants[merchantName] {
		err := fmt.Errorf("user rejected classification for %s", merchantName)
		call := MockConfirmCall{
			Pending: pending,
			Error:   err,
		}
		m.confirmCalls = append(m.confirmCalls, call)
		return model.Classification{}, err
	}

	// Determine the category to use
	var category string
	var status model.ClassificationStatus

	if customCategory, exists := m.customResponses[merchantName]; exists {
		// User provided custom category
		category = customCategory
		status = model.StatusUserModified
		m.userClassified++
	} else if m.autoAccept {
		// Auto-accept the suggestion
		category = pending.SuggestedCategory
		status = model.StatusClassifiedByAI
		m.autoClassified++
	} else {
		// Default behavior: accept but mark as user modified
		category = pending.SuggestedCategory
		status = model.StatusUserModified
		m.userClassified++
	}

	classification := model.Classification{
		Transaction:  pending.Transaction,
		Category:     category,
		Status:       status,
		Confidence:   pending.Confidence,
		ClassifiedAt: time.Now(),
	}

	// Record the call
	call := MockConfirmCall{
		Pending:        pending,
		Classification: classification,
		Error:          nil,
	}
	m.confirmCalls = append(m.confirmCalls, call)
	m.totalTransactions++

	return classification, nil
}

// BatchConfirmClassifications confirms multiple classifications at once.
func (m *MockPrompter) BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(pending) == 0 {
		return []model.Classification{}, nil
	}

	// If preset batch classifications are available, use them
	if len(m.batchClassifications) > 0 {
		// Ensure we're returning classifications for the right transactions
		result := make([]model.Classification, 0, len(m.batchClassifications))
		for _, c := range m.batchClassifications {
			// Update transaction from pending to ensure we have the latest data
			for _, p := range pending {
				if strings.Contains(p.Transaction.ID, c.Transaction.ID) ||
					p.Transaction.MerchantName == c.Transaction.MerchantName {
					c.Transaction = p.Transaction
					break
				}
			}
			result = append(result, c)
		}

		// Record the call
		call := MockBatchConfirmCall{
			Pending:         pending,
			Classifications: result,
			Error:           nil,
		}
		m.batchConfirmCalls = append(m.batchConfirmCalls, call)
		m.totalTransactions += len(result)

		// Clear the preset responses after use
		m.batchClassifications = nil

		return result, nil
	}

	// For testing high-variance detection, check if this is an Amazon batch
	merchantName := pending[0].Transaction.MerchantName
	if merchantName == "" {
		merchantName = pending[0].Transaction.Name
	}

	// If this is Amazon and we have multiple transactions with high variance,
	// simulate individual review
	if strings.Contains(strings.ToLower(merchantName), "amazon") && len(pending) > 5 {
		hasHighVariance := false
		var minAmount, maxAmount float64
		for i, p := range pending {
			if i == 0 {
				minAmount = p.Transaction.Amount
				maxAmount = p.Transaction.Amount
			} else {
				if p.Transaction.Amount < minAmount {
					minAmount = p.Transaction.Amount
				}
				if p.Transaction.Amount > maxAmount {
					maxAmount = p.Transaction.Amount
				}
			}
		}
		if maxAmount > minAmount*10 {
			hasHighVariance = true
		}

		if hasHighVariance {
			// Process individually
			classifications := make([]model.Classification, 0, len(pending))
			for _, p := range pending {
				classification, err := m.ConfirmClassification(ctx, p)
				if err != nil {
					continue // Skip rejected ones
				}
				classifications = append(classifications, classification)
			}

			call := MockBatchConfirmCall{
				Pending:         pending,
				Classifications: classifications,
				Error:           nil,
			}
			m.batchConfirmCalls = append(m.batchConfirmCalls, call)
			// Don't increment newVendorRules for high variance merchants
			return classifications, nil
		}
	}

	// Check if we should reject this merchant
	if m.rejectMerchants[merchantName] {
		err := fmt.Errorf("user rejected batch classification for %s", merchantName)
		call := MockBatchConfirmCall{
			Pending: pending,
			Error:   err,
		}
		m.batchConfirmCalls = append(m.batchConfirmCalls, call)
		return nil, err
	}

	// Process as a batch
	classifications := make([]model.Classification, len(pending))

	// Determine the category to use
	var category string
	var status model.ClassificationStatus

	if customCategory, exists := m.customResponses[merchantName]; exists {
		// User provided custom category
		category = customCategory
		status = model.StatusUserModified
		m.userClassified += len(pending)
		m.newVendorRules++
	} else if m.autoAccept {
		// Auto-accept the suggestion
		category = pending[0].SuggestedCategory
		status = model.StatusClassifiedByAI
		m.autoClassified += len(pending)
		m.newVendorRules++
	} else {
		// Default behavior: accept but mark as user modified
		category = pending[0].SuggestedCategory
		status = model.StatusUserModified
		m.userClassified += len(pending)
		m.newVendorRules++
	}

	for i, p := range pending {
		classifications[i] = model.Classification{
			Transaction:  p.Transaction,
			Category:     category,
			Status:       status,
			Confidence:   p.Confidence,
			ClassifiedAt: time.Now(),
		}
	}

	// Record the call
	call := MockBatchConfirmCall{
		Pending:         pending,
		Classifications: classifications,
		Error:           nil,
	}
	m.batchConfirmCalls = append(m.batchConfirmCalls, call)
	m.totalTransactions += len(classifications)

	return classifications, nil
}

// GetCompletionStats returns statistics about the classification run.
func (m *MockPrompter) GetCompletionStats() service.CompletionStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	return service.CompletionStats{
		TotalTransactions: m.totalTransactions,
		AutoClassified:    m.autoClassified,
		UserClassified:    m.userClassified,
		NewVendorRules:    m.newVendorRules,
		Duration:          time.Since(m.startTime),
	}
}

// SetCustomResponse sets a custom category response for a specific merchant.
func (m *MockPrompter) SetCustomResponse(merchant, category string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customResponses[merchant] = category
}

// SetRejectMerchant marks a merchant to be rejected during confirmation.
func (m *MockPrompter) SetRejectMerchant(merchant string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rejectMerchants[merchant] = true
}

// GetConfirmCalls returns all single confirmation calls for verification.
func (m *MockPrompter) GetConfirmCalls() []MockConfirmCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]MockConfirmCall, len(m.confirmCalls))
	copy(calls, m.confirmCalls)
	return calls
}

// GetBatchConfirmCalls returns all batch confirmation calls for verification.
func (m *MockPrompter) GetBatchConfirmCalls() []MockBatchConfirmCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]MockBatchConfirmCall, len(m.batchConfirmCalls))
	copy(calls, m.batchConfirmCalls)
	return calls
}

// ConfirmCallCount returns the number of single confirmation calls.
func (m *MockPrompter) ConfirmCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.confirmCalls)
}

// BatchConfirmCallCount returns the number of batch confirmation calls.
func (m *MockPrompter) BatchConfirmCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batchConfirmCalls)
}

// Reset clears all recorded calls and statistics.
func (m *MockPrompter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.confirmCalls = make([]MockConfirmCall, 0)
	m.batchConfirmCalls = make([]MockBatchConfirmCall, 0)
	m.batchClassifications = nil
	m.totalTransactions = 0
	m.autoClassified = 0
	m.userClassified = 0
	m.newVendorRules = 0
	m.startTime = time.Now()
}

// SetBatchResponse sets preset classifications for batch confirmation calls.
func (m *MockPrompter) SetBatchResponse(classifications []model.Classification) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchClassifications = classifications
}
