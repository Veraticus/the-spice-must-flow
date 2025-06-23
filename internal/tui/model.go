package tui

import (
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

const (
	StateList State = iota
	StateClassifying
	StateBatch
	StateDirectionConfirm
	StateExporting
	StateHelp
)

// View represents the current view mode.
type View int

const (
	ViewTransactions View = iota
	ViewMerchantGroups
	ViewCalendar
	ViewStats
)

// Model holds the main TUI state.
type Model struct {
	theme               themes.Theme
	startTime           time.Time
	llm                 engine.Classifier
	lastError           error
	storage             service.Storage
	classifications     map[string]model.Classification
	errorChan           chan<- error
	resultChan          chan<- promptResult
	pendingDirection    *engine.PendingDirection
	transactionList     components.TransactionListModel
	directionView       components.DirectionConfirmModel
	batchView           components.BatchViewModel
	config              Config
	keymap              KeyMap
	categories          []model.Category
	pending             []model.PendingClassification
	checkPatterns       []model.CheckPattern
	transactions        []model.Transaction
	classifier          components.ClassifierModel
	statsPanel          components.StatsPanelModel
	sessionStats        service.CompletionStats
	height              int
	width               int
	currentPendingIndex int
	state               State
	view                View
	quitting            bool
	ready               bool
}

// newModel creates a new model with the given configuration.
func newModel(cfg Config) Model {
	return Model{
		state:           StateList,
		view:            ViewTransactions,
		classifications: make(map[string]model.Classification),
		config:          cfg,
		keymap:          DefaultKeyMap(),
		theme:           cfg.Theme,
		storage:         cfg.Storage,
		llm:             cfg.Classifier,
		startTime:       time.Now(),
		width:           cfg.Width,
		height:          cfg.Height,
	}
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
		// Load real data
		cmds = append(cmds, m.loadTransactions())
		cmds = append(cmds, m.loadCategories())
		cmds = append(cmds, m.loadCheckPatterns())
	}

	return tea.Batch(cmds...)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cmd := m.handleGlobalKeys(msg); cmd != nil {
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.handleResize()

	case dataLoadedMsg:
		m.handleDataLoaded(msg)
		m.ready = true

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
