# Plaid Setup Guide

## Overview

The spice-must-flow uses Plaid to securely connect to your financial institutions and import transactions. This guide will help you set up Plaid integration.

## Prerequisites

1. A Plaid account (sign up at https://plaid.com)
2. Access to your financial institution accounts

## Configuration

### Environment Variables (Recommended for Security)

Set the following environment variables:

```bash
export SPICE_PLAID_CLIENT_ID="your-client-id"
export SPICE_PLAID_SECRET="your-secret-key"
export SPICE_PLAID_ENVIRONMENT="sandbox"  # or "development" or "production"
export SPICE_PLAID_ACCESS_TOKEN="your-access-token"
```

### Configuration File

Alternatively, you can add Plaid settings to `~/.config/spice/config.yaml`:

```yaml
plaid:
  client_id: "your-client-id"
  secret: "your-secret-key"
  environment: "sandbox"  # or "development" or "production"
  access_token: "your-access-token"
```

**Note:** For security reasons, we recommend using environment variables for sensitive data like API keys.

## Obtaining Plaid Credentials

### 1. Client ID and Secret

1. Log in to your Plaid Dashboard
2. Navigate to Team Settings > Keys
3. Copy your Client ID and Secret

### 2. Environment

- **sandbox**: For testing with fake data (free)
- **development**: For testing with real data (100 free Items)
- **production**: For production use (paid)

### 3. Access Token

You'll need to use Plaid Link to obtain an access token for each financial institution. The access token represents your authorization to access a specific account.

For testing in sandbox mode, you can use Plaid's quickstart tool to generate a test access token.

## Usage

### Import Transactions

```bash
# Import last 30 days of transactions
spice import

# Import specific date range
spice import --start-date 2024-01-01 --end-date 2024-01-31

# Import last 90 days
spice import --days 90

# List available accounts
spice import --list-accounts

# Import from specific accounts only
spice import --accounts "account_id_1,account_id_2"

# Preview without saving
spice import --dry-run
```

### Troubleshooting

#### Rate Limits

The Plaid client automatically handles rate limiting with exponential backoff. If you encounter persistent rate limit errors, wait a few minutes before retrying.

#### Authentication Errors

- Verify your credentials are correct
- Check that your environment matches your Plaid account type
- Ensure your access token is valid and not expired

#### No Transactions Found

- Check the date range you're querying
- Verify the account has transactions in that period
- Some institutions may have delays in transaction availability

## Security Best Practices

1. **Never commit credentials to version control**
2. Use environment variables for sensitive data
3. Rotate your API keys regularly
4. Use the appropriate environment for your use case
5. Monitor your Plaid Dashboard for unusual activity