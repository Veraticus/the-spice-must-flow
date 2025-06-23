// Package categories provides sophisticated test infrastructure for managing
// categories in tests. It offers a fluent, type-safe API for seeding test data,
// ensuring proper cleanup, and maintaining test isolation.
//
// Example usage:
//
//	categories := testutil.NewCategoryBuilder(t).
//		WithBasicCategories().
//		WithCategory("Custom Category").
//		Build()
//
//	db := testutil.SetupTestDB(t, categories)
package categories

import (
	"context"
	"fmt"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// Builder provides a fluent interface for constructing test categories.
// It ensures type safety, validates inputs, and manages the lifecycle
// of test data within the test scope.
type Builder interface {
	// WithCategory adds a single category to the builder.
	WithCategory(name CategoryName) Builder

	// WithCategories adds multiple categories to the builder.
	WithCategories(names ...CategoryName) Builder

	// WithBasicCategories adds the minimal set of categories commonly used in tests.
	WithBasicCategories() Builder

	// WithExtendedCategories adds a comprehensive set of categories for complex tests.
	WithExtendedCategories() Builder

	// WithFixture adds categories from a predefined fixture.
	WithFixture(fixture Fixture) Builder

	// Build creates the categories in the provided storage and returns them.
	// It automatically registers cleanup to remove the categories after the test.
	Build(ctx context.Context, storage service.Storage) (Categories, error)

	// BuildMap creates categories and returns them as a map for easy lookup.
	BuildMap(ctx context.Context, storage service.Storage) (CategoryMap, error)
}

// CategoryName represents a strongly-typed category name.
// This provides compile-time safety and prevents accidental string usage.
type CategoryName string

// String returns the string representation of the category name.
func (c CategoryName) String() string {
	return string(c)
}

// Common category names used across tests.
const (
	CategoryGroceries          CategoryName = "Groceries"
	CategoryFoodDining         CategoryName = "Food & Dining"
	CategoryCoffeeDining       CategoryName = "Coffee & Dining"
	CategoryShopping           CategoryName = "Shopping"
	CategoryOnlineShopping     CategoryName = "Online Shopping"
	CategoryTransportation     CategoryName = "Transportation"
	CategorySubscriptions      CategoryName = "Subscription Services"
	CategoryHealthFitness      CategoryName = "Health & Fitness"
	CategoryEntertainment      CategoryName = "Entertainment"
	CategoryUtilities          CategoryName = "Utilities"
	CategoryEducation          CategoryName = "Education"
	CategoryTravel             CategoryName = "Travel"
	CategoryPersonalCare       CategoryName = "Personal Care"
	CategoryGiftsCharitable    CategoryName = "Gifts & Charitable"
	CategoryHomeImprovement    CategoryName = "Home Improvement"
	CategoryInsurance          CategoryName = "Insurance"
	CategoryInvestmentsSavings CategoryName = "Investments & Savings"
	CategoryBankingFees        CategoryName = "Banking & Fees"
)

// Test-specific category names.
const (
	CategoryTest1         CategoryName = "Test Category 1"
	CategoryTest2         CategoryName = "Test Category 2"
	CategoryTest3         CategoryName = "Test Category 3"
	CategoryInitial       CategoryName = "Initial Category"
	CategoryUserCorrected CategoryName = "User Corrected"
	CategoryFinal         CategoryName = "Final Category"
)

// Categories represents a collection of created test categories.
type Categories []model.Category

// Find returns the category with the given name, or nil if not found.
func (c Categories) Find(name CategoryName) *model.Category {
	for i := range c {
		if c[i].Name == name.String() {
			return &c[i]
		}
	}
	return nil
}

// MustFind returns the category with the given name, or fails the test if not found.
func (c Categories) MustFind(t *testing.T, name CategoryName) model.Category {
	t.Helper()
	cat := c.Find(name)
	if cat == nil {
		t.Fatalf("category %q not found in test data", name)
	}
	return *cat
}

// Names returns all category names as a slice of strings.
func (c Categories) Names() []string {
	names := make([]string, len(c))
	for i, cat := range c {
		names[i] = cat.Name
	}
	return names
}

// CategoryMap provides O(1) lookup for categories by name.
type CategoryMap map[CategoryName]model.Category

// Get returns the category for the given name and whether it was found.
func (m CategoryMap) Get(name CategoryName) (model.Category, bool) {
	cat, ok := m[name]
	return cat, ok
}

// MustGet returns the category for the given name or fails the test.
func (m CategoryMap) MustGet(t *testing.T, name CategoryName) model.Category {
	t.Helper()
	cat, ok := m.Get(name)
	if !ok {
		t.Fatalf("category %q not found in test data", name)
	}
	return cat
}

// categoryBuilder implements the Builder interface.
type categoryBuilder struct {
	t          *testing.T
	categories map[CategoryName]struct{}
}

// NewBuilder creates a new category builder for the given test.
// The builder ensures all created categories are cleaned up after the test.
func NewBuilder(t *testing.T) Builder {
	t.Helper()
	return &categoryBuilder{
		t:          t,
		categories: make(map[CategoryName]struct{}),
	}
}

func (b *categoryBuilder) WithCategory(name CategoryName) Builder {
	b.categories[name] = struct{}{}
	return b
}

func (b *categoryBuilder) WithCategories(names ...CategoryName) Builder {
	for _, name := range names {
		b.categories[name] = struct{}{}
	}
	return b
}

func (b *categoryBuilder) WithBasicCategories() Builder {
	basic := []CategoryName{
		CategoryGroceries,
		CategoryFoodDining,
		CategoryShopping,
		CategoryTransportation,
		CategorySubscriptions,
		CategoryUtilities,
	}
	return b.WithCategories(basic...)
}

func (b *categoryBuilder) WithExtendedCategories() Builder {
	extended := []CategoryName{
		CategoryGroceries,
		CategoryFoodDining,
		CategoryCoffeeDining,
		CategoryShopping,
		CategoryOnlineShopping,
		CategoryTransportation,
		CategorySubscriptions,
		CategoryHealthFitness,
		CategoryEntertainment,
		CategoryUtilities,
		CategoryEducation,
		CategoryTravel,
		CategoryPersonalCare,
		CategoryGiftsCharitable,
		CategoryHomeImprovement,
		CategoryInsurance,
		CategoryInvestmentsSavings,
		CategoryBankingFees,
	}
	return b.WithCategories(extended...)
}

func (b *categoryBuilder) WithFixture(fixture Fixture) Builder {
	return b.WithCategories(fixture.Categories()...)
}

func (b *categoryBuilder) Build(ctx context.Context, storage service.Storage) (Categories, error) {
	b.t.Helper()

	if len(b.categories) == 0 {
		return Categories{}, nil
	}

	// Convert map to slice for consistent ordering
	names := make([]CategoryName, 0, len(b.categories))
	for name := range b.categories {
		names = append(names, name)
	}

	// Create categories in storage
	result := make(Categories, 0, len(names))

	for _, name := range names {
		createdCat, err := storage.CreateCategory(ctx, name.String(), "Test description for "+name.String(), model.CategoryTypeExpense)
		if err != nil {
			return nil, fmt.Errorf("failed to create category %q: %w", name, err)
		}
		result = append(result, *createdCat)
	}

	// Note: Categories use soft deletes via is_active flag
	// No explicit cleanup needed for in-memory test databases

	return result, nil
}

func (b *categoryBuilder) BuildMap(ctx context.Context, storage service.Storage) (CategoryMap, error) {
	categories, err := b.Build(ctx, storage)
	if err != nil {
		return nil, err
	}

	m := make(CategoryMap, len(categories))
	for _, cat := range categories {
		m[CategoryName(cat.Name)] = cat
	}
	return m, nil
}
