// Package model defines the core domain models used throughout the application.
package model

import "time"

// ClassificationStatus indicates how a transaction was categorized.
type ClassificationStatus string

// Classification status constants.
const (
	StatusUnclassified     ClassificationStatus = "UNCLASSIFIED"
	StatusClassifiedByRule ClassificationStatus = "CLASSIFIED_BY_RULE"
	StatusClassifiedByAI   ClassificationStatus = "CLASSIFIED_BY_AI"
	StatusUserModified     ClassificationStatus = "USER_MODIFIED"
)

// Classification represents a transaction after processing.
type Classification struct {
	ClassifiedAt time.Time
	Category     string
	Status       ClassificationStatus
	Notes        string
	Transaction  Transaction
	Confidence   float64
}

// PendingClassification represents a transaction awaiting user confirmation.
type PendingClassification struct {
	SuggestedCategory   string
	CategoryDescription string
	CategoryRankings    CategoryRankings
	AllCategories       []Category
	CheckPatterns       []CheckPattern
	Transaction         Transaction
	Confidence          float64
	SimilarCount        int
	IsNewCategory       bool
}
