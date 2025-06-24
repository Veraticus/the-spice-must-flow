package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// Harness provides a complete TUI testing environment.
type Harness struct {
	testData       TestData
	lastRenderTime time.Time
	ctx            context.Context
	model          tea.Model
	classifier     *MockClassifier
	cancel         context.CancelFunc
	output         *bytes.Buffer
	storage        *MockStorage
	hooks          map[HookType]HookFunc
	inputChan      chan tea.Msg
	outputChan     chan string
	stateChan      chan tui.State
	viewChan       chan tui.View
	errorChan      chan error
	t              *testing.T
	program        *tea.Program
	keySequence    []string
	screenHistory  []string
	config         tui.Config
	renderCount    int
	width          int
	height         int
	mu             sync.RWMutex
	started        bool
	shutdown       bool
	closed         bool
}

// HookType represents different points where hooks can be injected.
type HookType string

const (
	// HookBeforeUpdate is triggered before an update cycle.
	HookBeforeUpdate HookType = "before_update"
	// HookAfterUpdate is triggered after an update cycle.
	HookAfterUpdate HookType = "after_update"
	// HookOnRender is triggered when the view is rendered.
	HookOnRender HookType = "on_render"
	// HookOnError is triggered when an error occurs.
	HookOnError HookType = "on_error"
)

// HookFunc is a function that can be injected at various points.
type HookFunc func(h *Harness, msg tea.Msg)

// PausePoint allows pausing test execution at specific points.
type PausePoint struct {
	Callback    func(*Harness)
	AfterKeys   int
	AfterRender int
	AtState     tui.State
	Duration    time.Duration
}

// TestData holds test fixtures for the harness.
type TestData struct {
	Classifications map[string]model.Classification
	Transactions    []model.Transaction
	Categories      []model.Category
	CheckPatterns   []model.CheckPattern
}

// NewHarness creates a new TUI test harness.
func NewHarness(t *testing.T, opts ...HarnessOption) *Harness {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	h := &Harness{
		t:          t,
		ctx:        ctx,
		cancel:     cancel,
		output:     new(bytes.Buffer),
		inputChan:  make(chan tea.Msg, 100),
		outputChan: make(chan string, 100),
		stateChan:  make(chan tui.State, 100),
		viewChan:   make(chan tui.View, 100),
		errorChan:  make(chan error, 10),
		hooks:      make(map[HookType]HookFunc),
		width:      80,
		height:     24,
		testData:   generateDefaultTestData(),
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	// Create mock storage
	h.storage = NewMockStorage()
	h.storage.SetTransactions(h.testData.Transactions)
	h.storage.SetCategories(h.testData.Categories)
	h.storage.SetCheckPatterns(h.testData.CheckPatterns)

	// Create mock classifier
	h.classifier = NewMockClassifier()

	// Build config - default to standalone mode for interactive testing
	h.config = tui.Config{
		Theme:                  themes.Default,
		Storage:                h.storage,
		Classifier:             h.classifier,
		Width:                  h.width,
		Height:                 h.height,
		ShowStats:              true,
		ShowPreview:            true,
		EnableVirtualScrolling: true,
		EnableCaching:          false, // Disable for predictable tests
		EnableAnimations:       false, // Disable for faster tests
		MouseSupport:           false, // Disable for simpler tests
		TestMode:               false, // Use real data from mocks
	}

	return h
}

// Start initializes and runs the TUI in test mode.
func (h *Harness) Start() error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return fmt.Errorf("harness already started")
	}
	h.started = true
	h.mu.Unlock()

	// Create test model with instrumentation
	baseModel := h.createTestModel()
	instrumentedModel := h.instrumentModel(baseModel)
	h.model = instrumentedModel

	// Create program with test configuration
	h.program = tea.NewProgram(
		instrumentedModel,
		tea.WithContext(h.ctx),
		tea.WithOutput(h.output),
		tea.WithInput(h),
		tea.WithoutRenderer(), // We'll render manually for testing
	)

	// Run program in background
	go func() {
		if _, err := h.program.Run(); err != nil {
			// Check if we're shutting down before sending
			h.mu.RLock()
			shutdown := h.shutdown
			h.mu.RUnlock()

			if !shutdown {
				select {
				case h.errorChan <- err:
				default:
				}
			}
		}
	}()

	// Send initial window size to trigger rendering
	h.program.Send(tea.WindowSizeMsg{
		Width:  h.width,
		Height: h.height,
	})

	// Wait for initial render
	return h.waitForReady()
}

// SendKeys sends a sequence of key presses to the TUI.
func (h *Harness) SendKeys(keys ...string) {
	h.mu.Lock()
	h.keySequence = append(h.keySequence, keys...)
	h.mu.Unlock()

	for _, key := range keys {
		h.sendKey(key)
		h.waitForRender()
	}
}

// SendKey sends a single key press.
func (h *Harness) sendKey(key string) {
	msg := tea.KeyMsg{
		Type: tea.KeyRunes,
	}

	// Handle special keys
	switch key {
	case "enter":
		msg.Type = tea.KeyEnter
	case "tab":
		msg.Type = tea.KeyTab
	case "backspace":
		msg.Type = tea.KeyBackspace
	case "delete":
		msg.Type = tea.KeyDelete
	case "up":
		msg.Type = tea.KeyUp
	case "down":
		msg.Type = tea.KeyDown
	case "left":
		msg.Type = tea.KeyLeft
	case "right":
		msg.Type = tea.KeyRight
	case "home":
		msg.Type = tea.KeyHome
	case "end":
		msg.Type = tea.KeyEnd
	case "pgup":
		msg.Type = tea.KeyPgUp
	case "pgdown":
		msg.Type = tea.KeyPgDown
	case "esc":
		msg.Type = tea.KeyEsc
	default:
		switch {
		case len(key) == 1:
			msg.Runes = []rune(key)
		case len(key) > 1 && key[0] == '^':
			// Handle ctrl keys
			msg.Type = tea.KeyCtrlA + tea.KeyType(key[1]-'a')
		default:
			msg.Runes = []rune(key)
		}
	}

	// Send directly to the program instead of using channel
	h.program.Send(msg)
}

// WaitForState waits for the TUI to reach a specific state.
func (h *Harness) WaitForState(state tui.State, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case currentState := <-h.stateChan:
			if currentState == state {
				return nil
			}
			// Re-enqueue if not the expected state
			h.stateChan <- currentState
		case <-timer.C:
			return fmt.Errorf("timeout waiting for state %v", state)
		case <-h.ctx.Done():
			return h.ctx.Err()
		}
	}
}

// GetCurrentScreen returns the current rendered screen.
func (h *Harness) GetCurrentScreen() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.screenHistory) == 0 {
		return ""
	}
	return h.screenHistory[len(h.screenHistory)-1]
}

// AssertScreen verifies the current screen matches expected content.
func (h *Harness) AssertScreen(t *testing.T, expectedContent ...string) {
	t.Helper()

	screen := h.GetCurrentScreen()
	for _, content := range expectedContent {
		require.Contains(t, screen, content, "screen should contain expected content")
	}
}

// AssertNotScreen verifies the current screen does not contain content.
func (h *Harness) AssertNotScreen(t *testing.T, unexpectedContent ...string) {
	t.Helper()

	screen := h.GetCurrentScreen()
	for _, content := range unexpectedContent {
		require.NotContains(t, screen, content, "screen should not contain unexpected content")
	}
}

// LoadTransactions loads test transactions into the TUI.
func (h *Harness) LoadTransactions(transactions []model.Transaction) {
	h.mu.Lock()
	h.testData.Transactions = transactions
	h.mu.Unlock()

	h.storage.SetTransactions(transactions)

	// Send a message to reload transactions
	h.program.Send(tea.Msg(struct{ reload bool }{reload: true}))
	h.waitForRender()
}

// SelectTransaction selects a transaction by ID.
func (h *Harness) SelectTransaction(txnID string) error {
	// Find transaction index
	var index int
	found := false

	h.mu.RLock()
	for i, txn := range h.testData.Transactions {
		if txn.ID == txnID {
			index = i
			found = true
			break
		}
	}
	h.mu.RUnlock()

	if !found {
		return fmt.Errorf("transaction %s not found", txnID)
	}

	// Navigate to transaction
	// First go to top
	h.SendKeys("g")

	// Then navigate down to the transaction
	for i := 0; i < index; i++ {
		h.SendKeys("j")
	}

	// Don't press enter - let the workflow control that
	return nil
}

// GetStorage returns the mock storage for test verification.
func (h *Harness) GetStorage() *MockStorage {
	return h.storage
}

// GetClassifier returns the mock classifier for test setup.
func (h *Harness) GetClassifier() *MockClassifier {
	return h.classifier
}

// SetAISuggestions configures AI suggestions for a specific transaction.
func (h *Harness) SetAISuggestions(txnID string, rankings model.CategoryRankings) {
	h.classifier.SetRankings(txnID, rankings)
}

// SetDefaultAISuggestions configures default AI suggestions for all transactions.
func (h *Harness) SetDefaultAISuggestions(rankings model.CategoryRankings) {
	h.classifier.SetDefaultRankings(rankings)
}

// AssertBatchModeActive is a convenience method for batch mode assertions.
func (h *Harness) AssertBatchModeActive(t *testing.T, expectedCount int) {
	t.Helper()
	assertions := NewAssertions(h)
	assertions.AssertBatchModeActive(t, expectedCount)
}

// GetProgram returns the underlying tea.Program for advanced usage.
func (h *Harness) GetProgram() *tea.Program {
	return h.program
}

// GetTestData returns the test data for debugging.
func (h *Harness) GetTestData() TestData {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.testData
}

// Cleanup shuts down the harness and cleans up resources.
func (h *Harness) Cleanup() {
	h.mu.Lock()
	if h.shutdown {
		h.mu.Unlock()
		return
	}
	h.shutdown = true
	h.closed = true
	h.mu.Unlock()

	if h.program != nil {
		h.program.Quit()
	}

	h.cancel()

	// Close channels
	close(h.inputChan)
	close(h.outputChan)
	close(h.stateChan)
	close(h.viewChan)
	close(h.errorChan)
}

// Read implements io.Reader for tea.Program input.
func (h *Harness) Read(_ []byte) (n int, err error) {
	// Since we're using program.Send() directly for input,
	// this can just block to prevent the program from reading stdin
	<-h.ctx.Done()
	return 0, io.EOF
}

// Private helper methods

func (h *Harness) createTestModel() tea.Model {
	// Apply all options based on our config
	var opts []tui.Option

	opts = append(opts,
		tui.WithStorage(h.config.Storage),
		tui.WithClassifier(h.config.Classifier),
		tui.WithTheme(h.config.Theme),
		tui.WithSize(h.config.Width, h.config.Height),
		tui.WithFeatures(
			h.config.EnableVirtualScrolling,
			h.config.EnableCaching,
			h.config.EnableAnimations,
			h.config.MouseSupport,
		),
		tui.WithTestMode(h.config.TestMode),
	)

	// PrompterMode removed - TUI now runs in standalone mode

	// Create the prompter
	prompter, err := tui.New(h.ctx, opts...)
	if err != nil {
		h.t.Fatalf("failed to create TUI prompter: %v", err)
	}

	// Type assert to get the concrete prompter
	tuiPrompter, ok := prompter.(*tui.Prompter)
	if !ok {
		h.t.Fatal("unexpected prompter type")
	}

	// Get the underlying model
	model := tuiPrompter.Model()

	return &testModel{
		harness:  h,
		config:   h.config,
		prompter: tuiPrompter,
		model:    model,
	}
}

func (h *Harness) instrumentModel(base tea.Model) tea.Model {
	return &instrumentedModel{
		base:    base,
		harness: h,
	}
}

func (h *Harness) waitForReady() error {
	// Wait for model to be ready (not just first render)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			lastScreen := h.GetCurrentScreen()
			h.mu.RLock()
			txnCount := len(h.testData.Transactions)
			h.mu.RUnlock()
			return fmt.Errorf("timeout waiting for TUI to be ready (expected %d transactions), last screen: %s", txnCount, lastScreen)
		case <-h.ctx.Done():
			return h.ctx.Err()
		default:
			screen := h.GetCurrentScreen()

			// Check if we've rendered and the loading screen is gone
			if screen != "" && !strings.Contains(screen, "Loading The Spice Must Flow") {
				// Also check for a valid UI element to ensure it's really ready
				if strings.Contains(screen, "Transactions") || strings.Contains(screen, "Navigate") {
					// Additional check: verify we have the expected number of transactions displayed
					h.mu.RLock()
					expectedTxns := len(h.testData.Transactions)
					h.mu.RUnlock()

					// If we expect transactions, ensure at least one is displayed
					if expectedTxns > 0 {
						// Check if any transaction is visible by looking for common transaction elements
						hasTransactions := false
						for _, txn := range h.testData.Transactions {
							if strings.Contains(screen, txn.MerchantName) {
								hasTransactions = true
								break
							}
						}
						if hasTransactions {
							return nil
						}
						// If no transactions visible yet, keep waiting
					} else {
						// No transactions expected, we're ready
						return nil
					}
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (h *Harness) waitForRender() {
	// Wait for next render with a short timeout
	start := time.Now()
	h.mu.RLock()
	lastCount := h.renderCount
	h.mu.RUnlock()

	for time.Since(start) < 100*time.Millisecond {
		h.mu.RLock()
		currentCount := h.renderCount
		h.mu.RUnlock()

		if currentCount > lastCount {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// HarnessOption configures the test harness.
type HarnessOption func(*Harness)

// WithSize sets the terminal size for testing.
func WithSize(width, height int) HarnessOption {
	return func(h *Harness) {
		h.width = width
		h.height = height
	}
}

// WithTestData sets custom test data.
func WithTestData(data TestData) HarnessOption {
	return func(h *Harness) {
		h.testData = data
	}
}

// WithHook adds a hook function.
func WithHook(hookType HookType, fn HookFunc) HarnessOption {
	return func(h *Harness) {
		h.hooks[hookType] = fn
	}
}

// // WithPrompterMode removed - TUI now runs in standalone mode
// // func  HarnessOption {
// 	return func(h *Harness) {
// 		// PrompterMode removed
// 	}
// }

// generateDefaultTestData creates a standard set of test data.
func generateDefaultTestData() TestData {
	return TestData{
		Transactions: []model.Transaction{
			{
				ID:           "txn1",
				AccountID:    "acc1",
				MerchantName: "Whole Foods",
				Amount:       -123.45,
				Date:         time.Now().Add(-24 * time.Hour),
				Type:         "PURCHASE",
			},
			{
				ID:           "txn2",
				AccountID:    "acc1",
				MerchantName: "Netflix",
				Amount:       -15.99,
				Date:         time.Now().Add(-48 * time.Hour),
				Type:         "SUBSCRIPTION",
			},
		},
		Categories: []model.Category{
			{ID: 1, Name: "Groceries"},
			{ID: 2, Name: "Entertainment"},
			{ID: 3, Name: "Transportation"},
		},
		CheckPatterns:   []model.CheckPattern{},
		Classifications: make(map[string]model.Classification),
	}
}
