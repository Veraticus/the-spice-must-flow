// Package categories provides a comprehensive test infrastructure for managing
// categories in tests. It emphasizes type safety, elegant APIs, and proper test isolation.
//
// # Overview
//
// The package provides three main components:
//
// 1. **Builder Pattern**: A fluent API for constructing test categories
// 2. **Fixtures**: Predefined category sets for common test scenarios
// 3. **Type Safety**: Strongly-typed category names preventing string errors
//
// # Basic Usage
//
// The simplest way to set up a test with categories:
//
//	func TestMyFeature(t *testing.T) {
//		db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
//			return b.WithBasicCategories()
//		})
//
//		// Use db.Storage for your test...
//	}
//
// # Using Fixtures
//
// Fixtures provide consistent category sets:
//
//	func TestComplexScenario(t *testing.T) {
//		db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
//			return b.WithFixture(categories.FixtureStandard)
//		})
//
//		// Categories from FixtureStandard are now available
//	}
//
// # Custom Categories
//
// Add specific categories for your test:
//
//	func TestVendorRules(t *testing.T) {
//		db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
//			return b.
//				WithBasicCategories().
//				WithCategory("Special Category").
//				WithCategories("Cat1", "Cat2", "Cat3")
//		})
//
//		// Access categories via db.Categories
//		groceries := db.Categories.MustFind(t, categories.CategoryGroceries)
//	}
//
// # Category Constants
//
// The package provides strongly-typed constants for common categories:
//
//	categories.CategoryGroceries      // "Groceries"
//	categories.CategoryFoodDining     // "Food & Dining"
//	categories.CategoryShopping       // "Shopping"
//	// ... and many more
//
// # Thread Safety
//
// All builders and fixtures are safe for concurrent use. Each test gets its own
// isolated database instance with its own category set.
//
// # Best Practices
//
// 1. Use fixtures for consistency across related tests
// 2. Define custom CategoryName constants for test-specific categories
// 3. Use MustFind/MustGet methods in tests for cleaner failures
// 4. Leverage the builder's method chaining for readable test setup
//
// # Migration from Hardcoded Categories
//
// If you're updating existing tests that assumed hardcoded categories:
//
// Before:
//
//	storage := createTestStorage(t)
//	// Test assumes "Groceries" exists...
//
// After:
//
//	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
//		return b.WithCategory(categories.CategoryGroceries)
//	})
//	storage := db.Storage
//	// "Groceries" is guaranteed to exist
package categories
