# Terminal User Interface (TUI) Design Document

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Architecture Overview](#architecture-overview)
3. [Component Design](#component-design)
4. [User Experience Flows](#user-experience-flows)
5. [Testing Strategy](#testing-strategy)
6. [Integration Guide](#integration-guide)
7. [Performance Optimizations](#performance-optimizations)
8. [Development Workflow](#development-workflow)

## Executive Summary

This document describes the design for a sophisticated Terminal User Interface (TUI) that enhances the transaction classification experience while maintaining backward compatibility with the existing CLI interface.

### Key Design Principles

- **Progressive Enhancement**: The TUI is a drop-in replacement for the CLI prompter
- **Interface Segregation**: Small, focused interfaces following Go idioms
- **Test-First Development**: Comprehensive testing at every level
- **Performance First**: Handles thousands of transactions smoothly
- **Developer Experience**: Easy to iterate and test with fake data

### Visual Preview

```
┌─ Transactions ────────────────────────┬─ Details ─────────────────────┬─ Categories ──────────┐
│ 📁 Whole Foods Market (12)       $823 │ WHOLE FOODS MARKET #4521      │ 🥬 Groceries      92% │
│   ├─ Jan 15  #4521           $67.23  │ January 15, 2024              │ 🍕 Dining Out     45% │
│   ├─ Jan 08  #4521           $120.45 │                               │ 🏠 Home Supplies  23% │
│   └─ Jan 02  #4521           $89.00  │ Amount: $67.23                │ 💊 Healthcare     12% │
│                                       │ Type: DEBIT                   │                       │
│ 📁 Amazon.com (8)               $450  │                               │ ─────────────────────── │
│   ├─ Jan 14  AMZN*Books      $29.99  │ AI Analysis:                  │ 🔍 Search: _          │
│   └─ Jan 10  AMZN*Prime     $139.00  │ "Regular grocery shopping"    │                       │
├───────────────────────────────────────┴───────────────────────────────┴───────────────────────┤
│ [↑↓] Navigate  [Enter] Classify  [Space] Multi-select  [/] Search  [?] Help                   │
│ Progress: ████████░░░░ 67/95 (71%)  Time Saved: ~12 min  Auto: 45  Manual: 22                │
└────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Architecture Overview

### Core Interface Implementation

The TUI implements the existing `engine.Prompter` interface, ensuring complete backward compatibility:

```go
// internal/tui/prompter.go
package tui

import (
    "context"
    "github.com/Veraticus/the-spice-must-flow/internal/engine"
    "github.com/Veraticus/the-spice-must-flow/internal/model"
    "github.com/Veraticus/the-spice-must-flow/internal/service"
    tea "github.com/charmbracelet/bubbletea"
)

// Prompter implements engine.Prompter with a rich TUI
type Prompter struct {
    program    *tea.Program
    model      Model
    resultChan chan promptResult
    errorChan  chan error
}

type promptResult struct {
    classification model.Classification
    err           error
}

// Ensure we implement the interface
var _ engine.Prompter = (*Prompter)(nil)

// New creates a TUI prompter that can replace the CLI prompter
func New(ctx context.Context, opts ...Option) (engine.Prompter, error) {
    cfg := defaultConfig()
    for _, opt := range opts {
        opt(&cfg)
    }
    
    model := newModel(cfg)
    p := &Prompter{
        model:      model,
        resultChan: make(chan promptResult, 1),
        errorChan:  make(chan error, 1),
    }
    
    p.program = tea.NewProgram(
        model,
        tea.WithContext(ctx),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )
    
    return p, nil
}

// ConfirmClassification implements engine.Prompter
func (p *Prompter) ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error) {
    // Send pending classification to TUI
    p.program.Send(classificationRequestMsg{
        pending: pending,
        single:  true,
    })
    
    // Wait for user interaction
    select {
    case result := <-p.resultChan:
        return result.classification, result.err
    case <-ctx.Done():
        return model.Classification{}, ctx.Err()
    }
}

// BatchConfirmClassifications implements engine.Prompter
func (p *Prompter) BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
    // Send batch to TUI
    p.program.Send(batchClassificationRequestMsg{
        pending: pending,
    })
    
    // Collect results
    var results []model.Classification
    for i := 0; i < len(pending); i++ {
        select {
        case result := <-p.resultChan:
            if result.err != nil {
                return nil, result.err
            }
            results = append(results, result.classification)
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    
    return results, nil
}
```

### Model-Update-View Architecture

The TUI follows bubbletea's MVU pattern with clear separation of concerns:

```go
// internal/tui/model.go
type Model struct {
    // Core state
    state       State
    view        View
    
    // Data
    transactions     []model.Transaction
    categories       []model.Category
    classifications  map[string]model.Classification
    pending         []model.PendingClassification
    
    // UI Components
    transactionList TransactionListModel
    classifier      ClassifierModel
    batchView       BatchViewModel
    statsPanel      StatsPanelModel
    
    // Services
    storage    service.Storage
    llm        engine.Classifier
    
    // Communication
    resultChan chan<- promptResult
    
    // Configuration
    config     Config
    keymap     KeyMap
    theme      Theme
}

type State int

const (
    StateList State = iota
    StateClassifying
    StateBatch
    StateExporting
)

type View int

const (
    ViewTransactions View = iota
    ViewMerchantGroups
    ViewCalendar
    ViewStats
)

func (m Model) Init() tea.Cmd {
    return tea.Batch(
        m.loadTransactions(),
        m.loadCategories(),
        tea.EnterAltScreen,
    )
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    
    // Handle global messages
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if cmd := m.handleGlobalKeys(msg); cmd != nil {
            return m, cmd
        }
    
    case tea.WindowSizeMsg:
        m.handleResize(msg)
        
    case dataLoadedMsg:
        m.handleDataLoaded(msg)
        
    case classificationRequestMsg:
        m.startClassification(msg.pending)
        return m, m.focusOnTransaction(msg.pending.Transaction.ID)
    }
    
    // Delegate to active component
    switch m.state {
    case StateList:
        newList, cmd := m.transactionList.Update(msg)
        m.transactionList = newList
        cmds = append(cmds, cmd)
        
    case StateClassifying:
        newClassifier, cmd := m.classifier.Update(msg)
        m.classifier = newClassifier
        cmds = append(cmds, cmd)
        
        // Check if classification is complete
        if newClassifier.IsComplete() {
            m.resultChan <- promptResult{
                classification: newClassifier.GetResult(),
            }
            m.state = StateList
        }
    }
    
    return m, tea.Batch(cmds...)
}

func (m Model) View() string {
    // Responsive layout based on terminal size
    if m.config.width < 80 {
        return m.renderCompactView()
    }
    
    if m.config.width < 120 {
        return m.renderMediumView()
    }
    
    return m.renderFullView()
}
```

### Message-Based Communication

All state changes happen through messages, ensuring predictable behavior:

```go
// internal/tui/messages.go

// Request messages from engine
type classificationRequestMsg struct {
    pending model.PendingClassification
    single  bool
}

type batchClassificationRequestMsg struct {
    pending []model.PendingClassification
}

// UI interaction messages
type transactionSelectedMsg struct {
    id    string
    index int
}

type categorySelectedMsg struct {
    category   model.Category
    confidence float64
}

type navigationMsg struct {
    direction Direction
    jump      bool // true for page up/down
}

// Async operation messages
type aiSuggestionMsg struct {
    transactionID string
    rankings      model.CategoryRankings
    err          error
}

type searchResultsMsg struct {
    query   string
    results []SearchResult
}

// State transition messages
type switchViewMsg struct {
    view View
}

type undoRequestMsg struct{}

type exportRequestMsg struct {
    format ExportFormat
}
```

## Component Design

### Transaction List Component

The main view for navigating and selecting transactions:

```go
// internal/tui/components/transaction_list.go
package components

import (
    "fmt"
    "github.com/Veraticus/the-spice-must-flow/internal/model"
    "github.com/charmbracelet/bubbles/table"
    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type TransactionListModel struct {
    // Data
    transactions []model.Transaction
    filtered     []model.Transaction
    groups       []TransactionGroup
    
    // UI State
    cursor       int
    selected     map[string]bool
    mode         ListMode
    
    // Components
    table        table.Model
    viewport     viewport.Model
    
    // Features
    search       SearchModel
    filter       FilterModel
    sort         SortConfig
    
    // Layout
    width        int
    height       int
    focused      bool
}

type ListMode int

const (
    ModeNormal ListMode = iota
    ModeVisual
    ModeSearch
    ModeFilter
)

type TransactionGroup struct {
    Merchant     string
    Transactions []model.Transaction
    Total        float64
    Collapsed    bool
}

func NewTransactionList(transactions []model.Transaction) TransactionListModel {
    // Group by merchant
    groups := groupTransactions(transactions)
    
    // Setup table
    columns := []table.Column{
        {Title: "Date", Width: 10},
        {Title: "Merchant", Width: 25},
        {Title: "Amount", Width: 12},
        {Title: "Category", Width: 20},
        {Title: "Status", Width: 10},
    }
    
    t := table.New(
        table.WithColumns(columns),
        table.WithFocused(true),
        table.WithHeight(20),
    )
    
    // Apply theme
    s := table.DefaultStyles()
    s.Header = s.Header.
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("240")).
        BorderBottom(true).
        Bold(false)
    s.Selected = s.Selected.
        Foreground(lipgloss.Color("229")).
        Background(lipgloss.Color("57")).
        Bold(false)
    t.SetStyles(s)
    
    return TransactionListModel{
        transactions: transactions,
        filtered:     transactions,
        groups:       groups,
        selected:     make(map[string]bool),
        table:        t,
        mode:         ModeNormal,
    }
}

func (m TransactionListModel) Update(msg tea.Msg) (TransactionListModel, tea.Cmd) {
    var cmds []tea.Cmd
    
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch m.mode {
        case ModeNormal:
            cmd := m.handleNormalMode(msg)
            cmds = append(cmds, cmd)
        case ModeVisual:
            cmd := m.handleVisualMode(msg)
            cmds = append(cmds, cmd)
        case ModeSearch:
            newSearch, cmd := m.search.Update(msg)
            m.search = newSearch
            cmds = append(cmds, cmd)
            
            // Apply search results
            if newSearch.IsComplete() {
                m.applySearch(newSearch.Results())
                m.mode = ModeNormal
            }
        }
    }
    
    // Update table
    newTable, cmd := m.table.Update(msg)
    m.table = newTable
    cmds = append(cmds, cmd)
    
    return m, tea.Batch(cmds...)
}

func (m TransactionListModel) handleNormalMode(msg tea.KeyMsg) tea.Cmd {
    switch msg.String() {
    case "j", "down":
        m.cursor = min(m.cursor+1, len(m.filtered)-1)
        return m.ensureVisible()
        
    case "k", "up":
        m.cursor = max(m.cursor-1, 0)
        return m.ensureVisible()
        
    case "G":
        m.cursor = len(m.filtered) - 1
        return m.ensureVisible()
        
    case "g":
        if m.lastKey == "g" {
            m.cursor = 0
            return m.ensureVisible()
        }
        
    case "v":
        m.mode = ModeVisual
        m.visualStart = m.cursor
        
    case "/":
        m.mode = ModeSearch
        m.search.Focus()
        
    case "enter":
        if m.cursor < len(m.filtered) {
            return selectTransactionCmd(m.filtered[m.cursor])
        }
    }
    
    return nil
}

func (m TransactionListModel) View() string {
    if m.height < 10 {
        return "Terminal too small"
    }
    
    // Build table rows
    rows := m.buildTableRows()
    m.table.SetRows(rows)
    
    // Layout sections
    header := m.renderHeader()
    table := m.table.View()
    footer := m.renderFooter()
    
    // Combine with proper spacing
    content := lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        table,
        footer,
    )
    
    return m.style.Render(content)
}

// Test helper for creating fake data
func MakeTestTransactions(count int) []model.Transaction {
    merchants := []string{
        "Whole Foods Market",
        "Amazon.com",
        "Shell Oil",
        "Netflix",
        "Starbucks",
        "Target",
        "Uber",
    }
    
    var transactions []model.Transaction
    for i := 0; i < count; i++ {
        merchant := merchants[i%len(merchants)]
        transactions = append(transactions, model.Transaction{
            ID:           fmt.Sprintf("txn_%d", i),
            MerchantName: merchant,
            Name:         fmt.Sprintf("%s #%d", merchant, i),
            Amount:       float64(20 + i*10%200),
            Date:         time.Now().AddDate(0, 0, -i),
            Direction:    model.DirectionExpense,
        })
    }
    
    return transactions
}
```

### AI-Powered Classifier Component

Smart classification with visual feedback:

```go
// internal/tui/components/classifier.go
type ClassifierModel struct {
    // Current transaction
    transaction model.Transaction
    pending     model.PendingClassification
    
    // AI state
    rankings     model.CategoryRankings
    loading      bool
    error        error
    
    // User selection
    cursor       int
    customInput  textinput.Model
    mode         ClassifierMode
    
    // Visual elements
    progressBar  progress.Model
    sparkline    Sparkline
    
    // Result
    result       *model.Classification
    complete     bool
}

type ClassifierMode int

const (
    ModeSelectingSuggestion ClassifierMode = iota
    ModeEnteringCustom
    ModeConfirming
)

func (m ClassifierModel) Update(msg tea.Msg) (ClassifierModel, tea.Cmd) {
    switch msg := msg.(type) {
    case aiSuggestionMsg:
        m.loading = false
        if msg.err != nil {
            m.error = msg.err
        } else {
            m.rankings = msg.rankings
        }
        
    case tea.KeyMsg:
        switch m.mode {
        case ModeSelectingSuggestion:
            switch msg.String() {
            case "j", "down":
                m.cursor = min(m.cursor+1, len(m.rankings)-1)
                
            case "k", "up":
                m.cursor = max(m.cursor-1, 0)
                
            case "enter", "a":
                // Accept selected suggestion
                if m.cursor < len(m.rankings) {
                    return m, m.confirmCategory(m.rankings[m.cursor])
                }
                
            case "c":
                // Custom category
                m.mode = ModeEnteringCustom
                m.customInput.Focus()
                return m, textinput.Blink
                
            case "s":
                // Skip
                m.result = &model.Classification{
                    Transaction: m.transaction,
                    Status:     model.StatusUnclassified,
                }
                m.complete = true
            }
            
        case ModeEnteringCustom:
            var cmd tea.Cmd
            m.customInput, cmd = m.customInput.Update(msg)
            
            if msg.String() == "enter" {
                category := m.customInput.Value()
                if category != "" {
                    return m, m.createCustomCategory(category)
                }
            }
            
            return m, cmd
        }
    }
    
    return m, nil
}

func (m ClassifierModel) View() string {
    if m.loading {
        return m.renderLoading()
    }
    
    if m.error != nil {
        return m.renderError()
    }
    
    sections := []string{
        m.renderTransaction(),
        m.renderSuggestions(),
    }
    
    if m.mode == ModeEnteringCustom {
        sections = append(sections, m.renderCustomInput())
    }
    
    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m ClassifierModel) renderSuggestions() string {
    if len(m.rankings) == 0 {
        return "No suggestions available"
    }
    
    var suggestions []string
    for i, ranking := range m.rankings[:min(5, len(m.rankings))] {
        // Build confidence bar
        confidence := int(ranking.Score * 100)
        bar := m.renderConfidenceBar(confidence)
        
        // Format line
        prefix := "  "
        if i == m.cursor {
            prefix = "> "
        }
        
        line := fmt.Sprintf("%s%s %s %d%%",
            prefix,
            ranking.Category,
            bar,
            confidence,
        )
        
        if i == m.cursor {
            line = m.theme.Selected.Render(line)
        }
        
        suggestions = append(suggestions, line)
    }
    
    return m.theme.Box.Render(strings.Join(suggestions, "\n"))
}

func (m ClassifierModel) renderConfidenceBar(confidence int) string {
    width := 20
    filled := int(float64(width) * float64(confidence) / 100.0)
    
    bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
    
    color := m.theme.Success
    if confidence < 50 {
        color = m.theme.Error
    } else if confidence < 80 {
        color = m.theme.Warning
    }
    
    return color.Render(bar)
}
```

### Statistics and Progress Component

Real-time session statistics:

```go
// internal/tui/components/stats.go
type StatsModel struct {
    // Session data
    total         int
    classified    int
    autoClassified int
    skipped       int
    
    // Time tracking
    startTime     time.Time
    avgTime       time.Duration
    
    // Visual elements
    progressBar   progress.Model
    sparkline     Sparkline
    distribution  PieChart
}

func (m StatsModel) View() string {
    progress := float64(m.classified) / float64(m.total)
    
    stats := []string{
        fmt.Sprintf("Progress: %d/%d (%.0f%%)", m.classified, m.total, progress*100),
        fmt.Sprintf("Auto-classified: %d", m.autoClassified),
        fmt.Sprintf("Time saved: %s", m.calculateTimeSaved()),
        "",
        m.renderCategoryDistribution(),
    }
    
    return m.theme.StatsBox.Render(strings.Join(stats, "\n"))
}

func (m StatsModel) renderCategoryDistribution() string {
    // ASCII art bar chart
    categories := m.getCategoryStats()
    maxCount := 0
    for _, count := range categories {
        if count > maxCount {
            maxCount = count
        }
    }
    
    var bars []string
    for cat, count := range categories {
        barLen := int(float64(count) / float64(maxCount) * 20)
        bar := strings.Repeat("█", barLen)
        line := fmt.Sprintf("%-15s %s %d", cat, bar, count)
        bars = append(bars, line)
    }
    
    return strings.Join(bars, "\n")
}
```

## User Experience Flows

### Navigation Patterns

The TUI supports multiple navigation paradigms:

```go
// internal/tui/navigation/patterns.go

// Vim-style navigation
var vimBindings = KeyBindings{
    "h": moveLeft,
    "j": moveDown,
    "k": moveUp,
    "l": moveRight,
    "g": startGCommand,
    "G": moveToEnd,
    "/": startSearch,
    "n": nextMatch,
    "N": prevMatch,
    "v": enterVisualMode,
    "V": enterVisualLineMode,
    "y": yankSelected,
    "p": pasteAfter,
}

// Quick jumps
var jumpCommands = map[string]tea.Cmd{
    "gg": jumpToStart,
    "G":  jumpToEnd,
    "gd": jumpToNextDay,
    "gD": jumpToPrevDay,
    "gm": jumpToNextMonth,
    "gM": jumpToPrevMonth,
    "gu": jumpToUnclassified,
}

// Mouse support
func handleMouse(m Model, msg tea.MouseMsg) (Model, tea.Cmd) {
    switch msg.Type {
    case tea.MouseWheelUp:
        return m, scrollUp(3)
    case tea.MouseWheelDown:
        return m, scrollDown(3)
    case tea.MouseLeft:
        return m, selectAtPosition(msg.X, msg.Y)
    case tea.MouseRight:
        return m, showContextMenu(msg.X, msg.Y)
    }
    return m, nil
}
```

### Batch Operations Flow

Efficient bulk classification:

```go
// internal/tui/flows/batch.go
type BatchFlow struct {
    state BatchState
    steps []BatchStep
}

type BatchState int

const (
    BatchStateSelecting BatchState = iota
    BatchStateGrouping
    BatchStatePreviewing
    BatchStateConfirming
    BatchStateProcessing
    BatchStateComplete
)

func (f *BatchFlow) Start(transactions []model.Transaction) tea.Cmd {
    f.state = BatchStateSelecting
    f.steps = []BatchStep{
        {Name: "Select Transactions", Complete: false},
        {Name: "Review Grouping", Complete: false},
        {Name: "Preview Changes", Complete: false},
        {Name: "Confirm", Complete: false},
    }
    
    return showBatchInterfaceCmd()
}

func (f *BatchFlow) Update(msg tea.Msg) (BatchFlow, tea.Cmd) {
    switch f.state {
    case BatchStateSelecting:
        switch msg := msg.(type) {
        case selectionCompleteMsg:
            f.steps[0].Complete = true
            f.state = BatchStateGrouping
            return f, f.analyzeGroups(msg.selected)
        }
        
    case BatchStateGrouping:
        switch msg := msg.(type) {
        case groupingCompleteMsg:
            f.steps[1].Complete = true
            f.state = BatchStatePreviewing
            return f, f.generatePreview(msg.groups)
        }
        
    case BatchStatePreviewing:
        switch msg := msg.(type) {
        case tea.KeyMsg:
            if msg.String() == "enter" {
                f.steps[2].Complete = true
                f.state = BatchStateConfirming
                return f, nil
            }
        }
        
    case BatchStateConfirming:
        switch msg := msg.(type) {
        case confirmMsg:
            if msg.confirmed {
                f.steps[3].Complete = true
                f.state = BatchStateProcessing
                return f, f.processBatch()
            }
        }
    }
    
    return f, nil
}
```

## Testing Strategy

### Unit Testing Components

Each component is independently testable:

```go
// internal/tui/components/transaction_list_test.go
func TestTransactionList(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() TransactionListModel
        input    tea.Msg
        validate func(t *testing.T, m TransactionListModel)
    }{
        {
            name: "navigate down increases cursor",
            setup: func() TransactionListModel {
                return NewTransactionList(MakeTestTransactions(10))
            },
            input: tea.KeyMsg{Type: tea.KeyDown},
            validate: func(t *testing.T, m TransactionListModel) {
                assert.Equal(t, 1, m.cursor)
            },
        },
        {
            name: "visual mode selection",
            setup: func() TransactionListModel {
                m := NewTransactionList(MakeTestTransactions(10))
                m.mode = ModeVisual
                m.visualStart = 2
                m.cursor = 5
                return m
            },
            input: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}},
            validate: func(t *testing.T, m TransactionListModel) {
                assert.Equal(t, 4, len(m.selected))
                assert.Equal(t, ModeNormal, m.mode)
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            model := tt.setup()
            newModel, _ := model.Update(tt.input)
            tt.validate(t, newModel)
        })
    }
}
```

### Integration Testing with Fake Data

Test complete workflows with realistic data:

```go
// internal/tui/integration_test.go
func TestClassificationWorkflow(t *testing.T) {
    // Create test harness
    th := NewTestHarness(t)
    
    // Setup fake data
    transactions := []model.Transaction{
        {
            ID:           "t1",
            MerchantName: "Whole Foods",
            Amount:       67.23,
            Date:         time.Now(),
        },
        {
            ID:           "t2",
            MerchantName: "Shell Oil",
            Amount:       45.00,
            Date:         time.Now().AddDate(0, 0, -1),
        },
    }
    
    categories := []model.Category{
        {ID: "groceries", Name: "Groceries"},
        {ID: "transport", Name: "Transportation"},
    }
    
    // Configure mock classifier
    th.MockClassifier.SetResponse("t1", model.CategoryRankings{
        {Category: "Groceries", Score: 0.92},
        {Category: "Dining", Score: 0.05},
    })
    
    // Initialize TUI
    tui := th.StartTUI(transactions, categories)
    
    // Execute workflow
    workflow := []tea.Msg{
        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Move down
        tea.KeyMsg{Type: tea.KeyEnter},                      // Select transaction
        th.WaitFor(aiSuggestionMsg{}),                       // Wait for AI
        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, // Accept
    }
    
    results := th.ExecuteWorkflow(tui, workflow)
    
    // Verify outcomes
    assert.Equal(t, "groceries", results.Classifications["t1"].CategoryID)
    assert.Equal(t, model.StatusClassifiedByAI, results.Classifications["t1"].Status)
    
    // Verify visual output
    th.AssertScreenContains("Groceries (92%)")
    th.AssertScreenContains("✓ Classified")
}

// Test harness for easier testing
type TestHarness struct {
    t              *testing.T
    MockStorage    *MockStorage
    MockClassifier *MockClassifier
    screens        []string
    results        TestResults
}

func (th *TestHarness) StartTUI(txns []model.Transaction, cats []model.Category) *Prompter {
    model := newModel(Config{
        Storage:    th.MockStorage,
        Classifier: th.MockClassifier,
    })
    
    model.transactions = txns
    model.categories = cats
    
    return &Prompter{
        model:      model,
        resultChan: make(chan promptResult, 10),
    }
}

func (th *TestHarness) ExecuteWorkflow(tui *Prompter, workflow []tea.Msg) TestResults {
    for _, msg := range workflow {
        // Update model
        newModel, cmd := tui.model.Update(msg)
        tui.model = newModel
        
        // Capture screen
        th.screens = append(th.screens, tui.model.View())
        
        // Execute commands
        if cmd != nil {
            th.executeCmd(tui, cmd)
        }
    }
    
    return th.results
}
```

### Visual Testing with Golden Files

Ensure UI consistency across changes:

```go
// internal/tui/visual_test.go
func TestVisualAppearance(t *testing.T) {
    scenarios := []struct {
        name   string
        setup  func() Model
        width  int
        height int
    }{
        {
            name: "transaction_list_empty",
            setup: func() Model {
                return newModel(Config{}).
                    WithSize(80, 24).
                    WithTransactions([]model.Transaction{})
            },
        },
        {
            name: "transaction_list_full",
            setup: func() Model {
                return newModel(Config{}).
                    WithSize(120, 40).
                    WithTransactions(MakeRealisticTransactions(100))
            },
        },
        {
            name: "classification_view",
            setup: func() Model {
                m := newModel(Config{}).WithSize(100, 30)
                m.state = StateClassifying
                m.classifier = ClassifierModel{
                    transaction: MakeTestTransaction(),
                    rankings: model.CategoryRankings{
                        {Category: "Groceries", Score: 0.92},
                        {Category: "Dining", Score: 0.45},
                        {Category: "Shopping", Score: 0.12},
                    },
                }
                return m
            },
        },
    }
    
    for _, sc := range scenarios {
        t.Run(sc.name, func(t *testing.T) {
            model := sc.setup()
            output := model.View()
            
            goldenFile := filepath.Join("testdata", "golden", sc.name+".txt")
            
            if *updateGolden {
                err := os.WriteFile(goldenFile, []byte(output), 0644)
                require.NoError(t, err)
                t.Skip("Updated golden file")
            }
            
            expected, err := os.ReadFile(goldenFile)
            require.NoError(t, err)
            
            if string(expected) != output {
                t.Errorf("Visual output mismatch\nExpected:\n%s\nActual:\n%s",
                    expected, output)
                
                // Write actual output for inspection
                actualFile := filepath.Join("testdata", "actual", sc.name+".txt")
                _ = os.WriteFile(actualFile, []byte(output), 0644)
            }
        })
    }
}

// Realistic test data generator
func MakeRealisticTransactions(count int) []model.Transaction {
    rand.Seed(42) // Deterministic for tests
    
    merchants := []struct {
        name     string
        category string
        minAmt   float64
        maxAmt   float64
    }{
        {"Whole Foods Market", "Groceries", 20, 200},
        {"Shell Oil 12345", "Transportation", 30, 80},
        {"Netflix.com", "Entertainment", 15, 15},
        {"Amazon.com", "Shopping", 10, 500},
        {"Starbucks", "Dining", 3, 12},
        {"Target", "Shopping", 15, 300},
        {"Uber", "Transportation", 8, 45},
    }
    
    var transactions []model.Transaction
    baseDate := time.Now()
    
    for i := 0; i < count; i++ {
        merchant := merchants[i%len(merchants)]
        amount := merchant.minAmt + rand.Float64()*(merchant.maxAmt-merchant.minAmt)
        
        transactions = append(transactions, model.Transaction{
            ID:           fmt.Sprintf("txn_%d", i),
            Hash:         fmt.Sprintf("hash_%d", i),
            Date:         baseDate.AddDate(0, 0, -(i/3)),
            Name:         strings.ToUpper(merchant.name),
            MerchantName: merchant.name,
            Amount:       math.Round(amount*100) / 100,
            AccountID:    "acc_001",
            Direction:    model.DirectionExpense,
        })
    }
    
    return transactions
}
```

### Performance Testing

Ensure smooth performance with large datasets:

```go
// internal/tui/performance_test.go
func BenchmarkLargeDataset(b *testing.B) {
    transactions := MakeRealisticTransactions(10000)
    categories := MakeTestCategories(50)
    
    model := newModel(Config{
        EnableVirtualScrolling: true,
        EnableCaching:         true,
    })
    model.transactions = transactions
    model.categories = categories
    
    b.Run("initial_render", func(b *testing.B) {
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            _ = model.View()
        }
    })
    
    b.Run("scroll_performance", func(b *testing.B) {
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
            _ = model.View()
        }
    })
    
    b.Run("search_performance", func(b *testing.B) {
        searchMsg := searchQueryMsg{query: "whole foods"}
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            model, _ = model.Update(searchMsg)
        }
    })
}

// Memory usage testing
func TestMemoryUsage(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping memory test in short mode")
    }
    
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    startAlloc := m.Alloc
    
    // Create large dataset
    model := newModel(Config{})
    model.transactions = MakeRealisticTransactions(50000)
    
    // Simulate user interactions
    for i := 0; i < 1000; i++ {
        model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
        _ = model.View()
    }
    
    runtime.GC()
    runtime.ReadMemStats(&m)
    endAlloc := m.Alloc
    
    memUsed := endAlloc - startAlloc
    memUsedMB := float64(memUsed) / 1024 / 1024
    
    assert.Less(t, memUsedMB, 100.0, "Memory usage should be under 100MB")
    t.Logf("Memory used: %.2f MB", memUsedMB)
}
```

## Integration Guide

### Enabling the TUI

The TUI can be enabled through configuration or command-line flags:

```go
// cmd/spice/classify.go
func runClassify(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    
    // Determine UI mode
    var prompter engine.Prompter
    if viper.GetBool("tui.enabled") || cmd.Flags().Changed("tui") {
        // Use rich TUI
        tuiPrompter, err := tui.New(ctx,
            tui.WithTheme(viper.GetString("tui.theme")),
            tui.WithKeyBindings(loadKeyBindings()),
        )
        if err != nil {
            return fmt.Errorf("failed to initialize TUI: %w", err)
        }
        
        // Start TUI in background
        go func() {
            if err := tuiPrompter.Start(); err != nil {
                log.Printf("TUI error: %v", err)
            }
        }()
        
        prompter = tuiPrompter
    } else {
        // Use traditional CLI
        prompter = cli.NewCLIPrompter(nil, nil)
    }
    
    // Rest of the classification logic remains the same
    engine := engine.New(db, classifier, prompter)
    return engine.ClassifyTransactions(ctx, fromDate)
}
```

### Configuration Options

```yaml
# ~/.config/spice/config.yml
tui:
  enabled: true
  theme: "catppuccin-mocha"
  
  features:
    virtual_scrolling: true
    mouse_support: true
    animations: true
    
  keybindings:
    quit: "q"
    help: "?"
    classify: "c"
    batch: "b"
    search: "/"
    filter: "f"
    
  layout:
    default_view: "transactions"
    show_preview: true
    sidebar_width: 30
```

### Theme Support

```go
// internal/tui/themes/themes.go
var BuiltinThemes = map[string]Theme{
    "default": {
        Primary:     lipgloss.Color("#7c3aed"),
        Secondary:   lipgloss.Color("#a78bfa"),
        Success:     lipgloss.Color("#10b981"),
        Warning:     lipgloss.Color("#f59e0b"),
        Error:       lipgloss.Color("#ef4444"),
        Background:  lipgloss.Color("#1a1a1a"),
        Foreground:  lipgloss.Color("#fafafa"),
        Border:      lipgloss.Color("#404040"),
    },
    "catppuccin-mocha": {
        Primary:     lipgloss.Color("#cba6f7"),
        Secondary:   lipgloss.Color("#f5c2e7"),
        Success:     lipgloss.Color("#a6e3a1"),
        Warning:     lipgloss.Color("#f9e2af"),
        Error:       lipgloss.Color("#f38ba8"),
        Background:  lipgloss.Color("#1e1e2e"),
        Foreground:  lipgloss.Color("#cdd6f4"),
        Border:      lipgloss.Color("#45475a"),
    },
}
```

## Performance Optimizations

### Virtual Scrolling

Handle large datasets efficiently:

```go
// internal/tui/components/virtual_scroll.go
type VirtualScroller struct {
    items       []Renderable
    viewport    viewport.Model
    cache       *RenderCache
    
    // Optimization settings
    overscan    int // Render extra items outside viewport
    cacheSize   int
    itemHeight  int // Fixed height for calculations
}

func (v *VirtualScroller) View() string {
    // Calculate visible range with overscan
    topIndex := max(0, v.viewport.YOffset/v.itemHeight - v.overscan)
    bottomIndex := min(len(v.items), 
        (v.viewport.YOffset+v.viewport.Height)/v.itemHeight + v.overscan)
    
    // Render only visible items
    var rendered []string
    for i := topIndex; i < bottomIndex; i++ {
        if cached, ok := v.cache.Get(i); ok {
            rendered = append(rendered, cached)
        } else {
            item := v.renderItem(i)
            v.cache.Set(i, item)
            rendered = append(rendered, item)
        }
    }
    
    // Add padding for items above
    topPadding := topIndex * v.itemHeight
    content := strings.Repeat("\n", topPadding/v.itemHeight) + 
               strings.Join(rendered, "\n")
    
    return v.viewport.View(content)
}
```

### Intelligent Caching

Cache expensive operations:

```go
// internal/tui/cache/intelligent.go
type IntelligentCache struct {
    render      *LRUCache      // Rendered strings
    data        *TTLCache      // API responses
    predictions *PredictCache  // Prefetch predictions
}

// Prefetch likely next items
func (c *IntelligentCache) PrefetchNext(current int, direction int) {
    // Predict next items based on navigation pattern
    predicted := make([]int, 0, 10)
    for i := 1; i <= 10; i++ {
        next := current + (direction * i)
        predicted = append(predicted, next)
    }
    
    // Async prefetch
    go c.prefetchItems(predicted)
}
```

### Async Operations

Non-blocking UI updates:

```go
// internal/tui/async/operations.go
type AsyncManager struct {
    operations map[string]*Operation
    results    chan OperationResult
    mu         sync.RWMutex
}

func (m *AsyncManager) ClassifyWithAI(
    ctx context.Context,
    transaction model.Transaction,
) tea.Cmd {
    op := m.StartOperation("classify", transaction.ID)
    
    return func() tea.Msg {
        // Long-running AI call
        rankings, err := m.classifier.SuggestCategoryRankings(
            ctx, transaction, m.categories,
        )
        
        m.CompleteOperation(op.ID)
        
        return aiSuggestionMsg{
            transactionID: transaction.ID,
            rankings:     rankings,
            err:          err,
        }
    }
}
```

## Development Workflow

### Quick Iteration with Hot Reload

```bash
# Development mode with hot reload
make dev-tui

# Run with test data
spice classify --tui --test-data

# Visual regression tests
go test ./internal/tui/... -update-golden
```

### Test Data Generator

```go
// cmd/spice/testdata.go
var testDataCmd = &cobra.Command{
    Use:   "test-data",
    Short: "Generate test data for TUI development",
    RunE: func(cmd *cobra.Command, args []string) error {
        count := viper.GetInt("count")
        
        // Generate realistic transactions
        transactions := tui.GenerateTestTransactions(count)
        
        // Save to database
        db, _ := storage.NewSQLiteStorage(":memory:")
        db.SaveTransactions(context.Background(), transactions)
        
        // Start TUI with test data
        prompter, _ := tui.New(context.Background(),
            tui.WithTestMode(true),
            tui.WithStorage(db),
        )
        
        return prompter.Start()
    },
}
```

### Debug Mode

```go
// internal/tui/debug/panel.go
type DebugPanel struct {
    enabled bool
    events  []DebugEvent
    state   map[string]interface{}
}

func (d *DebugPanel) View() string {
    if !d.enabled {
        return ""
    }
    
    sections := []string{
        d.renderEvents(),
        d.renderState(),
        d.renderPerformance(),
    }
    
    return d.theme.Debug.Render(
        strings.Join(sections, "\n---\n"),
    )
}

// Enable with Ctrl+D in development
func (m Model) handleDebugToggle() (Model, tea.Cmd) {
    m.debug.enabled = !m.debug.enabled
    m.debug.RecordEvent("Debug toggled", map[string]interface{}{
        "enabled": m.debug.enabled,
        "time":    time.Now(),
    })
    return m, nil
}
```

## Summary

This TUI design provides:

1. **Complete Backward Compatibility** - Implements `engine.Prompter` interface
2. **Superior User Experience** - Multi-pane layout, vim bindings, visual feedback
3. **Comprehensive Testing** - Unit, integration, visual, and performance tests
4. **High Performance** - Virtual scrolling, intelligent caching, async operations
5. **Developer Friendly** - Easy testing with fake data, hot reload, debug mode

The architecture follows Go best practices with small, focused interfaces, concrete types over `interface{}`, and explicit error handling. The testing strategy ensures reliability while the modular design enables easy iteration and enhancement.