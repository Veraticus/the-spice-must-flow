package tui

import (
	"context"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/components"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
)

// State represents the current state of the TUI.
type State int

// TUI state constants.
const (
	// StateList shows transaction list.
	StateList State = iota
	// StateClassifying shows classification interface.
	StateClassifying
	// StateBatch shows batch classification.
	StateBatch
	// StateDirectionConfirm shows direction confirmation.
	StateDirectionConfirm
	// StateExporting shows export progress.
	StateExporting
	// StateHelp shows help screen.
	StateHelp
)

// View represents the current view mode.
type View int

// View mode constants.
const (
	// ViewTransactions shows transaction list.
	ViewTransactions View = iota
	// ViewMerchantGroups shows grouped by merchant.
	ViewMerchantGroups
	// ViewCalendar shows calendar view.
	ViewCalendar
	// ViewStats shows statistics.
	ViewStats
)

// Model holds the main TUI state.
type Model struct {
	startTime           time.Time
	llm                 engine.Classifier
	lastError           error
	storage             service.Storage
	resultChan          chan<- promptResult
	errorChan           chan<- error
	readyCallback       func()
	classifications     map[string]model.Classification
	pendingDirection    *engine.PendingDirection
	lastClassification  *model.Classification
	theme               themes.Theme
	notificationType    string
	notification        string
	keymap              KeyMap
	transactions        []model.Transaction
	pending             []model.PendingClassification
	checkPatterns       []model.CheckPattern
	categories          []model.Category
	config              Config
	transactionList     components.TransactionListModel
	directionView       components.DirectionConfirmModel
	batchView           components.BatchViewModel
	classifier          components.ClassifierModel
	statsPanel          components.StatsPanelModel
	sessionStats        service.CompletionStats
	width               int
	view                View
	height              int
	state               State
	currentPendingIndex int
	quitting            bool
	ready               bool
}

// newModel creates a new model with the given configuration.
func newModel(cfg Config) Model {
	m := Model{
		state:           StateList,
		view:            ViewTransactions,
		classifications: make(map[string]model.Classification),
		config:          cfg,
		keymap:          DefaultKeyMap(),
		theme:           cfg.Theme,
		storage:         cfg.Storage,
		llm:             cfg.Classifier,
		categories:      cfg.Categories,
		checkPatterns:   cfg.Patterns,
		startTime:       time.Now(),
		width:           cfg.Width,
		height:          cfg.Height,
	}

	// Initialize components
	if cfg.ShowStats {
		m.statsPanel = components.NewStatsPanelModel(cfg.Theme)
	}

	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnterAltScreen,
	}

	// In test mode, generate fake data
	if m.config.TestMode {
		cmds = append(cmds, m.generateTestData())
	} else if m.storage != nil {
		// Check what data needs to be loaded
		needTransactions := true
		needCategories := len(m.categories) == 0
		needPatterns := len(m.checkPatterns) == 0

		// Load only missing data
		if needTransactions {
			cmds = append(cmds, m.loadTransactions())
		}
		if needCategories {
			cmds = append(cmds, m.loadCategories())
		}
		if needPatterns {
			cmds = append(cmds, m.loadCheckPatterns())
		}

		// If we already have categories, we're ready
		if len(m.categories) > 0 {
			m.checkReady()
		}
	}

	return tea.Batch(cmds...)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear any existing notification on key press
		m.notification = ""
		m.notificationType = ""

		if cmd := m.handleGlobalKeys(msg); cmd != nil {
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.handleResize()

	case dataLoadedMsg:
		m.handleDataLoaded(msg)
		m.checkReady()

	case transactionsLoadedMsg:
		if msg.err != nil {
			m.lastError = msg.err
			return m, m.showError(msg.err)
		}
		m.transactions = msg.transactions
		m.transactionList = components.NewTransactionList(m.transactions, m.theme)
		if m.config.ShowStats {
			m.statsPanel.SetTotal(len(m.transactions))
		}
		m.checkReady()

	case categoriesLoadedMsg:
		if msg.err != nil {
			m.lastError = msg.err
			return m, m.showError(msg.err)
		}
		m.categories = msg.categories
		m.checkReady()

	case checkPatternsLoadedMsg:
		if msg.err != nil {
			m.lastError = msg.err
			return m, m.showError(msg.err)
		}
		m.checkPatterns = msg.patterns

	case classificationRequestMsg:
		m.pending = []model.PendingClassification{msg.pending}
		m.currentPendingIndex = 0
		m.state = StateClassifying
		m.startClassification(msg.pending)
		return m, m.focusOnTransaction(msg.pending.Transaction.ID)

	case batchClassificationRequestMsg:
		m.pending = msg.pending
		m.currentPendingIndex = 0
		m.state = StateBatch
		m.batchView = components.NewBatchViewModel(msg.pending, m.theme)
		return m, nil

	case directionRequestMsg:
		m.pendingDirection = &msg.pending
		m.state = StateDirectionConfirm
		m.directionView = components.NewDirectionConfirmModel(msg.pending, m.theme)
		return m, nil

	case errorMsg:
		m.lastError = msg.err
		cmds = append(cmds, m.showError(msg.err))

	case showMessageMsg:
		m.notification = msg.message
		m.notificationType = "info"

	case notificationMsg:
		m.notification = msg.content
		m.notificationType = msg.messageType
	}

	// Delegate to active component based on state
	switch m.state {
	case StateList:
		newList, cmd := m.transactionList.Update(msg)
		m.transactionList = newList
		cmds = append(cmds, cmd)

		// Handle transaction selection
		if msg, ok := msg.(components.TransactionSelectedMsg); ok {
			cmds = append(cmds, m.handleTransactionSelection(msg))
		}

	case StateClassifying:
		newClassifier, cmd := m.classifier.Update(msg)
		m.classifier = newClassifier
		cmds = append(cmds, cmd)

		// Check if classification is complete
		if newClassifier.IsComplete() {
			result := newClassifier.GetResult()
			// Track the classification for undo
			m.lastClassification = &result
			m.classifications[result.Transaction.ID] = result
			m.resultChan <- promptResult{
				classification: result,
			}

			// Move to next pending or back to list
			if m.currentPendingIndex < len(m.pending)-1 {
				m.currentPendingIndex++
				m.startClassification(m.pending[m.currentPendingIndex])
			} else {
				m.state = StateList
			}
		}

	case StateBatch:
		newBatch, cmd := m.batchView.Update(msg)
		m.batchView = newBatch
		cmds = append(cmds, cmd)

		// Handle batch results
		if results := newBatch.GetResults(); len(results) > 0 {
			for _, result := range results {
				// Track the last classification for undo
				m.lastClassification = &result
				m.classifications[result.Transaction.ID] = result
				m.resultChan <- promptResult{
					classification: result,
				}
			}
			m.state = StateList
		}

	case StateDirectionConfirm:
		newDirection, cmd := m.directionView.Update(msg)
		m.directionView = newDirection
		cmds = append(cmds, cmd)

		// Check if direction confirmation is complete
		if newDirection.IsComplete() {
			m.resultChan <- promptResult{
				direction: newDirection.GetResult(),
			}
			m.state = StateList
			m.pendingDirection = nil
		}
	}

	// Update stats panel
	if m.config.ShowStats {
		newStats, cmd := m.statsPanel.Update(msg)
		m.statsPanel = newStats
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m Model) View() string {
	if !m.ready {
		return m.renderLoading()
	}

	if m.quitting {
		return ""
	}

	// Responsive layout based on terminal size
	if m.width < 80 {
		return m.renderCompactView()
	}

	if m.width < 120 {
		return m.renderMediumView()
	}

	return m.renderFullView()
}

// handleGlobalKeys handles keys that work in any state.
func (m Model) handleGlobalKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.state != StateClassifying && m.state != StateBatch {
			m.quitting = true
			return tea.Quit
		}
	case "?":
		if m.state == StateHelp {
			m.state = StateList
		} else {
			m.state = StateHelp
		}
		return nil
	case "ctrl+l":
		return tea.ClearScreen
	case "u", "ctrl+z":
		if m.state == StateList && m.lastClassification != nil {
			return m.undoLastClassification()
		}
	}
	return nil
}

// handleResize adjusts component sizes when terminal resizes.
func (m *Model) handleResize() {
	// Update component sizes
	if m.width > 120 {
		// Full layout
		m.transactionList.Resize(m.width/2, m.height-4)
		m.classifier.Resize(m.width/2, m.height-4)
		m.statsPanel.Resize(m.width/4, m.height-4)
	} else {
		// Compact layout
		m.transactionList.Resize(m.width-2, m.height-6)
		m.classifier.Resize(m.width-2, m.height-6)
		m.statsPanel.Resize(m.width-2, 4)
	}
}

// checkReady checks if all required data is loaded and calls the ready callback.
func (m *Model) checkReady() {
	// We're ready when categories are loaded (transactions might be empty)
	if !m.ready && len(m.categories) > 0 {
		m.ready = true
		if m.readyCallback != nil {
			go m.readyCallback()
		}
	}
}

// startClassification initializes classification for a pending transaction.
func (m *Model) startClassification(pending model.PendingClassification) {
	m.classifier = components.NewClassifierModel(
		pending,
		m.categories,
		m.theme,
		m.llm,
	)

	// Update stats
	m.sessionStats.TotalTransactions++
}

// focusOnTransaction scrolls to and highlights a specific transaction.
func (m Model) focusOnTransaction(id string) tea.Cmd {
	return func() tea.Msg {
		for i, txn := range m.transactions {
			if txn.ID == id {
				return components.FocusTransactionMsg{Index: i}
			}
		}
		return nil
	}
}

// State returns the current state of the TUI.
func (m Model) State() State {
	return m.state
}

// GetView returns the current view of the TUI.
func (m Model) GetView() View {
	return m.view
}

// undoLastClassification removes the last classification.
func (m *Model) undoLastClassification() tea.Cmd {
	if m.lastClassification == nil {
		return nil
	}

	// Save the classification details before clearing
	txnID := m.lastClassification.Transaction.ID

	// Try to delete from storage if it supports it
	ctx := context.Background()
	if deleter, ok := m.storage.(interface {
		DeleteClassification(context.Context, string) error
	}); ok {
		if err := deleter.DeleteClassification(ctx, txnID); err != nil {
			m.lastError = err
		}
	}

	// Remove from local state
	delete(m.classifications, txnID)

	// Update stats
	if m.sessionStats.TotalTransactions > 0 {
		m.sessionStats.TotalTransactions--
	}
	if m.sessionStats.UserClassified > 0 {
		m.sessionStats.UserClassified--
	}

	// Update the transaction in the list to show it's unclassified
	for i, txn := range m.transactions {
		if txn.ID == txnID {
			m.transactions[i].Category = nil
			break
		}
	}

	// Clear last classification
	m.lastClassification = nil

	// Refresh the transaction list
	m.transactionList = components.NewTransactionList(m.transactions, m.theme)

	// Show undo notification
	return func() tea.Msg {
		return notificationMsg{
			content:     "Undo",
			messageType: "info",
		}
	}
}
