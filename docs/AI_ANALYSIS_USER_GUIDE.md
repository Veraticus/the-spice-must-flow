# AI Analysis User Guide

The AI-powered analysis feature in `spice` provides intelligent insights into your transaction categorization, helping you identify issues, optimize patterns, and improve the overall quality of your financial data.

## Table of Contents

1. [Overview](#overview)
2. [Getting Started](#getting-started)
3. [Understanding the Analysis](#understanding-the-analysis)
4. [Using the Analysis Command](#using-the-analysis-command)
5. [Interpreting Results](#interpreting-results)
6. [Applying Fixes](#applying-fixes)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)

## Overview

The AI analysis feature examines your categorized transactions to identify:

- **Inconsistent Categorizations**: Similar transactions assigned to different categories
- **Missing Pattern Rules**: Recurring transactions that could benefit from automatic categorization
- **Ambiguous Vendors**: Merchants with transactions in multiple categories
- **Category Optimization**: Opportunities to merge, split, or reorganize categories
- **Overall Coherence**: A score indicating the consistency of your categorization system

## Getting Started

### Prerequisites

1. **Categorized Transactions**: You should have at least 30 days of categorized transactions
2. **API Key**: Configure your LLM API key (Claude or OpenAI) in your config file
3. **Categories**: A well-defined set of categories for your transactions

### Basic Usage

Run a basic analysis of the last 30 days:

```bash
spice analyze
```

This will:
1. Load your recent transactions
2. Analyze categorization patterns
3. Generate a comprehensive report
4. Display actionable insights

## Understanding the Analysis

### Coherence Score

The coherence score (0-100%) indicates how consistent your categorization is:

- **90-100%**: Excellent - Very consistent categorization
- **75-89%**: Good - Minor inconsistencies that can be improved
- **60-74%**: Fair - Notable issues that should be addressed
- **Below 60%**: Poor - Significant categorization problems

### Issue Types

#### 1. Inconsistent Categorization
Similar transactions from the same vendor are split across different categories.

**Example**: 
- "AMAZON MARKETPLACE" → Shopping (80%)
- "AMAZON MARKETPLACE" → Entertainment (20%)

#### 2. Missing Pattern Rules
Recurring transactions that are manually categorized each time.

**Example**:
- "NETFLIX" appears monthly but has no pattern rule

#### 3. Ambiguous Vendors
Vendors whose transactions legitimately span multiple categories.

**Example**:
- "TARGET" → Groceries (40%), Shopping (30%), Household (30%)

#### 4. Duplicate Patterns
Multiple pattern rules that match the same transactions.

## Using the Analysis Command

### Date Range Options

Analyze specific time periods:

```bash
# Last 90 days
spice analyze --start-date 2024-01-01 --end-date 2024-03-31

# Current year
spice analyze --start-date 2024-01-01

# Specific month
spice analyze --start-date 2024-06-01 --end-date 2024-06-30
```

### Focus Areas

Target specific aspects of your data:

```bash
# Focus on pattern effectiveness
spice analyze --focus patterns

# Analyze category distribution
spice analyze --focus categories

# Check overall coherence
spice analyze --focus coherence
```

### Output Formats

```bash
# Interactive mode (default) - navigate through issues
spice analyze

# Summary only - high-level overview
spice analyze --output summary

# JSON export - for programmatic use
spice analyze --output json > analysis.json
```

### Advanced Options

```bash
# Limit number of issues reported
spice analyze --max-issues 20

# Continue a previous session
spice analyze --session-id abc123

# Preview fixes without applying
spice analyze --dry-run

# Automatically apply high-confidence fixes
spice analyze --auto-apply
```

## Interpreting Results

### Reading the Report

A typical analysis report includes:

```
═══════════════════════════════════════════════════════════════
                    TRANSACTION ANALYSIS REPORT
═══════════════════════════════════════════════════════════════

Period: 2024-01-01 to 2024-03-31
Transactions Analyzed: 1,234
Coherence Score: 82%

SUMMARY
───────────────────────────────────────────────────────────────
✓ Strong category consistency for regular vendors
✗ 15 vendors with split categorization
✗ 8 recurring transactions without pattern rules
! 3 potentially miscategorized transaction groups

TOP ISSUES
───────────────────────────────────────────────────────────────
1. [HIGH] Inconsistent: AMAZON MARKETPLACE
   - Shopping: 145 transactions ($3,456.78)
   - Entertainment: 23 transactions ($567.89)
   - Suggested: Create pattern rule for "AMAZON DIGITAL"

2. [MEDIUM] Missing Pattern: SPOTIFY
   - 3 monthly transactions manually categorized
   - Suggested: Create pattern → Entertainment
```

### Understanding Severity Levels

- **CRITICAL**: Issues affecting data integrity or large amounts
- **HIGH**: Significant inconsistencies affecting many transactions
- **MEDIUM**: Moderate issues that would improve efficiency
- **LOW**: Minor optimizations or quality-of-life improvements

### Confidence Scores

Each suggestion includes a confidence score:

- **95-100%**: Very high confidence - safe to auto-apply
- **85-94%**: High confidence - review recommended
- **70-84%**: Moderate confidence - careful review needed
- **Below 70%**: Low confidence - manual decision required

## Applying Fixes

### Interactive Mode

When running in interactive mode, you can:

1. **Navigate**: Use arrow keys to browse issues
2. **View Details**: Press Enter to see affected transactions
3. **Apply Fix**: Press 'a' to apply the selected fix
4. **Skip**: Press 's' to skip and move to the next issue
5. **Quit**: Press 'q' to exit without saving

### Batch Application

For high-confidence fixes:

```bash
# Review what would be changed
spice analyze --dry-run

# Apply all fixes with >95% confidence
spice analyze --auto-apply
```

### Manual Application

For complex issues, you may need to:

1. **Create Pattern Rules**:
   ```bash
   spice patterns add "NETFLIX" --category Entertainment --confidence 0.95
   ```

2. **Recategorize Transactions**:
   ```bash
   spice recategorize --vendor "AMAZON MARKETPLACE" --from Shopping --to Entertainment --filter "DIGITAL"
   ```

3. **Adjust Categories**:
   ```bash
   spice categories add "Digital Services" --description "Streaming, software, and online services"
   ```

## Best Practices

### 1. Regular Analysis

Run analysis monthly to catch issues early:

```bash
# Add to your monthly routine
spice analyze --start-date $(date -d '1 month ago' +%Y-%m-01)
```

### 2. Focus on High-Impact Issues

Address issues in order of impact:
- Fix CRITICAL and HIGH severity issues first
- Apply high-confidence fixes automatically
- Review MEDIUM issues during regular maintenance

### 3. Refine Pattern Rules

After analysis, refine your patterns:

```bash
# List current patterns
spice patterns list

# Edit patterns with low match rates
spice patterns edit <id> --confidence 0.8
```

### 4. Category Hygiene

Keep categories focused and distinct:
- Merge categories with significant overlap
- Split categories that are too broad
- Remove unused categories

### 5. Document Decisions

For ambiguous vendors, document your categorization logic:

```bash
# Add notes to explain split categorization
spice vendors edit "TARGET" --note "Groceries for food, Shopping for other items"
```

## Troubleshooting

### Common Issues

#### Analysis Takes Too Long

For large datasets, limit the scope:

```bash
# Analyze one month at a time
spice analyze --start-date 2024-06-01 --end-date 2024-06-30

# Focus on specific issues
spice analyze --focus patterns --max-issues 10
```

#### Low Coherence Score

If your coherence score is consistently low:

1. Review your category definitions
2. Check for overlapping categories
3. Consider consolidating similar categories
4. Run analysis with `--focus categories`

#### API Rate Limits

If you hit rate limits:

```bash
# Use a smaller date range
spice analyze --start-date $(date -d '1 week ago' +%Y-%m-%d)

# Or continue from a saved session
spice analyze --session-id <previous-session-id>
```

### Error Messages

**"No transactions found"**
- Ensure you have categorized transactions in the date range
- Check your date format (YYYY-MM-DD)

**"Analysis failed after retries"**
- Check your internet connection
- Verify API key configuration
- Try with a smaller dataset

**"Invalid JSON response"**
- This is usually temporary - retry the analysis
- If persistent, try `--focus` on a specific area

### Getting Help

For additional help:

1. Check the verbose output:
   ```bash
   spice analyze --verbose
   ```

2. Review the session log:
   ```bash
   cat ~/.local/share/spice/analysis_sessions.db
   ```

3. Report issues with session ID:
   ```bash
   spice analyze --session-id <id> --debug
   ```

## Examples

### Monthly Categorization Review

```bash
#!/bin/bash
# monthly-review.sh

# Run analysis for last month
LAST_MONTH=$(date -d '1 month ago' +%Y-%m)
spice analyze \
  --start-date "$LAST_MONTH-01" \
  --end-date "$(date +%Y-%m-01)" \
  --output summary

# Auto-apply high-confidence fixes
spice analyze \
  --start-date "$LAST_MONTH-01" \
  --end-date "$(date +%Y-%m-01)" \
  --auto-apply \
  --min-confidence 0.95
```

### Quarterly Deep Analysis

```bash
# Comprehensive quarterly review
spice analyze \
  --start-date $(date -d '3 months ago' +%Y-%m-01) \
  --max-issues 50 \
  --output json > quarterly_analysis.json

# Generate category optimization report
jq '.category_summary | to_entries | sort_by(.value.transaction_count) | reverse' \
  quarterly_analysis.json
```

### Pre-Tax Preparation

```bash
# Analyze full year for tax preparation
YEAR=$(date -d '1 year ago' +%Y)
spice analyze \
  --start-date "$YEAR-01-01" \
  --end-date "$YEAR-12-31" \
  --focus coherence

# Export issues for accountant review
spice analyze \
  --start-date "$YEAR-01-01" \
  --end-date "$YEAR-12-31" \
  --output json | \
  jq '.issues[] | select(.severity == "HIGH" or .severity == "CRITICAL")' \
  > tax_prep_issues.json
```

## Conclusion

The AI analysis feature is a powerful tool for maintaining high-quality financial categorization. By running regular analyses and addressing identified issues, you can ensure your financial data remains accurate, consistent, and useful for budgeting and tax purposes.

Remember: The AI provides suggestions, but your domain knowledge is essential for making the final decisions about categorization.