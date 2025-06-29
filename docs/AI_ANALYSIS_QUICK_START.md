# AI Analysis Quick Start Guide

Get started with the AI analysis feature in 5 minutes.

## Prerequisites

Before running analysis, ensure you have:
- ✅ Imported transactions (`spice import`)
- ✅ Categorized transactions (`spice classify`)
- ✅ Configured your LLM API key

## Basic Analysis

### 1. Run Your First Analysis

```bash
# Analyze the last 30 days
spice analyze
```

You'll see output like:
```
═══════════════════════════════════════════════════════════════
                    TRANSACTION ANALYSIS REPORT
═══════════════════════════════════════════════════════════════

Period: 2024-05-30 to 2024-06-29
Transactions Analyzed: 342
Coherence Score: 78%

SUMMARY
───────────────────────────────────────────────────────────────
✓ Good category consistency for utilities and subscriptions
✗ 12 vendors with split categorization
✗ 5 recurring transactions without pattern rules
! 2 potentially miscategorized transaction groups
```

### 2. Review Top Issues

The analysis will highlight your most important issues:

```
TOP ISSUES
───────────────────────────────────────────────────────────────
1. [HIGH] Inconsistent: AMAZON MARKETPLACE
   - Shopping: 45 transactions ($1,234.56)
   - Entertainment: 8 transactions ($234.56)
   - Suggested: Create pattern rule for "AMAZON DIGITAL"
```

### 3. Apply a Fix

In interactive mode, navigate with arrow keys and press:
- `Enter` - View details
- `a` - Apply fix
- `s` - Skip
- `q` - Quit

## Common Scenarios

### Monthly Review Workflow

```bash
# 1. Import recent transactions
spice import

# 2. Classify new transactions
spice classify --batch

# 3. Run analysis
spice analyze

# 4. Apply high-confidence fixes
spice analyze --auto-apply --min-confidence 0.95

# 5. Export to sheets
spice flow --export
```

### Quarterly Cleanup

```bash
# Analyze last quarter
spice analyze --start-date 2024-01-01 --end-date 2024-03-31

# Focus on pattern opportunities
spice analyze --focus patterns

# Review and create suggested patterns
spice patterns add "NETFLIX" --category "Entertainment" --confidence 0.98
```

### Pre-Tax Preparation

```bash
# Analyze full year
spice analyze --year 2023

# Export analysis report
spice analyze --year 2023 --output json > tax_analysis_2023.json

# Fix any critical issues
spice analyze --year 2023 --auto-apply --severity critical
```

## Understanding Results

### Coherence Score Guide

Your coherence score indicates overall consistency:

| Score | Rating | What it Means |
|-------|---------|---------------|
| 90-100% | Excellent | Your categorization is highly consistent |
| 75-89% | Good | Minor improvements possible |
| 60-74% | Fair | Several issues to address |
| <60% | Poor | Significant inconsistencies |

### Issue Priorities

Focus on issues in this order:
1. **CRITICAL** - Data integrity issues
2. **HIGH** - Major inconsistencies
3. **MEDIUM** - Efficiency improvements
4. **LOW** - Nice-to-have optimizations

## Tips for Success

### 1. Start Small
```bash
# Analyze just one week to understand the output
spice analyze --start-date $(date -d '1 week ago' +%Y-%m-%d)
```

### 2. Use Dry Run First
```bash
# See what would change without applying
spice analyze --dry-run
```

### 3. Create Patterns for Recurring Transactions
```bash
# After analysis suggests patterns, create them
spice patterns add "SPOTIFY" --category "Entertainment" --confidence 0.95
spice patterns add "COMCAST" --category "Utilities" --confidence 0.95
```

### 4. Handle Split Vendors
For vendors like Amazon or Target that span categories:
```bash
# Create specific patterns
spice patterns add "AMAZON DIGITAL" --category "Entertainment" --regex
spice patterns add "AMAZON MARKETPLACE" --category "Shopping"
```

## Next Steps

1. **Read the Full Guide**: [AI Analysis User Guide](AI_ANALYSIS_USER_GUIDE.md)
2. **Explore Technical Details**: [Technical Reference](AI_ANALYSIS_TECHNICAL_REFERENCE.md)
3. **Set Up Automation**: Create scripts for regular analysis
4. **Share Insights**: Export reports for your accountant

## Troubleshooting

### "No transactions to analyze"
- Ensure you have categorized transactions: `spice stats`
- Check your date range

### "API rate limit exceeded"
- Use smaller date ranges
- Wait a few minutes and retry

### "Low coherence score"
- Normal for new users - improves over time
- Focus on HIGH severity issues first
- Consider consolidating similar categories

## Example Output

Here's what a healthy categorization looks like after using analysis:

```
Coherence Score: 94%

INSIGHTS
───────────────────────────────────────────────────────────────
✓ Excellent consistency in utility categorization (100%)
✓ Strong pattern coverage - 89% of recurring transactions automated
✓ Clear category boundaries with minimal overlap
✓ Only 2 ambiguous vendors requiring manual review

No critical or high-severity issues found.
```

Ready to improve your categorization? Run `spice analyze` now!