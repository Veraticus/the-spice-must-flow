# TUI Testing Design: View Model Architecture

## Problem Statement

Currently, I cannot see or interact with the TUI, making debugging slow and frustrating. The core issue is that the TUI's View() methods conflate data and presentation, returning styled strings meant for human consumption rather than program verification.

## Design Principle

**No test-specific code paths**. We test the real system by making the real system inherently testable through better architecture.

## Proposed Solution: View Model Separation

Separate data (what to show) from presentation (how to show it) using a View Model pattern. This makes the TUI testable without any test-specific branches or modes.

## Architecture Overview

```
Current Architecture:                    New Architecture:
┌─────────────────┐                     ┌─────────────────┐
│   Component     │                     │   Component     │
│                 │                     │                 │
│ State + Logic   │                     │ State + Logic   │
│       +         │                     └────────┬────────┘
│ View() string   │                              │
│ (mixed concerns)│                              ▼
└─────────────────┘                     ┌─────────────────┐
                                        │   View Model    │
                                        │  (pure data)    │
                                        └────────┬────────┘
                                                 │
                                        ┌────────▼────────┐
                                        │    Renderer     │
                                        │ (presentation)  │
                                        └─────────────────┘
```

## Core Types

### View Models

```go
// Package viewmodel defines the data structures for TUI rendering
package viewmodel

// ClassifierView represents the classifier component's display data
type ClassifierView struct {
    Transaction  TransactionView
    Mode         ClassifierMode
    Categories   []CategoryView
    Cursor       int
    CustomInput  string
    Error        string
}

// TransactionView represents transaction display data
type TransactionView struct {
    ID           string
    MerchantName string
    Amount       float64
    Date         time.Time
    Direction    model.TransactionDirection
}

// CategoryView represents a category option
type CategoryView struct {
    ID          int
    Name        string
    Icon        string
    Confidence  float64
    IsSelected  bool
    HasPattern  bool
}

// BatchView represents batch classification display data
type BatchView struct {
    Mode         BatchMode
    Groups       []TransactionGroup
    CurrentGroup int
    Progress     ProgressView
}

// ProgressView represents progress information
type ProgressView struct {
    Current int
    Total   int
    Status  string
}
```

### Renderer Interface

```go
// Package render handles view model to string conversion
package render

// Renderer converts view models to displayable strings
type Renderer interface {
    RenderClassifier(view viewmodel.ClassifierView) string
    RenderBatch(view viewmodel.BatchView) string
    RenderTransactionList(view viewmodel.TransactionListView) string
    RenderStats(view viewmodel.StatsView) string
}

// TerminalRenderer creates rich terminal output with colors and borders
type TerminalRenderer struct {
    theme themes.Theme
}

// PlainRenderer creates simple, parseable output for testing
type PlainRenderer struct{}
```

## Implementation Changes

### Before: Mixed Concerns

```go
func (m ClassifierModel) View() string {
    if m.loading {
        return m.renderLoading()
    }
    
    sections := []string{
        m.renderTransaction(),
        m.renderSuggestions(),
    }
    
    // 100+ lines mixing data access and styling...
    
    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
```

### After: Separated Concerns

```go
// Pure data extraction
func (m ClassifierModel) GetViewModel() viewmodel.ClassifierView {
    return viewmodel.ClassifierView{
        Transaction: viewmodel.TransactionView{
            ID:           m.transaction.ID,
            MerchantName: m.transaction.MerchantName,
            Amount:       m.transaction.Amount,
            Date:         m.transaction.Date,
            Direction:    m.transaction.Direction,
        },
        Mode:       m.mode,
        Categories: m.buildCategoryViews(),
        Cursor:     m.cursor,
        CustomInput: m.customInput.Value(),
        Error:      m.error,
    }
}

// Bubble Tea interface compliance
func (m ClassifierModel) View() string {
    return m.renderer.RenderClassifier(m.GetViewModel())
}

// Pure presentation logic
func (r TerminalRenderer) RenderClassifier(view viewmodel.ClassifierView) string {
    // All lipgloss styling here, working with pure data
}
```

## Testing Approach

### Simple Unit Tests

```go
func TestCategorySelection(t *testing.T) {
    // Create component with test data
    classifier := components.NewClassifierModel(
        testPending,
        testCategories,
        themes.Default,
        nil,
    )
    
    // Simulate user pressing 'c'
    classifier, _ = classifier.Update(tea.KeyMsg{
        Type: tea.KeyRunes,
        Runes: []rune{'c'},
    })
    
    // Verify view model state
    view := classifier.GetViewModel()
    assert.Equal(t, viewmodel.ModeSelectingCategory, view.Mode)
    assert.Equal(t, 20, len(view.Categories))
    assert.Equal(t, 0, view.Cursor)
    
    // Verify first category is selected
    assert.True(t, view.Categories[0].IsSelected)
    
    // Verify categories are sorted by confidence
    assert.Greater(t, view.Categories[0].Confidence, view.Categories[5].Confidence)
}
```

### Visual Testing

```go
func TestVisualOutput(t *testing.T) {
    classifier := createTestClassifier()
    view := classifier.GetViewModel()
    
    // Use plain renderer for golden files
    renderer := render.NewPlainRenderer()
    output := renderer.RenderClassifier(view)
    
    // Compare with golden file
    golden := filepath.Join("testdata", "classifier_category_selection.golden")
    if *update {
        os.WriteFile(golden, []byte(output), 0644)
    }
    
    expected, _ := os.ReadFile(golden)
    assert.Equal(t, string(expected), output)
}
```

### Plain Renderer Output Example

```
=== CLASSIFIER ===
Transaction: Whole Foods Market
Amount: 67.23
Date: 2024-01-15
Direction: expense

Mode: SelectingCategory

Categories (20 total, showing 1-10):
> [1] Groceries (92% confidence) 
  [5] Shopping (45% confidence)
  [2] Dining Out (12% confidence)
  [6] Healthcare (5% confidence)
  [20] Other (2% confidence)
  [18] Charity
  [9] Education
  [4] Entertainment
  [11] Fitness
  [13] Gifts

Cursor: 0
Offset: 0
```

## Implementation Phases

### Phase 1: Core View Model Types (2-3 hours)
1. Create `internal/tui/viewmodel` package
2. Define all view model structs
3. Add conversion helpers
4. Create basic tests

Deliverables:
- [ ] `viewmodel/classifier.go`
- [ ] `viewmodel/batch.go`
- [ ] `viewmodel/transaction.go`
- [ ] `viewmodel/common.go`

### Phase 2: Renderer Interface (2-3 hours)
1. Create `internal/tui/render` package
2. Define Renderer interface
3. Implement PlainRenderer
4. Create renderer tests

Deliverables:
- [ ] `render/renderer.go`
- [ ] `render/plain.go`
- [ ] `render/plain_test.go`

### Phase 3: Component Refactoring (4-5 hours)
1. Add GetViewModel to each component
2. Extract rendering logic
3. Update View() to use renderer
4. Maintain backward compatibility

Deliverables:
- [ ] `components/classifier.go` - Add GetViewModel
- [ ] `components/batch.go` - Add GetViewModel
- [ ] `components/transaction_list.go` - Add GetViewModel
- [ ] `components/stats.go` - Add GetViewModel

### Phase 4: Terminal Renderer (3-4 hours)
1. Create TerminalRenderer
2. Move all lipgloss code
3. Preserve existing styling
4. Test visual output

Deliverables:
- [ ] `render/terminal.go`
- [ ] `render/terminal_test.go`
- [ ] Visual regression tests

### Phase 5: Comprehensive Tests (3-4 hours)
1. Test all component states
2. Test all transitions
3. Create golden files
4. Add integration tests

Deliverables:
- [ ] Component unit tests
- [ ] Golden file tests
- [ ] Integration tests
- [ ] Test data generators

## Benefits

1. **No Test-Specific Code**: Testing uses the same code paths as production
2. **Type Safety**: View models are strongly typed with no interface{}
3. **Maintainable**: Style changes don't break tests
4. **Debuggable**: Can inspect exact data at any point
5. **Extensible**: Easy to add new renderers (JSON, HTML, etc.)

## Success Criteria

After implementation:
- [ ] All TUI states are verifiable through view models
- [ ] Tests run without any terminal emulation
- [ ] Visual output is validated through golden files
- [ ] No test-specific flags or modes in components
- [ ] Debugging is possible by inspecting view model data

## Questions Resolved

1. **Q: Will this catch all bugs?**
   A: Yes, because we test the actual data and logic, not terminal output.

2. **Q: Is this idiomatic Go?**
   A: Yes, small interfaces, no interface{}, strong typing throughout.

3. **Q: How much work is this?**
   A: ~15-20 hours total, but can be done incrementally.

## Next Steps

Begin with Phase 1 to establish the view model types. This immediately enables testing even before the full renderer system is in place.