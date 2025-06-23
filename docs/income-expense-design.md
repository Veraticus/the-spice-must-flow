# Income and Expense Tracking Design Document

## Overview

This document outlines the design for properly implementing income and expense tracking in the-spice-must-flow. Currently, the system treats all transactions as expenses, which prevents accurate cash flow analysis. This redesign will create a proper double-entry aware system while maintaining the app's delightful user experience.

## Goals

1. **Accurately track both income and expenses** with clear differentiation
2. **Maintain simplicity** - users shouldn't need accounting knowledge
3. **Preserve the delightful CLI experience** with smart defaults and AI assistance
4. **Enable meaningful cash flow analysis** showing money in vs money out
5. **Handle edge cases gracefully** (transfers, refunds, reimbursements)

## Core Design Decisions

### 1. Transaction Direction Representation

**Decision: Explicit Direction Field with Positive Amounts**

All transactions will store amounts as positive values with an explicit `direction` field:

```go
type TransactionDirection string

const (
    DirectionIncome   TransactionDirection = "income"
    DirectionExpense  TransactionDirection = "expense"
    DirectionTransfer TransactionDirection = "transfer"
)

type Transaction struct {
    // ... existing fields ...
    Amount    float64              // Always stored as positive
    Direction TransactionDirection // Explicit indicator
    // ... other fields ...
}
```

**Rationale:**
- **User Clarity**: "Income" and "Expense" labels are immediately understandable
- **Import Flexibility**: Different banks use different sign conventions; we can normalize intelligently
- **Query Simplicity**: `WHERE direction = 'expense'` is clearer than `WHERE amount < 0`
- **AI Integration**: LLMs can easily classify direction as part of categorization
- **Future Extensibility**: Easy to add new transaction types without breaking existing code

### 2. Transfer Handling

**Decision: Neutral Transfer Transactions**

Transfers between accounts will be marked with `DirectionTransfer` and excluded from income/expense totals by default:

```go
// Example transfer detection during import
if strings.Contains(desc, "transfer from") || strings.Contains(desc, "transfer to") {
    transaction.Direction = DirectionTransfer
    transaction.Category = "Transfers"  // Special system category
}
```

**User Experience:**
- Transfers appear in transaction list but not in income/expense totals
- Flow command shows transfers separately if present
- Users can optionally include transfers in analysis via flags

### 3. Refunds and Returns

**Decision: Refunds as Negative Expenses**

Refunds will be tracked as income transactions but categorized in the original expense category to reduce that category's total:

```go
// Refund handling logic
if transaction.Direction == DirectionIncome && isRefund(transaction) {
    // Prompt: "This looks like a refund. Which expense category should it reduce?"
    transaction.IsRefund = true
    transaction.RefundCategory = userSelectedCategory
}
```

**Example Flow:**
1. User spends $100 on "Shopping"
2. User receives $20 refund
3. Refund is marked as income but linked to "Shopping"
4. Shopping total shows: $100 - $20 = $80 net spent

### 4. Category System

**Decision: Strict Category Types**

Each category will be explicitly marked as either income or expense:

```go
type CategoryType string

const (
    CategoryTypeIncome  CategoryType = "income"
    CategoryTypeExpense CategoryType = "expense"
    CategoryTypeSystem  CategoryType = "system"  // For transfers, adjustments
)

type Category struct {
    ID          string
    Name        string
    Type        CategoryType
    Description string
    IsActive    bool
    CreatedAt   time.Time
}
```

**Default Income Categories:**
- Salary & Wages
- Freelance Income  
- Interest & Dividends
- Refunds & Reimbursements
- Gifts Received
- Other Income

**Category Rules:**
- Categories can only be used for their designated type
- Attempting to use an income category for an expense prompts for correction
- System provides smart suggestions based on transaction type

### 5. Enhanced Flow Command

**Decision: Comprehensive Cash Flow Display**

The `spice flow` command will show a complete financial picture:

```
╭───────────────────────────────────────────────────────────────╮
│  💰 Cash Flow Summary - December 2024                         │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│  📈 INCOME                              $12,459.32            │
│  ├─ Salary & Wages                      $10,230.00           │
│  ├─ Freelance Income                     $2,100.00           │
│  └─ Interest & Dividends                   $129.32           │
│                                                               │
│  📉 EXPENSES                            $8,234.21             │
│  ├─ Housing                              $2,500.00           │
│  ├─ Food & Dining                       $1,234.56           │
│  ├─ Transportation                         $543.21           │
│  └─ Other (8 categories)                $3,956.44           │
│                                                               │
│  ➡️  TRANSFERS (excluded)                 $500.00            │
│                                                               │
│  ═══════════════════════════════════════════════             │
│  ✨ NET CASH FLOW                      +$4,225.11            │
│                                                               │
│  📊 Insights:                                                 │
│  • Income increased 15% from last month                      │
│  • Largest expense: Housing (30.3% of expenses)              │
│  • You saved 33.9% of your income                            │
╰───────────────────────────────────────────────────────────────╯
```

### 6. Import Processing

**Decision: Intelligent Direction Detection**

Each importer will use transaction type and pattern matching to set direction:

```go
func (p *PlaidImporter) detectDirection(txn plaid.Transaction) TransactionDirection {
    // Check transaction type first
    switch txn.PaymentChannel {
    case "direct_deposit":
        return DirectionIncome
    case "interest":
        return DirectionIncome
    }
    
    // Check amount sign (Plaid-specific logic)
    if txn.Amount < 0 {
        return DirectionIncome  // Plaid uses negative for credits
    }
    
    // Pattern matching for common income patterns
    if isLikelyIncome(txn.Name) {
        return DirectionIncome
    }
    
    return DirectionExpense
}
```

### 7. Classification Enhancement

**Decision: AI-Assisted Direction Classification**

The classification prompt will be enhanced to determine both category and direction:

```go
prompt := `Analyze this transaction and provide:
1. Whether this is income, expense, or transfer
2. The most appropriate category
3. Confidence level (0-1)

Transaction: %s
Amount: $%.2f
Type: %s
Date: %s

Respond in JSON format:
{
    "direction": "income|expense|transfer",
    "category": "category name",
    "confidence": 0.95,
    "reasoning": "brief explanation"
}`
```

## Migration Strategy

### Phase 1: Core Model Changes (Breaking Change)
1. Add `direction` column to transactions table
2. Add `type` column to categories table  
3. Create default income categories
4. Migrate existing transactions (all marked as expense initially)
5. Update all importers to set direction

### Phase 2: Smart Migration
1. Re-analyze existing transactions using patterns:
   - CREDIT, INT, DIRECTDEP → Income
   - Known payroll patterns → Income
   - Interest transactions → Income
   - Everything else → Expense
2. Allow users to bulk re-classify after migration

### Phase 3: UI Updates
1. Update classification flow to handle income/expense selection
2. Enhance flow command with new display format
3. Update Google Sheets export to separate income/expense
4. Add `spice recategorize --fix-direction` command

## Edge Case Handling

### Fee Reimbursements
- **Classification**: Income
- **Category**: "Refunds & Reimbursements"
- **Display**: Shown in income section

### Cashback Rewards
- **Classification**: Income
- **Category**: "Rewards & Cashback" (new default category)
- **Display**: Shown in income section

### Tax Withholdings
- **Classification**: Expense
- **Category**: "Taxes"
- **Note**: Money leaving account = expense, regardless of reason

### Split Transactions
- **Future Enhancement**: Allow splitting single transaction into multiple categories/directions
- **Example**: Paycheck split into income (gross) and expense (tax withholding)

## Configuration

New configuration options in `config.yaml`:

```yaml
classification:
  # Patterns to identify income transactions during import
  income_patterns:
    - "PAYROLL"
    - "SALARY" 
    - "DIRECT DEP"
    - "INTEREST"
    - "DIVIDEND"
  
  # Patterns to identify transfers
  transfer_patterns:
    - "TRANSFER FROM"
    - "TRANSFER TO"
    - "XFER"

flow:
  # Include transfers in cash flow calculation
  include_transfers: false
  
  # Show zero-amount categories
  show_empty_categories: false
  
  # Number of insights to display
  max_insights: 3
```

## API Changes

### Transaction Model
```go
type Transaction struct {
    // Existing fields...
    Direction     TransactionDirection `json:"direction"`
    IsRefund      bool                `json:"is_refund,omitempty"`
    RefundCategory string             `json:"refund_category,omitempty"`
}
```

### Category Model  
```go
type Category struct {
    // Existing fields...
    Type CategoryType `json:"type"`
}
```

### New Service Methods
```go
interface TransactionService {
    // Existing methods...
    GetIncomeByPeriod(ctx context.Context, start, end time.Time) ([]Transaction, error)
    GetExpensesByPeriod(ctx context.Context, start, end time.Time) ([]Transaction, error)
    GetCashFlow(ctx context.Context, start, end time.Time) (*CashFlowSummary, error)
}
```

## Testing Strategy

1. **Unit Tests**: Test direction detection logic for each importer
2. **Integration Tests**: Test full import → classify → flow pipeline
3. **Migration Tests**: Ensure existing data migrates correctly
4. **Edge Case Tests**: Verify handling of refunds, transfers, etc.

## Success Metrics

1. **Accuracy**: 95%+ correct automatic direction classification
2. **User Satisfaction**: Reduced manual corrections needed
3. **Performance**: No noticeable slowdown in classification
4. **Completeness**: Handle 99% of real-world transaction types

## Future Enhancements

1. **Multi-Account Support**: Track transfers between user's accounts
2. **Budget Tracking**: Set income/expense budgets with alerts
3. **Cash Flow Forecasting**: Predict future cash position
4. **Investment Tracking**: Handle investment income/gains specially
5. **Bill Detection**: Identify recurring expenses automatically

## Conclusion

This design transforms the-spice-must-flow from an expense tracker into a complete personal finance system while maintaining its delightful, CLI-first user experience. The explicit direction model provides clarity and flexibility while enabling powerful cash flow analysis that helps users truly understand their financial picture.