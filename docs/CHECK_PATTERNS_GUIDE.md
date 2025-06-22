# Check Patterns User Guide

## Overview

Check patterns are a powerful feature in the-spice-must-flow that help automatically categorize check transactions. Since checks often have minimal description (just "Check Paid #1234"), patterns help the system learn your recurring check payments and categorize them automatically.

## Why Use Check Patterns?

Without check patterns:
- Every check requires manual categorization
- No context about what the check was for
- Repetitive work for recurring payments

With check patterns:
- Automatic categorization of matching checks
- Confidence boost for AI suggestions
- Time saved on recurring payments
- Pattern usage tracking

## Creating Your First Pattern

### Interactive Creation

The easiest way to create a pattern is using the interactive command:

```bash
spice checks add
```

You'll be guided through:
1. **Pattern Name**: A descriptive name (e.g., "Monthly cleaning")
2. **Category**: Which category to assign matching transactions
3. **Amount Matching**: How to match amounts
4. **Day Restrictions**: Optional day-of-month matching
5. **Notes**: Optional description for your reference

### Amount Matching Options

#### 1. Exact Amount
Perfect for fixed recurring payments:
```
Amount matching:
  [1] Exact amount
  [2] Range
  [3] Multiple amounts
Choice: 1
Enter amount: 100.00
```

#### 2. Amount Range
Useful when amounts vary slightly:
```
Choice: 2
Enter minimum amount: 3000
Enter maximum amount: 3100
```

#### 3. Multiple Amounts
For payments that alternate between specific amounts:
```
Choice: 3
Enter amounts (comma-separated): 100, 200
```

### Day of Month Restrictions

Add timing constraints for better accuracy:
```
Day of month restriction? [y/N]: y
Enter start day (1-31): 1
Enter end day (1-31): 7
```

This pattern only matches checks written in the first week of the month.

## Common Pattern Examples

### 1. Rent Payment
```bash
Pattern name: Monthly rent
Category: Housing
Amount: $3,000-$3,100 (range)
Day restriction: 1-5
Notes: Landlord payment
```

### 2. Cleaning Service
```bash
Pattern name: Bi-weekly cleaning
Category: Home Services
Amounts: $100, $200 (multiple)
Day restriction: None
Notes: CleanCo - alternates between regular and deep clean
```

### 3. Quarterly Taxes
```bash
Pattern name: Estimated taxes
Category: Taxes
Amount: $5,000-$6,000 (range)
Day restriction: 10-20
Notes: IRS quarterly payment
```

### 4. Weekly Allowance
```bash
Pattern name: Kids allowance
Category: Personal
Amount: $50 (exact)
Day restriction: None
Notes: Weekly allowance for kids
```

## Managing Patterns

### List All Patterns
```bash
spice checks list

🌶️ Check Patterns

ID  Pattern Name        Amount(s)      Category         Uses  Last Used
──  ─────────────────  ────────────   ──────────────   ────  ──────────
1   Monthly cleaning   $100, $200     Home Services    24    2024-12-15
2   Rent payment       $3,000-$3,100  Housing         12    2024-12-01  
3   Quarterly taxes    $5,000-$6,000  Taxes            4    2024-10-15
```

### Edit a Pattern
```bash
spice checks edit 1
```

You can modify:
- Pattern name
- Category assignment
- Amount matching rules
- Day restrictions
- Active/inactive status

### Delete a Pattern
```bash
spice checks delete 1

⚠️  Delete check pattern "Monthly cleaning"? [y/N]:
```

### Test Pattern Matching
Before creating a pattern, test what would match:
```bash
spice checks test 100.00

🌶️ Testing amount $100.00

Matching patterns:
1. "Monthly cleaning" → Home Services (24 uses)
3. "Miscellaneous services" → Personal (2 uses)
```

## How Patterns Work During Classification

### Pattern Matching Process

1. **Check Detection**: System identifies check transactions
2. **Pattern Search**: Finds all patterns matching the amount
3. **Date Filtering**: Applies day-of-month restrictions if set
4. **Confidence Boost**: Adds pattern confidence to AI suggestions
5. **Auto-Classification**: If confidence ≥ 85%, auto-categorizes

### Pattern Indicators in UI

When classifying, patterns are highlighted:
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
```

## Advanced Features

### Check Number Patterns

For banks that use predictable check numbers:
```json
{
  "modulo": 10,
  "offset": 5
}
```
This matches check numbers like: 15, 25, 35, 45...

### Pattern Statistics

Patterns track usage:
- **Use Count**: How many times the pattern matched
- **Last Used**: When it last matched a transaction
- **Success Rate**: How often the suggestion was accepted

### Confidence Boosting

Patterns add confidence to AI suggestions:
- Default boost: +30%
- Frequently used patterns: Higher boost
- Recently used patterns: Higher boost

## Best Practices

### 1. Start Specific, Then Generalize
Begin with exact amounts for known payments, then create ranges as you understand the variation.

### 2. Use Descriptive Names
"Cleaning service - BiClean Inc" is better than "Cleaning"

### 3. Review Pattern Performance
```bash
spice checks list --sort=uses
```
Patterns with 0 uses might need adjustment.

### 4. Combine with Vendor Rules
Patterns work alongside vendor rules. Use:
- **Vendor rules**: For transactions with clear merchant names
- **Check patterns**: For generic check descriptions

### 5. Regular Maintenance
Review patterns quarterly:
- Delete unused patterns
- Adjust amounts for inflation
- Update categories as needed

## Troubleshooting

### Pattern Not Matching

1. **Check the amount range**: Is it too narrow?
2. **Review day restrictions**: Are they too restrictive?
3. **Test the pattern**: `spice checks test <amount>`

### Too Many Matches

1. **Narrow the amount range**
2. **Add day-of-month restrictions**
3. **Create more specific patterns**

### Wrong Category Assignment

1. **Edit the pattern**: `spice checks edit <id>`
2. **Update the category**
3. **Consider creating a new, more specific pattern**

## Migration from Manual Categorization

If you've been manually categorizing checks:

1. **Export recent transactions**: Identify patterns in your check history
2. **Create patterns**: Start with your most frequent check amounts
3. **Test on historical data**: Use checkpoints to safely test
4. **Refine patterns**: Adjust based on results

Example migration workflow:
```bash
# Create a checkpoint before testing
spice checkpoint create --tag before-patterns

# Create patterns based on history
spice checks add

# Re-run classification on recent checks
spice classify --from 2024-01-01 --only-checks

# If happy, keep changes. If not:
spice checkpoint restore before-patterns
```

## Future Enhancements

Coming improvements to check patterns:
- Pattern learning from user behavior
- Template library for common patterns
- Pattern sharing between users
- Advanced matching with regex
- Payee prediction based on patterns