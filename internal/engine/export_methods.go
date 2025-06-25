package engine

import (
	"context"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// GetTransactionsToClassify exposes the method for batch classification.
func (e *ClassificationEngine) GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error) {
	// For batch mode, we don't support resume - just get all unclassified transactions
	return e.storage.GetTransactionsToClassify(ctx, fromDate)
}

// GroupByMerchant exposes the grouping method.
func (e *ClassificationEngine) GroupByMerchant(transactions []model.Transaction) map[string][]model.Transaction {
	return e.groupByMerchant(transactions)
}

// SortMerchantsByVolume exposes the merchant sorting method.
func (e *ClassificationEngine) SortMerchantsByVolume(merchantGroups map[string][]model.Transaction) []string {
	return e.sortMerchantsByVolume(merchantGroups)
}

// GetCategories exposes the category getter.
func (e *ClassificationEngine) GetCategories(ctx context.Context) ([]model.Category, error) {
	return e.storage.GetCategories(ctx)
}

// GetVendor exposes the vendor getter.
func (e *ClassificationEngine) GetVendor(ctx context.Context, merchant string) (*model.Vendor, error) {
	return e.getVendor(ctx, merchant)
}

// GetMatchingCheckPatterns exposes check pattern matching.
func (e *ClassificationEngine) GetMatchingCheckPatterns(ctx context.Context, transaction model.Transaction) ([]model.CheckPattern, error) {
	return e.storage.GetMatchingCheckPatterns(ctx, transaction)
}

// FilterCategoriesByDirection exposes the category filtering.
func (e *ClassificationEngine) FilterCategoriesByDirection(categories []model.Category, transactions []model.Transaction) []model.Category {
	return e.filterCategoriesByDirection(categories, transactions)
}

// GetClassifier exposes the classifier interface.
func (e *ClassificationEngine) GetClassifier() Classifier {
	return e.classifier
}

// SaveClassification exposes the save method.
func (e *ClassificationEngine) SaveClassification(ctx context.Context, classification *model.Classification) error {
	return e.storage.SaveClassification(ctx, classification)
}

// SaveVendor exposes the vendor save method.
func (e *ClassificationEngine) SaveVendor(ctx context.Context, vendor *model.Vendor) error {
	return e.storage.SaveVendor(ctx, vendor)
}
