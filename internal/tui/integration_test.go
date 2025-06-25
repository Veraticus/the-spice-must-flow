package tui

import (
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/components"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveClassificationFlow(t *testing.T) {
	// Create model with test config
	cfg := Config{
		Theme:  themes.Default,
		Width:  80,
		Height: 24,
	}
	m := newModel(cfg)

	// Set up some test transactions
	txns := []model.Transaction{
		{
			ID:           "txn1",
			MerchantName: "Test Store",
			Amount:       50.00,
			Type:         "DEBIT",
		},
		{
			ID:           "txn2",
			MerchantName: "Another Store",
			Amount:       75.00,
			Type:         "DEBIT",
			Category:     []string{"Shopping"},
		},
	}
	m.transactions = txns
	m.transactionList = components.NewTransactionList(txns, m.theme)

	// Test 1: Initial state should be StateList
	assert.Equal(t, StateList, m.state)

	// Test 2: Enter should show transaction details, not start classification
	msg := components.TransactionSelectedMsg{
		Transaction: txns[0],
		Index:       0,
	}
	cmd := m.handleTransactionSelection(msg)
	require.NotNil(t, cmd)

	// Execute the command to get the message
	resultMsg := cmd()
	detailsMsg, ok := resultMsg.(components.TransactionDetailsRequestMsg)
	require.True(t, ok)
	assert.Equal(t, txns[0].ID, detailsMsg.Transaction.ID)

	// Update the model with the details message
	newModel, _ := m.Update(detailsMsg)
	if model, valid := newModel.(Model); valid {
		m = model
	}

	// Test 3: State should now be StateDetails
	assert.Equal(t, StateDetails, m.state)

	// Test 4: Pressing 'S' in list view should start engine classification
	m.state = StateList
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}
	newModel, cmd = m.Update(keyMsg)
	if model, valid := newModel.(Model); valid {
		m = model
	}

	require.NotNil(t, cmd)
	resultMsg = cmd()
	_, ok = resultMsg.(components.StartEngineClassificationMsg)
	assert.True(t, ok)

	// Test 5: Back to list from details
	m.state = StateDetails
	backMsg := components.BackToListMsg{}
	newModel, _ = m.Update(backMsg)
	if model, valid := newModel.(Model); valid {
		m = model
	}
	assert.Equal(t, StateList, m.state)

	// Test 6: AI classification request from details
	m.state = StateDetails
	aiMsg := components.AIClassificationRequestMsg{
		Transaction: txns[0],
	}
	newModel, _ = m.Update(aiMsg)
	if model, valid := newModel.(Model); valid {
		m = model
	}
	assert.Equal(t, StateClassifying, m.state)
}

func TestStartEngineClassification(t *testing.T) {
	cfg := Config{
		Theme:  themes.Default,
		Width:  80,
		Height: 24,
	}
	m := newModel(cfg)

	// Set up transactions - one classified, one not
	m.transactions = []model.Transaction{
		{
			ID:           "txn1",
			MerchantName: "Unclassified Store",
			Amount:       50.00,
			Category:     nil, // Unclassified
		},
		{
			ID:           "txn2",
			MerchantName: "Classified Store",
			Amount:       75.00,
			Category:     []string{"Shopping"}, // Already classified
		},
	}

	// Start engine classification
	cmd := m.startEngineClassification()
	require.NotNil(t, cmd)

	resultMsg := cmd()
	batchMsg, ok := resultMsg.(batchClassificationRequestMsg)
	require.True(t, ok)

	// Should only include unclassified transactions
	assert.Len(t, batchMsg.pending, 1)
	assert.Equal(t, "txn1", batchMsg.pending[0].Transaction.ID)
}

func TestNoUnclassifiedTransactions(t *testing.T) {
	cfg := Config{
		Theme:  themes.Default,
		Width:  80,
		Height: 24,
	}
	m := newModel(cfg)

	// All transactions are classified
	m.transactions = []model.Transaction{
		{
			ID:       "txn1",
			Category: []string{"Shopping"},
		},
		{
			ID:       "txn2",
			Category: []string{"Food"},
		},
	}

	// Start engine classification
	cmd := m.startEngineClassification()
	require.NotNil(t, cmd)

	resultMsg := cmd()
	notifMsg, ok := resultMsg.(notificationMsg)
	require.True(t, ok)
	assert.Equal(t, "All transactions are already classified", notifMsg.content)
	assert.Equal(t, "info", notifMsg.messageType)
}
