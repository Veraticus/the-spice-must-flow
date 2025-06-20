# TODO: Dynamic Category System - Implementation Status

## ✅ Completed

The dynamic categorization system has been successfully implemented! Categories are now:
- Stored in the database (no hardcoded lists)
- Created dynamically based on AI suggestions and user approval
- Suggested by the LLM when confidence < 90%
- Fully under user control

### What Was Done:
1. ✅ Updated LLM classifier to return `isNew` flag based on confidence
2. ✅ Modified CLI prompter to handle new category suggestions with special UI
3. ✅ Added `ensureCategoryExists` to engine for automatic category creation
4. ✅ Removed all seeded categories from migrations (clean slate approach)
5. ✅ Added comprehensive test coverage for new category flows

## 🚧 Next Steps

A comprehensive testing infrastructure needs to be built to handle the dynamic category system in tests. See `TESTING_CATEGORIES_PROMPT.md` for detailed requirements.

### Original Implementation Plan (For Reference)

## Phase 1: Database Schema Changes

### 1.1 Create Categories Table Migration
- [ ] Create new migration file: `internal/storage/migrations/003_add_categories.go`
- [ ] Add categories table:
  ```sql
  CREATE TABLE IF NOT EXISTS categories (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      name TEXT UNIQUE NOT NULL,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      is_active BOOLEAN DEFAULT 1
  );
  ```
- [ ] Add foreign key constraint to vendors table (or handle in code if SQLite doesn't enforce)
- [ ] Add index on categories.name for performance

### 1.2 Seed Initial Categories
- [ ] Create seed data migration or initialization function
- [ ] Basic categories to seed:
  - Food & Dining
  - Transportation
  - Shopping
  - Utilities
  - Entertainment
  - Other Expenses

## Phase 2: Update Storage Layer

### 2.1 Add Category Methods to Storage Interface
- [ ] Update `internal/service/interfaces.go` to add:
  ```go
  GetCategories(ctx context.Context) ([]model.Category, error)
  GetCategoryByName(ctx context.Context, name string) (*model.Category, error)
  CreateCategory(ctx context.Context, name string) (*model.Category, error)
  ```

### 2.2 Implement Category Methods in SQLite Storage
- [ ] Implement `GetCategories()` - return all active categories
- [ ] Implement `GetCategoryByName()` - check if category exists
- [ ] Implement `CreateCategory()` - create new category
- [ ] Add category validation in `SaveVendor()` - ensure category exists
- [ ] Add category validation in `SaveClassification()` - ensure category exists

### 2.3 Update Model
- [ ] Add `Category` struct to `internal/model/category.go`:
  ```go
  type Category struct {
      ID        int
      Name      string
      CreatedAt time.Time
      IsActive  bool
  }
  ```

## Phase 3: Update LLM Classifier

### 3.1 Update LLM Interface
- [ ] Modify `internal/service/interfaces.go` LLMClassifier:
  ```go
  SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (category string, confidence float64, isNew bool, err error)
  ```

### 3.2 Update LLM Implementation
- [ ] Remove hardcoded category list from `internal/llm/classifier.go`
- [ ] Update `buildPrompt()` to accept categories as parameter
- [ ] Modify prompt to:
  - Include dynamic category list
  - Request confidence score (0.0-1.0)
  - Allow suggesting new categories when confidence < 0.9
- [ ] Update response parsing to extract:
  - Category name
  - Confidence score
  - Whether it's a new suggestion (based on confidence)

### 3.3 Update Prompt Template
- [ ] New prompt structure:
  ```
  Categorize this transaction into one of these existing categories:
  [Dynamic list of categories]
  
  If confidence >= 90%, respond:
  CATEGORY: <existing category>
  CONFIDENCE: <0.90-1.0>
  
  If confidence < 90%, suggest a new category:
  CATEGORY: <new category suggestion>
  CONFIDENCE: <0.0-0.89>
  NEW: true
  ```

## Phase 4: Update Classification Engine

### 4.1 Modify Classification Flow
- [ ] Update `internal/engine/classifier.go`:
  - [ ] Load categories before calling LLM
  - [ ] Pass categories to LLM
  - [ ] Handle low confidence suggestions

### 4.2 Update User Interaction for New Categories
- [ ] Modify `internal/cli/prompter.go` to handle new category flow:
  ```
  Low confidence match (72%)
  Suggested NEW category: Fitness & Health
  [N] Create new category
  [E] Pick from existing  
  [C] Enter custom name
  ```
- [ ] Implement category creation flow
- [ ] Show existing categories when user chooses [E]

### 4.3 Update Batch Review
- [ ] Handle new category suggestions in batch mode
- [ ] Allow creating category that applies to all in batch

## Phase 5: Update CLI Commands

### 5.1 Add Category Management Commands
- [ ] Create `cmd/spice/categories.go`
- [ ] Implement subcommands:
  - [ ] `spice category list` - show all categories
  - [ ] `spice category add <name>` - manually add category
  - [ ] `spice category disable <name>` - soft delete
  - [ ] `spice category stats` - show usage statistics

### 5.2 Update Classify Command
- [ ] Remove any hardcoded category references
- [ ] Add category count to startup message
- [ ] Handle empty category list gracefully

## Phase 6: Testing

### 6.1 Unit Tests
- [ ] Test category CRUD operations
- [ ] Test LLM with confidence scores
- [ ] Test new category flow
- [ ] Test category validation in vendor/classification saves

### 6.2 Integration Tests
- [ ] Test full classification flow with new categories
- [ ] Test category creation during classification
- [ ] Test with empty category list
- [ ] Test with many categories (performance)

### 6.3 Manual Testing Scenarios
- [ ] First run with no categories
- [ ] Classification with high confidence match
- [ ] Classification with low confidence (new category)
- [ ] Batch review with new category
- [ ] Category management commands

## Phase 7: Migration & Cleanup

### 7.1 Remove Old Code
- [ ] Remove hardcoded category list from LLM classifier
- [ ] Remove any category validation against fixed list
- [ ] Update any tests using hardcoded categories

### 7.2 Documentation Updates
- [ ] Update README with new category system
- [ ] Update ARCHITECTURE.md if needed
- [ ] Add examples of category evolution

## Implementation Order

1. **Start with Phase 1-2**: Database and storage layer (foundation)
2. **Then Phase 3**: LLM updates (core logic)
3. **Then Phase 4**: Engine and UI updates (user experience)
4. **Then Phase 5**: CLI commands (management tools)
5. **Then Phase 6**: Testing (validation)
6. **Finally Phase 7**: Cleanup (polish)

## Configuration Considerations

### Confidence Threshold
- [ ] Add to config: `classification.new_category_threshold: 0.9`
- [ ] Make configurable but default to 90%

### Category Limits
- [ ] Consider max categories limit (e.g., 100)
- [ ] Add warning if too many categories

## Edge Cases to Handle

- [ ] Empty category list on first run
- [ ] User cancels new category creation
- [ ] Category name conflicts (case sensitivity)
- [ ] Very long category names
- [ ] Special characters in category names
- [ ] Migrating existing data (if any)

## Success Criteria

- [ ] Categories dynamically created based on usage
- [ ] No hardcoded category lists
- [ ] LLM confidence drives new category suggestions
- [ ] Clean user experience for category management
- [ ] All tests passing
- [ ] No regressions in existing functionality