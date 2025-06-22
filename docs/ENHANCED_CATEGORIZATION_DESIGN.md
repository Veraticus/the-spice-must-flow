# Enhanced Category Selection & Check Pattern Design

## Overview
This document outlines the design for improving transaction categorization in the-spice-must-flow by:
1. Having the LLM rank ALL categories by likelihood
2. Implementing check pattern matching for better check categorization
3. Providing an enhanced category selection UI with descriptions

## Problem Statement
Currently:
- Checks from Ally Bank have no payee information (just "Check Paid #1234")
- Users must manually categorize every check
- Category selection is a simple text field with no descriptions
- No learning from check patterns (e.g., $100 checks are usually cleaning)

## System Flow

### Classification Decision Tree
```
Transaction arrives
    ↓
Is there a vendor rule match?
    → YES: Auto-classify (100% confidence) ✓
    → NO: Continue ↓
    
Is it a check transaction?
    → YES: Load check patterns
    → NO: Continue ↓
    
Send to LLM with:
- Transaction details
- ALL categories with descriptions
- Check pattern hints (if applicable)
    ↓
LLM returns ranked categories
    ↓
Is top category ≥85% confidence?
    → YES: Auto-classify ✓
    → NO: Show ranked list to user
```

## Database Schema

### Check Patterns Table
```sql
CREATE TABLE check_patterns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_name TEXT NOT NULL,      -- "Monthly cleaning"
    amount_min REAL,                  -- 100.00
    amount_max REAL,                  -- 200.00 (null = exact match)
    check_number_pattern TEXT,        -- JSON: {"modulo": 10, "offset": 5}
    day_of_month_min INTEGER,         -- 1
    day_of_month_max INTEGER,         -- 7
    category TEXT NOT NULL,           -- "Home Services"
    notes TEXT,                       -- "Cleaning service payment"
    confidence_boost REAL DEFAULT 0.3,-- How much to boost LLM confidence
    active BOOLEAN DEFAULT 1,
    use_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_check_patterns_amount ON check_patterns(amount_min, amount_max);
CREATE INDEX idx_check_patterns_active ON check_patterns(active);
```

## LLM Integration

### New Prompt Structure
```go
func (c *Classifier) buildPromptWithRanking(txn model.Transaction, categories []model.Category, checkPatterns []model.CheckPattern) string {
    prompt := `You are a financial transaction classifier. Your task is to rank ALL provided categories by how likely this transaction belongs to each one.

Transaction Details:
%s

%s

Categories to rank:
%s

Instructions:
1. Analyze the transaction and rank EVERY category by likelihood (0.0 to 1.0)
2. The scores should be relative probabilities (they don't need to sum to 1.0)
3. If none of the existing categories fit well, you may suggest ONE new category with score >0.7
4. Return results in this exact format:

RANKINGS:
category_name|score
category_name|score
...

NEW_CATEGORY (only if needed):
name: Category Name
score: 0.75
description: One sentence description of what belongs in this category`

    // Build transaction details (existing code)
    transactionDetails := buildTransactionDetails(txn)
    
    // Add check pattern hints if applicable
    checkHints := ""
    if txn.Type == "CHECK" && len(checkPatterns) > 0 {
        checkHints = "Check Pattern Matches:\n"
        for _, pattern := range checkPatterns {
            checkHints += fmt.Sprintf("- Pattern '%s' suggests category '%s' (based on %d previous uses)\n", 
                pattern.PatternName, pattern.Category, pattern.UseCount)
        }
        checkHints += "\n"
    }
    
    // Build category list with descriptions
    categoryList := ""
    for _, cat := range categories {
        categoryList += fmt.Sprintf("- %s: %s\n", cat.Name, cat.Description)
    }
    
    return fmt.Sprintf(prompt, transactionDetails, checkHints, categoryList)
}
```

### LLM Response Parser
```go
type CategoryRanking struct {
    Category    string
    Score       float64
    IsNew       bool
    Description string // only for new categories
}

func parseLLMRankings(response string) ([]CategoryRanking, error) {
    // Parse the RANKINGS: section
    // Parse the NEW_CATEGORY: section if present
    // Sort by score descending
    // Return ranked list
}
```

## Check Pattern CLI Commands

### Command Structure
```bash
spice checks list                          # List all check patterns
spice checks add                          # Interactive pattern creation
spice checks edit <pattern-id>           # Edit existing pattern
spice checks delete <pattern-id>         # Delete pattern
spice checks test <amount> [--date=...] # Test which patterns match
```

### Example: Creating a Check Pattern
```bash
$ spice checks add

🌶️ Create Check Pattern

Pattern name: Monthly cleaning
Category: Home Services

Amount matching:
  [1] Exact amount
  [2] Range
  [3] Multiple amounts
Choice: 3

Enter amounts (comma-separated): 100, 200

Day of month restriction? [y/N]: n

Notes (optional): Cleaning service payment

✓ Pattern created: "Monthly cleaning"
  Matches checks for $100.00 or $200.00 → Home Services
```

### Example: Listing Patterns
```bash
$ spice checks list

🌶️ Check Patterns

ID  Pattern Name        Amount(s)      Category         Uses  Last Used
──  ─────────────────  ────────────   ──────────────   ────  ──────────
1   Monthly cleaning   $100, $200     Home Services    24    2024-12-15
2   Rent payment       $3,000-$3,100  Housing         12    2024-12-01  
3   Quarterly taxes    $5,000-$6,000  Taxes            4    2024-10-15
4   Weekly allowance   $50            Personal         48    2024-12-20
```

## Enhanced UI Flow

### When Confidence < 85%
```
┌─ Transaction Classification ─────────────────────────┐
│ 🌶️ Check Paid #1195                                 │
│                                                      │
│ Amount: $100.00                                      │
│ Date: Dec 31, 2024                                   │
│                                                      │
│ 🎯 Check pattern match: "Monthly cleaning"           │
│                                                      │
│ Select category (ranked by likelihood):              │
│                                                      │
│ 1. Home Services (72% match) ⭐ matches pattern      │
│    Cleaning, repairs, maintenance, landscaping       │
│                                                      │
│ 2. Personal Care (18% match)                        │
│    Hair, beauty, spa services                        │
│                                                      │
│ 3. Gifts & Donations (8% match)                     │
│    Charitable giving, presents                       │
│                                                      │
│ 4. Shopping (5% match)                              │
│    Clothing, electronics, general retail             │
│                                                      │
│ [... remaining categories ranked ...]                │
│                                                      │
│ [N] Create new category                              │
│                                                      │
│ Enter number or category name:                       │
└─────────────────────────────────────────────────────┘
```

## Implementation TODO List

### Phase 1: Database & Model Layer
- [X] Create migration for `check_patterns` table
- [X] Add `CheckPattern` model to `internal/model/`
- [X] Update service interfaces to include check pattern methods
- [X] Implement check pattern storage methods in SQLiteStorage

### Phase 2: Check Pattern Management CLI
- [X] Create `cmd/spice/checks.go` with main command structure
- [X] Implement `checks list` command with formatted table output
- [X] Implement `checks add` command with interactive prompts
- [X] Implement `checks edit` command
- [X] Implement `checks delete` command with confirmation
- [X] Implement `checks test` command for pattern matching testing
- [X] Add unit tests for all check commands

### Phase 3: LLM Enhancement
- [X] Create `CategoryRanking` struct in model package
- [X] Update `Classifier` interface to support ranking method
- [X] Implement `SuggestCategoryRankings` method in classifier
- [X] Create new prompt builder for ranking all categories
- [X] Implement LLM response parser for rankings format
- [X] Update existing `SuggestCategory` to use rankings internally
- [X] Add comprehensive tests for ranking functionality

### Phase 4: Check Pattern Integration
- [X] Add `GetMatchingPatterns` method to storage layer
- [X] Integrate pattern matching into classification engine
- [X] Modify LLM prompt to include pattern hints
- [X] Add pattern confidence boosting logic
- [X] Update pattern use counts when patterns match
- [X] Add tests for pattern matching logic

### Phase 5: Enhanced UI/UX
- [X] Refactor `promptCustomCategory` to `promptCategorySelection`
- [X] Implement ranked category display with descriptions
- [X] Add pattern match indicators in UI
- [X] Support both number and name input for category selection
- [X] Add "New Category" option with description generation
- [X] Update progress tracking for new workflow
- [X] Add comprehensive UI tests

### Phase 6: Integration & Testing
- [X] Update classification engine to use new ranking system
- [X] Ensure vendor rules still bypass ranking when matched
- [X] Test auto-classification threshold (85% confidence)
- [X] Add integration tests for full workflow
- [X] Test check pattern matching with real data
- [X] Performance test ranking all categories

### Phase 7: Documentation & Polish
- [ ] Update README with check pattern examples
- [ ] Add check pattern documentation to CLAUDE.md
- [ ] Create user guide for check patterns
- [ ] Add migration guide for existing users
- [ ] Polish all user-facing messages
- [ ] Final testing pass

## Success Criteria
1. Check patterns reduce manual categorization by >50% for check transactions
2. Category selection shows all options with descriptions
3. LLM confidence threshold prevents incorrect auto-classifications
4. Pattern management CLI is intuitive and functional
5. No performance regression from ranking all categories

## Future Enhancements
1. Pattern learning - automatically suggest patterns based on user behavior
2. Pattern templates - pre-built patterns for common check types
3. Pattern sharing - export/import patterns between users
4. Advanced matching - regex patterns, payee guessing
5. Mobile/web UI for pattern management
