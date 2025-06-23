package tui

import (
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Request messages from engine.
type classificationRequestMsg struct {
	pending model.PendingClassification
	single  bool
}

type batchClassificationRequestMsg struct {
	pending []model.PendingClassification
}

type directionRequestMsg struct {
	pending engine.PendingDirection
}

// Data loading messages.
type dataLoadedMsg struct {
	err      error
	dataType string
	count    int
}

type transactionsLoadedMsg struct {
	err          error
	transactions []model.Transaction
}

type categoriesLoadedMsg struct {
	err        error
	categories []model.Category
}

type checkPatternsLoadedMsg struct {
	err      error
	patterns []model.CheckPattern
}

// UI interaction messages.
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

// Async operation messages.
type aiSuggestionMsg struct {
	err           error
	transactionID string
	rankings      model.CategoryRankings
}

type searchResultsMsg struct {
	query   string
	results []SearchResult
}

// State transition messages.
type switchViewMsg struct {
	view View
}

type undoRequestMsg struct{}

type exportRequestMsg struct {
	format ExportFormat
}

// Error handling.
type errorMsg struct {
	err     error
	context string
}

// Direction type for navigation.
type Direction int

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
	DirectionPageUp
	DirectionPageDown
	DirectionHome
	DirectionEnd
)

// SearchResult represents a search match.
type SearchResult struct {
	TransactionID string
	MerchantName  string
	Date          string
	MatchType     string
	Amount        float64
	Score         float64
}

// ExportFormat represents available export formats.
type ExportFormat string

const (
	ExportFormatCSV   ExportFormat = "csv"
	ExportFormatJSON  ExportFormat = "json"
	ExportFormatExcel ExportFormat = "xlsx"
)
