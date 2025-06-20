# Testing SimpleFIN Integration

## Quick Start

1. Set your SimpleFIN token:
   ```bash
   export SIMPLEFIN_TOKEN="your-simplefin-token-url"
   ```

2. Build and test:
   ```bash
   make build
   ./bin/spice test-simplefin
   ```

3. Check different date ranges:
   ```bash
   # Last 30 days (default)
   ./bin/spice test-simplefin
   
   # Last 90 days
   ./bin/spice test-simplefin -d 90
   
   # Last year
   ./bin/spice test-simplefin -d 365
   
   # Verbose mode (see raw data)
   ./bin/spice test-simplefin -v
   ```

## What to Look For

1. **Transaction History**: How far back can we get data?
2. **Data Quality**: Are merchant names usable?
3. **Account Coverage**: Do we see all your accounts?
4. **Transaction Completeness**: Any missing transactions?

## Next Steps

Based on what we find, we'll:
1. Enhance merchant normalization if needed
2. Implement transaction fetching strategies for limited history
3. Add CSV import as fallback for older data