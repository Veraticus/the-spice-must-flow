# CLAUDE.md - the-spice-must-flow

This file provides project-specific guidance that complements ~/.claude/CLAUDE.md.

## Project Overview

the-spice-must-flow is a personal finance categorization engine that ingests financial transactions from Plaid, OFX/QFX files, and SimpleFin, uses AI to intelligently categorize them, and exports reports to Google Sheets. Written in idiomatic Go with emphasis on testability, extensibility, and delightful CLI experience.

**Module**: `github.com/Veraticus/the-spice-must-flow`

## Critical Development Commands

```bash
# ALWAYS run after implementing features:
make test && make lint

# Full validation before commits:
make test-all  # includes race detection, integration tests

# Auto-fix common issues:
make fix

# Build the CLI:
make build

# Database debugging (on nix systems):
nix-shell -p sqlite --run 'sqlite3 ~/.local/share/spice/spice.db "SELECT * FROM transactions LIMIT 5;"'

# Common database queries:
# Check for negative amounts (should be 0 after migration 13):
nix-shell -p sqlite --run 'sqlite3 ~/.local/share/spice/spice.db "SELECT COUNT(*) FROM transactions WHERE amount < 0;"'

# View check patterns:
nix-shell -p sqlite --run 'sqlite3 ~/.local/share/spice/spice.db "SELECT * FROM check_patterns;"'

# View categories:
nix-shell -p sqlite --run 'sqlite3 ~/.local/share/spice/spice.db "SELECT id, name, type, description FROM categories ORDER BY name;"'
```

## Project-Specific Ultrathinking Triggers

Use ultrathinking for these the-spice-must-flow challenges:
- **Classification Algorithm Design**: "Let me ultrathink about the optimal transaction classification flow"
- **Pattern Matching Logic**: "I need to ultrathink through the check pattern matching algorithm"
- **Database Migration Strategy**: "Let me ultrathink about backward-compatible schema changes"
- **LLM Prompt Engineering**: "I'll ultrathink on the most effective prompt structure for categorization"

## Agent Spawning Opportunities

Leverage multiple agents for these common tasks:

### Feature Implementation
```
"I'll spawn agents to handle this feature:
- Agent 1: Research existing categorization patterns in internal/engine/
- Agent 2: Analyze the storage layer interfaces in internal/storage/
- Agent 3: Review test patterns in *_test.go files"
```

### Complex Refactoring
```
"Spawning agents for this refactor:
- Agent 1: Map all usages of the old interface
- Agent 2: Design the new interface structure
- Agent 3: Create migration plan for dependent code"
```

### Test Coverage
```
"I'll use multiple agents:
- Agent 1: Write table-driven tests for the happy path
- Agent 2: Create edge case scenarios
- Agent 3: Add integration tests"
```

## Go Idioms - Project Standards

### Interface Design (CRITICAL)
```go
// GOOD - Small, focused interface
type Categorizer interface {
    Categorize(ctx context.Context, txn model.Transaction) (string, error)
}

// BAD - Kitchen sink interface
type Service interface {
    Categorize(...) 
    Store(...)
    Export(...)
    // ... 20 more methods
}
```

### Error Handling Pattern
```go
// ALWAYS wrap errors with context
if err := store.SaveTransaction(ctx, txn); err != nil {
    return fmt.Errorf("failed to save transaction %s: %w", txn.ID, err)
}

// NEVER use custom error types unless absolutely necessary
// NEVER return bare errors without context
```

### Table-Driven Tests (REQUIRED)
```go
func TestCheckPattern_Matches(t *testing.T) {
    tests := []struct {
        name        string
        pattern     CheckPattern
        transaction Transaction
        want        bool
    }{
        {
            name:    "exact amount match",
            pattern: CheckPattern{AmountMin: floatPtr(100.00), AmountMax: floatPtr(100.00)},
            transaction: Transaction{Amount: 100.00, Type: "CHECK"},
            want:    true,
        },
        // ... comprehensive test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.pattern.Matches(tt.transaction)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Static Typing Rules
```go
// AVOID interface{} or any - use concrete types
// Exception: When working with JSON unmarshaling or truly dynamic data

// GOOD
func ProcessTransaction(txn model.Transaction) error

// BAD  
func ProcessTransaction(data interface{}) error

// ACCEPTABLE for JSON parsing
var result map[string]any
err := json.Unmarshal(data, &result)

// GOOD - Explicit type conversion
amount := decimal.NewFromFloat(txn.Amount)

// BAD - Implicit casting assumptions
amount := txn.Amount.(float64) // NEVER DO THIS
```

## Architecture Principles

### Service Layer Pattern
All major components follow interface-first design:
1. Define interface in `internal/service/`
2. Implement in appropriate package
3. Use dependency injection in constructors

### Storage Layer Rules
- ALL database operations go through interfaces
- Transactions use explicit transaction objects
- Migrations are forward-only (no rollbacks)
- Use prepared statements exclusively
- Schema version tracked via SQLite PRAGMA user_version

### Transaction Amount Storage
- **ALWAYS store amounts as absolute values (positive)**
- Use the `direction` field (income/expense/transfer) to indicate money flow
- Never store negative amounts in the database
- All import sources (Plaid, OFX, SimpleFin) must convert to absolute values

### CLI Command Structure
```go
// Each command in its own file: cmd/spice/[command].go
// Shared logic in cmd/spice/[command]_helper.go
// Always use cobra's RunE for error handling
```

## Testing Requirements

### Coverage Standards
- Business logic: >90% coverage REQUIRED
- Storage layer: >85% coverage
- CLI commands: Test business logic, not cobra parsing
- Integration tests: Cover critical paths

### Test Organization
```
package_test.go         # Unit tests
package_integration_test.go  # Integration tests (require external services)
testdata/              # Test fixtures
```

## Common Pitfalls to Avoid

1. **Over-abstracting**: Don't create interfaces with only one implementation
2. **Ignoring context**: ALWAYS pass context.Context as first parameter
3. **Magic numbers**: Use named constants for all numeric values
4. **Skipping validation**: Validate at service boundaries
5. **Forgetting indexes**: Add indexes for all WHERE clause columns

## Performance Patterns

### Batch Operations
```go
// GOOD - Batch inserts
func (s *Storage) SaveTransactions(ctx context.Context, txns []Transaction) error {
    // Use single transaction with prepared statement
}

// BAD - Individual inserts
for _, txn := range txns {
    storage.SaveTransaction(ctx, txn) // N database calls
}
```

### Concurrent Processing
```go
// Use worker pools for parallel processing
// See internal/engine/engine.go for pattern
```

## Security Requirements

1. **Never log sensitive data**: No API keys, tokens, or full account numbers
2. **Validate all inputs**: Especially date ranges and numeric values
3. **Use parameterized queries**: Never concatenate SQL
4. **Sanitize for display**: HTML escape user-provided data

## Database Schema & Migrations

### Migration System
- Migrations defined in `internal/storage/migrations.go`
- Version tracked via SQLite PRAGMA user_version
- Current schema version: 13 (check ExpectedSchemaVersion constant)
- Run migrations: `./spice migrate`
- Check status: `./spice migrate --status`

### Key Tables
- `transactions`: Core transaction data (amounts are ALWAYS positive)
- `classifications`: Transaction categorization data
- `vendors`: Merchant-to-category mappings
- `categories`: Available categories with types (income/expense/system)
- `check_patterns`: Rules for auto-categorizing check transactions

## CLI Commands Overview

### Core Commands
- `spice classify [--batch]`: Categorize transactions (batch mode recommended)
- `spice import`: Import from Plaid
- `spice import-ofx <file>`: Import OFX/QFX files
- `spice flow --export`: Export to Google Sheets
- `spice categories`: Manage categories
- `spice vendors`: Manage vendor rules
- `spice checks`: Manage check patterns
- `spice recategorize`: Re-classify existing transactions

## Project-Specific Checkpoints

Before marking any task complete:
- [ ] Run `make test && make lint`
- [ ] Check test coverage: `go test -cover ./...`
- [ ] Verify no sensitive data in logs
- [ ] Ensure all new files have proper package documentation
- [ ] Confirm interfaces are small and focused
- [ ] Validate error messages provide context
- [ ] If modifying transactions: ensure amounts remain positive
- [ ] If adding migrations: increment ExpectedSchemaVersion

## When to Re-read This File

- Starting any new feature
- After any failing test
- When design feels complex
- Every 30 minutes of coding
- Before asking for review
- When working with database or migrations