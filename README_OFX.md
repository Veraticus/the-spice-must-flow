# OFX/QFX Import Guide

## Exporting from Your Bank

### Chase
1. Log into Chase online banking
2. Go to your account
3. Click "Download account activity"
4. Select date range (get the full year for taxes)
5. Choose "Quicken (.QFX)" format
6. Download file

### Ally Bank
1. Log into Ally online banking
2. Select your account
3. Click "Download Transactions"
4. Choose date range
5. Select "Quicken (QFX)" format
6. Download file

## Importing to Spice

```bash
# Import single file
./spice import-ofx ~/Downloads/Chase1234.qfx

# Import all QFX files in a directory
./spice import-ofx ~/Downloads/*.qfx

# Import from multiple patterns
./spice import-ofx ~/Downloads/Chase*.qfx ~/Downloads/Ally*.qfx

# Import all 2024 files with dry run
./spice import-ofx ~/Downloads/*2024*.qfx --dry-run

# Import with verbose output
./spice import-ofx ~/Downloads/*.qfx -v
```

## Deduplication

Transactions are automatically deduplicated across all files based on:
- Date
- Amount  
- Merchant name
- Account ID

This means you can safely import overlapping month ranges or re-import files.

## What OFX Provides

- **Transaction IDs**: Unique identifiers for deduplication
- **Clean Merchant Names**: Better than CSV in many cases
- **Account Numbers**: Last 4 digits typically
- **Transaction Types**: Debit, credit, check, etc.
- **Check Numbers**: When applicable

## Typical Workflow

1. Export Jan-Sept 2024 as QFX from Chase/Ally
2. Import with `spice import-ofx`
3. Use SimpleFIN for Oct-Dec 2024 (last 90 days)
4. Run categorization on combined data
5. Export to Google Sheets for taxes

## Notes

- OFX files can contain multiple accounts
- Transactions are automatically deduplicated by hash
- Some banks limit export to 18-24 months of history
- QFX is Quicken's variant of OFX (we support both)