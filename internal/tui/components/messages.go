package components

import "github.com/Veraticus/the-spice-must-flow/internal/model"

// TransactionDetailsRequestMsg requests to show transaction details.
type TransactionDetailsRequestMsg struct {
	Classification *model.Classification
	Transaction    model.Transaction
}

// BackToListMsg requests to go back to the transaction list.
type BackToListMsg struct{}

// ManualClassificationRequestMsg requests manual classification.
type ManualClassificationRequestMsg struct {
	Transaction model.Transaction
}

// AIClassificationRequestMsg requests AI classification.
type AIClassificationRequestMsg struct {
	Transaction model.Transaction
}

// StartEngineClassificationMsg starts engine-driven classification.
type StartEngineClassificationMsg struct{}

// ShowHelpMsg shows the help screen.
type ShowHelpMsg struct{}
