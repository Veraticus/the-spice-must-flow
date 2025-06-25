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

// UI interaction messages - removed unused types

// Error handling.
type errorMsg struct {
	err error
}

// showMessageMsg displays a message to the user.
type showMessageMsg struct {
	message string
}

// notificationMsg displays a notification to the user.
type notificationMsg struct {
	content     string
	messageType string // "info", "success", "error"
}

// Removed unused types: Direction, SearchResult, ExportFormat
