package integration

import (
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// testModel wraps the TUI prompter for testing.
type testModel struct {
	harness  *Harness
	prompter *tui.Prompter
	config   tui.Config
	model    tui.Model
}

// Init implements tea.Model.
func (m *testModel) Init() tea.Cmd {
	// The model is already initialized through the prompter
	return m.model.Init()
}

// Update implements tea.Model.
func (m *testModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Update the underlying model
	newModel, cmd := m.model.Update(msg)
	tuiModel, ok := newModel.(tui.Model)
	if !ok {
		// This should never happen, but handle it gracefully
		panic("unexpected model type in testModel.Update")
	}
	m.model = tuiModel
	return m, cmd
}

// View implements tea.Model.
func (m *testModel) View() string {
	return m.model.View()
}

// instrumentedModel wraps a model to capture state changes.
type instrumentedModel struct {
	base    tea.Model
	harness *Harness
}

// Init implements tea.Model.
func (m *instrumentedModel) Init() tea.Cmd {
	return m.base.Init()
}

// Update implements tea.Model with instrumentation.
func (m *instrumentedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Call before hook
	if hook, ok := m.harness.hooks[HookBeforeUpdate]; ok {
		hook(m.harness, msg)
	}

	// Update base model
	newBase, cmd := m.base.Update(msg)
	m.base = newBase

	// Track state changes
	if tuiModel, ok := m.extractTUIModel(); ok {
		select {
		case m.harness.stateChan <- tuiModel.State():
		default:
		}

		select {
		case m.harness.viewChan <- tuiModel.View():
		default:
		}
	}

	// Call after hook
	if hook, ok := m.harness.hooks[HookAfterUpdate]; ok {
		hook(m.harness, msg)
	}

	return m, cmd
}

// View implements tea.Model with instrumentation.
func (m *instrumentedModel) View() string {
	view := m.base.View()

	// Update render tracking
	m.harness.mu.Lock()
	m.harness.renderCount++
	m.harness.lastRenderTime = time.Now()
	m.harness.screenHistory = append(m.harness.screenHistory, view)

	// Keep history bounded
	if len(m.harness.screenHistory) > 100 {
		m.harness.screenHistory = m.harness.screenHistory[1:]
	}
	m.harness.mu.Unlock()

	// Send to output channel if not closed
	m.harness.mu.Lock()
	if !m.harness.closed {
		select {
		case m.harness.outputChan <- view:
		default:
		}
	}
	m.harness.mu.Unlock()

	// Call render hook
	if hook, ok := m.harness.hooks[HookOnRender]; ok {
		hook(m.harness, nil)
	}

	return view
}

// extractTUIModel attempts to extract the TUI model for state inspection.
func (m *instrumentedModel) extractTUIModel() (*tuiModelAccessor, bool) {
	// Try to extract the TUI model through type assertion
	if testModel, ok := m.base.(*testModel); ok {
		return &tuiModelAccessor{model: testModel.model}, true
	}

	return nil, false
}

// tuiModelAccessor provides access to TUI model internals.
type tuiModelAccessor struct {
	model tui.Model
}

// State returns the current TUI state.
func (a *tuiModelAccessor) State() tui.State {
	return a.model.State()
}

// View returns the current TUI view.
func (a *tuiModelAccessor) View() tui.View {
	return a.model.GetView()
}
