// Package workflows provides test workflow builders for integration testing.
package workflows

import (
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	"github.com/stretchr/testify/require"
)

// Builder provides a fluent API for constructing TUI test workflows.
type Builder struct {
	t          *testing.T
	harness    *integration.Harness
	assertions *integration.Assertions
	name       string
	steps      []Step
	timeout    time.Duration
}

// Step represents a single workflow step.
type Step struct {
	Name        string
	Action      func(*integration.Harness) error
	Assertions  []func(*testing.T, *integration.Assertions)
	SkipOnError bool
}

// NewBuilder creates a new workflow builder.
func NewBuilder(t *testing.T, harness *integration.Harness) *Builder {
	t.Helper()
	return &Builder{
		t:          t,
		harness:    harness,
		assertions: integration.NewAssertions(harness),
		steps:      []Step{},
		timeout:    10 * time.Second,
	}
}

// WithName sets the workflow name.
func (b *Builder) WithName(name string) *Builder {
	b.name = name
	return b
}

// WithTimeout sets the workflow timeout.
func (b *Builder) WithTimeout(timeout time.Duration) *Builder {
	b.timeout = timeout
	return b
}

// LoadTransactions adds a step to load test transactions.
func (b *Builder) LoadTransactions(transactions []model.Transaction) *Builder {
	b.steps = append(b.steps, Step{
		Name: "Load transactions",
		Action: func(h *integration.Harness) error {
			h.LoadTransactions(transactions)
			return nil
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertTransactionCount(t, len(transactions))
			},
		},
	})
	return b
}

// NavigateToTransaction adds navigation steps to reach a specific transaction.
func (b *Builder) NavigateToTransaction(txnID string) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Navigate to transaction %s", txnID),
		Action: func(h *integration.Harness) error {
			return h.SelectTransaction(txnID)
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertSelectedTransaction(t, txnID)
			},
		},
	})
	return b
}

// StartClassification adds a step to begin classification.
func (b *Builder) StartClassification() *Builder {
	b.steps = append(b.steps, Step{
		Name: "Start classification",
		Action: func(h *integration.Harness) error {
			h.SendKeys("enter")
			return h.WaitForState(tui.StateClassifying, 1*time.Second)
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertCurrentState(t, tui.StateClassifying)
				// Give the UI a moment to render after state change
				time.Sleep(100 * time.Millisecond)
				a.AssertCategoryListVisible(t)
			},
		},
	})
	return b
}

// SelectCategorySuggestion adds a step to select a category by number.
func (b *Builder) SelectCategorySuggestion(number int) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Select category suggestion #%d", number),
		Action: func(h *integration.Harness) error {
			h.SendKeys(fmt.Sprintf("%d", number))
			return nil
		},
	})
	return b
}

// CustomClassification adds a step for custom category entry.
func (b *Builder) CustomClassification(categoryName string) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Enter custom category: %s", categoryName),
		Action: func(h *integration.Harness) error {
			h.SendKeys("c") // Go to category picker
			time.Sleep(100 * time.Millisecond)
			h.SendKeys("/") // Switch to custom input mode
			time.Sleep(100 * time.Millisecond)

			// Type the category name
			for _, char := range categoryName {
				h.SendKeys(string(char))
			}

			h.SendKeys("enter")
			return nil
		},
	})
	return b
}

// AcceptClassification adds a step to accept the current classification.
func (b *Builder) AcceptClassification() *Builder {
	b.steps = append(b.steps, Step{
		Name: "Accept classification",
		Action: func(h *integration.Harness) error {
			h.SendKeys("a") // Accept key
			return nil
		},
	})
	return b
}

// SkipTransaction adds a step to skip the current transaction.
func (b *Builder) SkipTransaction() *Builder {
	b.steps = append(b.steps, Step{
		Name: "Skip transaction",
		Action: func(h *integration.Harness) error {
			h.SendKeys("s") // Skip key
			return nil
		},
	})
	return b
}

// StartBatchMode adds a step to enter batch classification mode.
func (b *Builder) StartBatchMode() *Builder {
	b.steps = append(b.steps, Step{
		Name: "Start batch mode",
		Action: func(h *integration.Harness) error {
			h.SendKeys("b") // Batch key
			return h.WaitForState(tui.StateBatch, 1*time.Second)
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertCurrentState(t, tui.StateBatch)
			},
		},
	})
	return b
}

// SelectVisualRange adds steps for visual selection.
func (b *Builder) SelectVisualRange(startIndex, endIndex int) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Select visual range %d-%d", startIndex, endIndex),
		Action: func(h *integration.Harness) error {
			// Go to start position
			h.SendKeys("g") // Go to top
			for i := 0; i < startIndex; i++ {
				h.SendKeys("j")
			}

			// Enter visual mode
			h.SendKeys("v")

			// Select to end position
			for i := startIndex; i < endIndex; i++ {
				h.SendKeys("j")
			}

			return nil
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertVisualMode(t, endIndex-startIndex+1)
			},
		},
	})
	return b
}

// SearchFor adds a step to search for transactions.
func (b *Builder) SearchFor(query string) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Search for: %s", query),
		Action: func(h *integration.Harness) error {
			h.SendKeys("/") // Search key
			time.Sleep(100 * time.Millisecond)

			// Type the search query
			for _, char := range query {
				h.SendKeys(string(char))
			}

			h.SendKeys("enter")
			return nil
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertSearchMode(t, query)
			},
		},
	})
	return b
}

// AssertNotification adds an assertion for a notification.
func (b *Builder) AssertNotification(msgType, content string) *Builder {
	b.steps = append(b.steps, Step{
		Name: "Check notification",
		Action: func(_ *integration.Harness) error {
			return nil // Just assertion
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertNotification(t, msgType, content)
			},
		},
	})
	return b
}

// AssertClassificationSaved adds an assertion for saved classification.
func (b *Builder) AssertClassificationSaved(txnID, category string) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Verify classification saved: %s -> %s", txnID, category),
		Action: func(_ *integration.Harness) error {
			return nil // Just assertion
		},
		Assertions: []func(*testing.T, *integration.Assertions){
			func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertTransactionClassified(t, txnID, category)
			},
		},
	})
	return b
}

// WaitFor adds a delay step.
func (b *Builder) WaitFor(duration time.Duration) *Builder {
	b.steps = append(b.steps, Step{
		Name: fmt.Sprintf("Wait %v", duration),
		Action: func(_ *integration.Harness) error {
			time.Sleep(duration)
			return nil
		},
	})
	return b
}

// Custom adds a custom step.
func (b *Builder) Custom(name string, action func(*integration.Harness) error) *Builder {
	b.steps = append(b.steps, Step{
		Name:   name,
		Action: action,
	})
	return b
}

// AssertCustom adds a custom assertion step.
func (b *Builder) AssertCustom(name string, assertion func(*testing.T, *integration.Assertions)) *Builder {
	b.steps = append(b.steps, Step{
		Name: name,
		Action: func(_ *integration.Harness) error {
			return nil
		},
		Assertions: []func(*testing.T, *integration.Assertions){assertion},
	})
	return b
}

// Execute runs the workflow.
func (b *Builder) Execute() {
	b.t.Helper()

	if b.name != "" {
		b.t.Run(b.name, func(t *testing.T) {
			b.executeSteps(t)
		})
	} else {
		b.executeSteps(b.t)
	}
}

func (b *Builder) executeSteps(t *testing.T) {
	t.Helper()

	// Set up timeout
	done := make(chan bool)
	timeout := time.NewTimer(b.timeout)
	defer timeout.Stop()

	go func() {
		for i, step := range b.steps {
			// Log step
			t.Logf("Step %d/%d: %s", i+1, len(b.steps), step.Name)

			// Execute action
			if step.Action != nil {
				if err := step.Action(b.harness); err != nil {
					if !step.SkipOnError {
						require.NoError(t, err, "step failed: %s", step.Name)
					} else {
						t.Logf("Step failed (continuing): %s - %v", step.Name, err)
					}
				}
			}

			// Run assertions
			for _, assertion := range step.Assertions {
				assertion(t, b.assertions)
			}

			// Small delay between steps for UI updates
			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-timeout.C:
		t.Fatalf("workflow timed out after %v", b.timeout)
	}
}

// CommonWorkflows provides pre-built workflows.
type CommonWorkflows struct{}

// ClassifySingleTransaction creates a workflow for classifying one transaction.
func (CommonWorkflows) ClassifySingleTransaction(txnID string, categoryNum int) func(*testing.T, *integration.Harness) {
	return func(t *testing.T, h *integration.Harness) {
		t.Helper()
		NewBuilder(t, h).
			WithName("Classify single transaction").
			NavigateToTransaction(txnID).
			StartClassification().
			SelectCategorySuggestion(categoryNum).
			AcceptClassification().
			AssertNotification("success", "Classification saved").
			Execute()
	}
}

// BatchClassifyTransactions creates a workflow for batch classification.
func (CommonWorkflows) BatchClassifyTransactions(txnIDs []string, category string) func(*testing.T, *integration.Harness) {
	return func(t *testing.T, h *integration.Harness) {
		t.Helper()
		builder := NewBuilder(t, h).
			WithName("Batch classify transactions").
			StartBatchMode()

		// Select transactions
		for i, txnID := range txnIDs {
			builder = builder.Custom(fmt.Sprintf("Select transaction %d", i+1), func(h *integration.Harness) error {
				return h.SelectTransaction(txnID)
			})
		}

		// Apply category
		builder.
			Custom("Apply category to batch", func(h *integration.Harness) error {
				h.SendKeys("c") // Category key in batch mode
				for _, char := range category {
					h.SendKeys(string(char))
				}
				h.SendKeys("enter")
				return nil
			}).
			AssertNotification("success", fmt.Sprintf("%d transactions classified", len(txnIDs))).
			Execute()
	}
}

// SearchAndClassify creates a workflow for searching and classifying.
func (CommonWorkflows) SearchAndClassify(searchQuery string, categoryNum int) func(*testing.T, *integration.Harness) {
	return func(t *testing.T, h *integration.Harness) {
		t.Helper()
		NewBuilder(t, h).
			WithName("Search and classify").
			SearchFor(searchQuery).
			WaitFor(200 * time.Millisecond).
			StartClassification().
			SelectCategorySuggestion(categoryNum).
			AcceptClassification().
			Execute()
	}
}
