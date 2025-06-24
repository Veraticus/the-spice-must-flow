package viewmodel

// AppState represents the overall application state.
type AppState int

const (
	// StateLoading indicates the application is loading data.
	StateLoading AppState = iota
	// StateClassifying indicates the user is classifying transactions.
	StateClassifying
	// StateStats indicates the user is viewing statistics.
	StateStats
	// StateError indicates an error has occurred.
	StateError
	// StateWaiting indicates the application is waiting for user input.
	StateWaiting
)

// AppView represents the entire application view model.
type AppView struct {
	Classifier      *ClassifierView
	Batch           *BatchView
	TransactionList *TransactionListView
	Stats           *StatsView
	Error           string
	StatusMessage   string
	KeyBindings     []KeyBinding
	ActiveComponent ComponentType
	State           AppState
	Width           int
	Height          int
	ShowHelp        bool
}

// ComponentType identifies which component is active.
type ComponentType int

const (
	// ComponentClassifier indicates the classifier component is active.
	ComponentClassifier ComponentType = iota
	// ComponentBatch indicates the batch classifier is active.
	ComponentBatch
	// ComponentTransactionList indicates the transaction list is active.
	ComponentTransactionList
	// ComponentStats indicates the statistics view is active.
	ComponentStats
	// ComponentHelp indicates the help overlay is active.
	ComponentHelp
)

// KeyBinding represents a keyboard shortcut.
type KeyBinding struct {
	Key         string
	Description string
	IsActive    bool
}

// Dimensions represents size constraints.
type Dimensions struct {
	Width  int
	Height int
}

// Position represents a position in the UI.
type Position struct {
	Row int
	Col int
}

// IsReady returns true if the application is ready for user interaction.
func (av AppView) IsReady() bool {
	return av.State != StateLoading && av.State != StateError
}

// HasError returns true if the application has a global error.
func (av AppView) HasError() bool {
	return av.Error != ""
}

// GetActiveKeyBindings returns only the currently active key bindings.
func (av AppView) GetActiveKeyBindings() []KeyBinding {
	var active []KeyBinding
	for _, kb := range av.KeyBindings {
		if kb.IsActive {
			active = append(active, kb)
		}
	}
	return active
}
