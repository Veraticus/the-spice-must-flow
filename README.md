# the-spice-must-flow

> "He who controls the spice controls the universe." - Frank Herbert, Dune

A personal finance categorization engine that ingests financial transactions from Plaid, uses AI to intelligently categorize them, and exports reports to Google Sheets. Built with Go, it emphasizes testability, extensibility, and a delightful CLI experience.

## Features

- 🏦 **Plaid Integration**: Automatically import transactions from your bank accounts
- 🤖 **AI-Powered Categorization**: Uses OpenAI, Anthropic, or Claude Code to intelligently categorize transactions
- 📊 **Google Sheets Export**: Export categorized transactions to Google Sheets for reporting
- 🔄 **Smart Deduplication**: SHA256 hashing prevents duplicate transaction processing
- 📚 **Learning System**: Remembers vendor categorizations to auto-categorize future transactions
- 🎯 **Batch Processing**: Groups transactions by merchant for efficient review
- 💾 **Resume Support**: Classification sessions can be paused and resumed
- 🎨 **Beautiful CLI**: Interactive command-line interface with progress indicators
- 🏷️ **Dynamic Categories**: Categories evolve based on your spending patterns with AI-generated descriptions
- 💾 **Database Checkpoints**: Save and restore database states for safe experimentation
- 🤝 **Category Sharing**: Export and import category configurations with colleagues
- 💰 **Check Pattern Recognition**: Automatically categorize recurring check payments based on amount and timing patterns
- 🎯 **Pattern-Based Classification**: Intelligent pattern rules that consider merchant, amount, and transaction direction
- 🔍 **AI-Powered Analysis**: Analyze your categorization for inconsistencies, missing patterns, and optimization opportunities

## Installation

### Prerequisites

- Go 1.21 or higher
- SQLite3
- Make

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Veraticus/the-spice-must-flow.git
cd the-spice-must-flow

# Install development tools
make install-tools

# Build the binary
make build

# The binary will be available at ./bin/spice
```

## Configuration

The application supports three ways to configure settings, in order of precedence:
1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file (lowest priority)

### Configuration File (Recommended)

Create a configuration file at `~/.config/spice/config.yaml`:

```bash
# Create config directory
mkdir -p ~/.config/spice

# Copy example config
cp config/config.example.yaml ~/.config/spice/config.yaml

# Edit with your settings
nano ~/.config/spice/config.yaml
```

The configuration file supports all settings and keeps your sensitive data organized. See `config/config.example.yaml` for a complete example with documentation.

### Environment Variables

You can also use environment variables. Prefix all settings with `SPICE_` and convert to uppercase with underscores:
- `llm.provider` → `SPICE_LLM_PROVIDER`
- `plaid.client_id` → `SPICE_PLAID_CLIENT_ID`
- `sheets.service_account_path` → `SPICE_SHEETS_SERVICE_ACCOUNT_PATH`

For API keys, you can also use the standard environment variables:
- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`

### Required Configuration

Configure these settings either in `~/.config/spice/config.yaml` or via environment variables:

#### Using config.yaml (Recommended):
```yaml
# Plaid API Credentials
plaid:
  client_id: "your-plaid-client-id"
  secret: "your-plaid-secret"
  environment: "sandbox"  # Options: sandbox, development, production

# AI Provider Configuration
llm:
  provider: "openai"  # Options: openai, anthropic, claudecode
  openai_api_key: "your-openai-api-key"
  # anthropic_api_key: "your-anthropic-api-key"  # If using Anthropic
  
  # For Claude Code (local CLI):
  # provider: "claudecode"
  # claude_code_path: "/path/to/claude"  # Optional, defaults to "claude"
  
  model: "gpt-4"
  temperature: 0.0

# Google Sheets (choose one auth method)
sheets:
  # For OAuth2 (recommended for Google Workspace):
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  # refresh_token will be added by 'spice sheets auth'
  
  # For Service Account (if allowed by your org):
  # service_account_path: "/path/to/service-account-key.json"
  
  spreadsheet_id: "your-spreadsheet-id"  # Optional
  spreadsheet_name: "My Finance Tracker"
```

#### Using Environment Variables:
```bash
# Plaid API Credentials
export SPICE_PLAID_CLIENT_ID="your-plaid-client-id"
export SPICE_PLAID_SECRET="your-plaid-secret"
export SPICE_PLAID_ENVIRONMENT="sandbox"

# AI Provider
export SPICE_LLM_PROVIDER="openai"
export OPENAI_API_KEY="your-openai-api-key"
# export ANTHROPIC_API_KEY="your-anthropic-api-key"  # If using Anthropic

# Google Sheets
export GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH="/path/to/service-account-key.json"
export GOOGLE_SHEETS_SPREADSHEET_ID="your-spreadsheet-id"
```

### Google Sheets Setup

#### Option 1: Service Account Authentication (Recommended)

1. **Create a Google Cloud Project:**
   - Go to [Google Cloud Console](https://console.cloud.google.com)
   - Create a new project or select an existing one
   - Enable the Google Sheets API for your project

2. **Create a Service Account:**
   - Go to "IAM & Admin" → "Service Accounts"
   - Click "Create Service Account"
   - Give it a name (e.g., "spice-sheets-writer")
   - Click "Create and Continue"
   - Skip the optional steps and click "Done"

3. **Generate a Key:**
   - Click on your new service account
   - Go to the "Keys" tab
   - Click "Add Key" → "Create new key"
   - Choose JSON format
   - Save the downloaded file securely

4. **Configure the Application:**
   ```bash
   export GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH="/path/to/your-service-account-key.json"
   ```

5. **Grant Access to Your Spreadsheet:**
   - Open your Google Sheets spreadsheet
   - Click "Share"
   - Add the service account email (found in the JSON file)
   - Give it "Editor" permissions

#### Option 2: OAuth2 Authentication

Use this method if you can't create service account keys (common in Google Workspace accounts).

1. **Create OAuth2 Credentials:**
   - Go to [Google Cloud Console](https://console.cloud.google.com)
   - Go to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth client ID"
   - Choose "Desktop app" as the application type
   - Note down the Client ID and Client Secret

2. **Configure and Authenticate:**
   ```bash
   # Add to your config.yaml or use environment variables
   export GOOGLE_SHEETS_CLIENT_ID="your-client-id"
   export GOOGLE_SHEETS_CLIENT_SECRET="your-client-secret"
   
   # Run the authentication flow
   spice auth sheets
   ```

3. **Complete Authentication:**
   - The command will open your browser
   - Log in to your Google account
   - Grant access to Google Sheets
   - The refresh token will be saved automatically

**Note:** The refresh token is saved to `~/.config/spice/sheets-token.json` and your config file. Google refresh tokens don't expire unless:
- You explicitly revoke access
- The token hasn't been used for 6 months
- You change your Google password
- Your Google account has 2FA changes

#### Quick Start for Google Workspace Users

If your organization blocks service account key creation, here's the fastest way to get started:

1. **Create OAuth2 credentials in Google Cloud Console** (one-time setup):
   ```
   Project → APIs & Services → Credentials → Create Credentials → OAuth client ID → Desktop app
   ```

2. **Update your config.yaml**:
   ```yaml
   sheets:
     client_id: "paste-your-client-id-here"
     client_secret: "paste-your-client-secret-here"
   ```

3. **Run the auth command**:
   ```bash
   spice auth sheets
   ```

4. **You're done!** The refresh token is saved and you won't need to authenticate again.

### Optional Configuration

Additional settings in `config.yaml`:
```yaml
# Database location (defaults to ~/.local/share/spice/spice.db)
database:
  path: "~/my-finance-data/spice.db"

# Advanced LLM settings
llm:
  max_tokens: 150
  rate_limit: 1000  # requests per minute
  cache_ttl: "24h"

# Classification settings
classification:
  batch_size: 50
  auto_approve_threshold: 0.95  # Auto-approve if confidence > 95%
  acceptance_threshold: 0.8     # Default threshold for --batch mode

# Logging
logging:
  level: "debug"  # Options: debug, info, warn, error
  format: "json"  # Options: console, json
```

Or via environment variables:
```bash
export SPICE_DATABASE_PATH="/path/to/spice.db"
export SPICE_LLM_MODEL="gpt-4"
export SPICE_LLM_MAX_TOKENS="150"
export SPICE_CLASSIFICATION_BATCH_SIZE="50"
export SPICE_LOGGING_LEVEL="debug"
```

### Using Claude Code (Local LLM)

Claude Code provides a free, local alternative to API-based LLMs:

```yaml
# In config.yaml
llm:
  provider: "claudecode"
  claude_code_path: "/home/user/.npm-global/bin/claude"  # Optional custom path
  model: "sonnet"  # or "opus", "haiku"
```

Benefits:
- No API keys required
- Works offline once installed
- No usage costs
- Categories get AI-generated descriptions locally

Note: Requires Claude Code CLI to be installed (`npm install -g @anthropic-ai/claude-code`).

## Usage

### 1. Connect Your Bank Accounts

Connect your bank accounts using Plaid:

```bash
# Connect your first bank (e.g., Chase credit card)
spice auth plaid

# Connect additional banks (e.g., Ally checking)
spice auth plaid

# For testing with fake data (optional):
spice auth plaid --env sandbox

# This will:
# - Open your browser to Plaid Link
# - Automatically set up HTTPS with a self-signed certificate (for production)
# - Let you securely connect your bank
# - Save the access token for future use
```

**HTTPS Certificate Note**: When using production mode, the tool automatically generates a self-signed certificate for secure OAuth flows. You'll see a browser security warning on first use - this is normal and expected. Simply click "Advanced" and "Proceed to localhost" to continue.

**OAuth vs Non-OAuth Banks**:
- **Non-OAuth banks**: Work immediately! Use username/password directly in Plaid Link
- **OAuth banks** (like Chase): Require extra setup in Plaid Dashboard

To check which banks require OAuth:
```bash
spice institutions search "bank name"
```

**Setting up for Production**:
1. Get production access from Plaid
2. In Plaid Dashboard → Team Settings → API → Allowed redirect URIs
3. Add: `https://localhost:8080/`
4. Save changes

Once configured, ALL banks (OAuth and non-OAuth) will work seamlessly!

Until then, you can:
- Use sandbox mode for testing: `spice auth plaid --env sandbox`
- Search for specific banks: `spice institutions search [bank name]`

**Multi-Bank Support**: Connect as many banks as you need. When you run `spice import`, it automatically fetches transactions from ALL connected banks seamlessly.

### 2. Import Transactions

Import transactions from ALL your connected accounts automatically:

```bash
# Import last 30 days from all banks
spice import

# Import specific date range from all banks
spice import --from 2024-01-01 --to 2024-01-31

# List all accounts across all banks
spice import --list-accounts

# Import from specific accounts only
spice import --account "Chase Credit (...1234)" --account "Ally Checking (...5678)"
```

The import command automatically:
- Fetches from all connected banks in parallel
- Shows progress for each bank
- Merges transactions seamlessly
- Handles errors gracefully (if one bank fails, others still import)

### 3. Manage Categories

Categories are dynamically created and managed. Use AI to generate helpful descriptions:

```bash
# List all categories
spice categories list

# Add new category with AI-generated description
spice categories add "Healthcare"

# Add with custom description
spice categories add "Pets" --description "Pet supplies, vet visits, grooming"

# Update category name or description
spice categories update 5 --name "Medical & Health"
spice categories update 5 --regenerate  # Generate new AI description

# Delete unused category
spice categories delete 5
```

### 4. Classify Transactions

Run the AI-powered classification workflow:

```bash
# Start interactive classification
spice classify

# The CLI will:
# - Group transactions by vendor
# - Use AI to suggest categories
# - Let you review and approve/modify suggestions
# - Learn from your decisions for future classifications
# - Suggest new categories when confidence is low
```

#### Batch Classification Mode

For faster classification of high-confidence transactions, use batch mode:

```bash
# Run batch classification with default threshold (0.8)
spice classify --batch

# Set custom confidence threshold (0.0 to 1.0)
spice classify --batch --acceptance-threshold 0.9

# Combine with other options
spice classify --batch --from 2024-01-01 --to 2024-12-31
```

**How Batch Mode Works:**
- Automatically classifies transactions that meet the confidence threshold
- Shows a summary of auto-classified transactions for review
- Falls back to interactive mode for low-confidence transactions
- Perfect for recurring vendors and well-established spending patterns

**Confidence Thresholds:**
- `0.9-1.0`: Very conservative - only auto-classify near-certain matches
- `0.8` (default): Balanced - auto-classify high-confidence matches
- `0.7`: More aggressive - may auto-classify some ambiguous transactions
- `0.5-0.6`: Very aggressive - use with caution

**Best Practices:**
- Start with the default threshold (0.8) and adjust based on accuracy
- Run `spice classify` interactively first to train the system
- Review batch results regularly to ensure accuracy
- Use higher thresholds for financial/tax-critical categorization

### 5. Analyze Your Categorization

Use AI-powered analysis to identify issues and optimize your categorization:

```bash
# Analyze last 30 days (default)
spice analyze

# Analyze specific date range
spice analyze --start-date 2024-01-01 --end-date 2024-03-31

# Focus on specific aspects
spice analyze --focus patterns    # Pattern rule effectiveness
spice analyze --focus categories  # Category usage and distribution
spice analyze --focus coherence   # Overall consistency

# Auto-apply high-confidence fixes
spice analyze --auto-apply

# Preview fixes without applying
spice analyze --dry-run
```

**What the Analysis Detects:**
- **Inconsistent Categorization**: Same vendor split across multiple categories
- **Missing Patterns**: Recurring transactions without pattern rules
- **Ambiguous Vendors**: Merchants that legitimately span categories
- **Category Optimization**: Opportunities to reorganize categories

**Analysis Output:**
- **Coherence Score**: 0-100% rating of categorization consistency
- **Prioritized Issues**: Sorted by severity and impact
- **Actionable Fixes**: Specific recommendations with confidence scores
- **Insights**: High-level observations about your data

For detailed analysis documentation, see [AI Analysis User Guide](docs/AI_ANALYSIS_USER_GUIDE.md).

### 6. Export to Google Sheets

Export your categorized transactions to a comprehensive financial report:

```bash
# Export current month
spice export

# Export specific month
spice export --month 2024-01

# Export custom date range
spice export --from 2024-01-01 --to 2024-03-31

# Export full year (validates data coverage)
spice export --year 2024
```

**Export Format:**

The Google Sheets export creates a multi-tab financial report optimized for tax preparation:

1. **Expenses Tab**: All expense transactions with date, amount, vendor, category, and business percentage
2. **Income Tab**: All income transactions with source and category information
3. **Vendor Summary**: Vendor-to-category mappings with total spending per vendor
4. **Category Summary**: Category totals with business percentages and month-by-month breakdowns
5. **Business Expenses**: Pre-calculated business deductions for Schedule C tax filing
6. **Monthly Flow**: Cash flow analysis showing income vs expenses by month

**Key Features:**
- Automatic separation of income and expenses
- Business expense calculations for tax deductions
- Month-by-month trend analysis
- Data validation ensures all transactions are categorized
- Warning system for data gaps (30+ days without transactions)
- Professional formatting with currency display, bold headers, and frozen rows

**Export Requirements:**
- All transactions in the date range must be categorized
- The system validates data completeness before export
- Large datasets are exported in batches for reliability

### 7. Database Checkpoints

Save and restore your database state for safe experimentation:

```bash
# Create a checkpoint
spice checkpoint create --tag "before-year-end"
spice checkpoint create  # Auto-named with timestamp

# List all checkpoints
spice checkpoint list

# Restore a checkpoint
spice checkpoint restore before-year-end

# Compare checkpoints
spice checkpoint diff before-year-end current

# Export checkpoint for sharing
spice checkpoint export before-year-end --output my-categories.spice

# Import shared checkpoint
spice checkpoint import colleague-categories.spice

# Auto-checkpoint before risky operations
spice import --auto-checkpoint
```

### 8. Pattern-Based Classification

The spice-must-flow uses intelligent pattern rules for accurate transaction categorization. Pattern rules are more flexible than simple vendor rules because they consider multiple transaction attributes:

#### Understanding Pattern Rules

Pattern rules match transactions based on:
- **Merchant patterns**: Exact match or regex patterns (e.g., "AMAZON.*" matches all Amazon variants)
- **Amount conditions**: Less than, greater than, equals, ranges (e.g., "< $10" for small purchases)
- **Transaction direction**: Income, expense, or transfer-specific rules
- **Priority**: Higher priority rules override lower ones

#### Managing Pattern Rules

```bash
# List all pattern rules
spice patterns list

# Show detailed pattern information
spice patterns show <id>

# Create a new pattern rule
spice patterns create --name "Amazon Refunds" \
  --merchant "Amazon" \
  --direction income \
  --category "Refund" \
  --confidence 95

# Create complex patterns
spice patterns create --name "Small Coffee Purchases" \
  --merchant "Coffee|Starbucks|Peet" --regex \
  --amount-condition lt --amount-value 10 \
  --category "Dining" \
  --confidence 85

# Edit existing patterns
spice patterns edit <id> --confidence 90 --priority 10

# Delete patterns
spice patterns delete <id>

# Test patterns against transactions
spice patterns test --merchant "Amazon" --amount 25.00 --direction expense
```

#### Pattern Examples

**Example 1: Amazon Refunds**
```bash
spice patterns create \
  --name "Amazon Refunds" \
  --merchant "Amazon" \
  --direction income \
  --category "Refund" \
  --confidence 95 \
  --description "Amazon income transactions are usually refunds"
```

**Example 2: Small Food Purchases**
```bash
spice patterns create \
  --name "Quick Food" \
  --merchant ".*" --regex \
  --amount-condition lt --amount-value 20 \
  --category "Fast Food" \
  --confidence 70 \
  --description "Small transactions under $20 are often fast food"
```

**Example 3: Large Purchases Need Review**
```bash
spice patterns create \
  --name "Large Expense Review" \
  --amount-condition gt --amount-value 1000 \
  --category "Large Purchase" \
  --confidence 50 \
  --priority 100 \
  --description "Flag large purchases for manual review"
```

#### Pattern vs Vendor Rules

Pattern rules are the recommended approach over vendor rules because:

| Feature | Pattern Rules | Vendor Rules |
|---------|--------------|--------------|
| Direction validation | ✅ Yes | ❌ No |
| Amount conditions | ✅ Yes | ❌ No |
| Regex matching | ✅ Yes | ❌ No |
| Priority system | ✅ Yes | ❌ No |
| Use counting | ✅ Yes | ❌ No |
| Confidence scoring | ✅ Yes | ❌ No |

**Migration Tip**: Vendor rules still work but are deprecated. Pattern rules are checked first during classification.

### 9. Check Pattern Management

Automatically categorize check transactions based on patterns:

```bash
# List all check patterns
spice checks list

# Create a new pattern interactively
spice checks add

# Edit existing pattern
spice checks edit 1

# Delete a pattern
spice checks delete 1

# Test which patterns match a given amount
spice checks test 100.00
```

Check patterns help categorize recurring check payments like:
- Monthly cleaning services ($100 or $200 → Home Services)
- Rent payments ($3,000-$3,100 → Housing)
- Quarterly tax payments ($5,000-$6,000 → Taxes)

Example creating a pattern:
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

### 10. Migrating to Pattern-Based Classification

If you have existing classifications using vendor rules, here's how to migrate to the more powerful pattern-based system:

#### Step 1: Analyze Your Current Vendor Rules
```bash
# List all vendor rules to understand current mappings
spice vendors list
```

#### Step 2: Create Pattern Rules from Vendor Rules
For each vendor rule, create a more intelligent pattern rule:

```bash
# Old vendor rule: "AMAZON" → "Shopping"
# Better pattern rules:
spice patterns create --name "Amazon Shopping" \
  --merchant "Amazon" --direction expense \
  --category "Shopping" --confidence 85

spice patterns create --name "Amazon Refunds" \
  --merchant "Amazon" --direction income \
  --category "Refund" --confidence 95
```

#### Step 3: Identify Common Patterns
Look for transaction patterns that vendor rules miss:

```bash
# Find all transactions in "Miscellaneous" or "Other" categories
spice transactions list --category "Miscellaneous"

# Create patterns for common groups
spice patterns create --name "Small Subscriptions" \
  --amount-condition range --amount-min 5 --amount-max 20 \
  --merchant "Spotify|Netflix|Apple" --regex \
  --category "Subscriptions" --confidence 90
```

#### Step 4: Test Your Patterns
Before applying patterns to all transactions:

```bash
# Test specific scenarios
spice patterns test --merchant "Target" --amount 150 --direction expense

# Run a dry-run recategorization
spice recategorize --dry-run --from 2024-01-01
```

#### Step 5: Apply Patterns to Existing Transactions
```bash
# Recategorize all transactions to apply new patterns
spice recategorize --from 2024-01-01 --force

# Or recategorize specific problematic merchants
spice recategorize --merchant "AMZN MKTP"
spice recategorize --merchant "GOOGLE"
```

#### Migration Best Practices

1. **Start with high-confidence patterns**: Create patterns for obvious cases first
2. **Use direction wisely**: Separate income and expense patterns for the same merchant
3. **Leverage amount conditions**: Many merchants have predictable price ranges
4. **Monitor pattern usage**: Check `spice patterns list` to see use counts
5. **Iterate and refine**: Adjust confidence scores based on accuracy

#### Common Pattern Templates

**Subscription Services:**
```bash
spice patterns create --name "Monthly Subscriptions" \
  --merchant "Netflix|Spotify|HBO|Disney|Apple Music" --regex \
  --amount-condition range --amount-min 5 --amount-max 50 \
  --direction expense \
  --category "Entertainment" \
  --confidence 90
```

**Refunds and Returns:**
```bash
spice patterns create --name "General Refunds" \
  --direction income \
  --category "Refund" \
  --confidence 80 \
  --description "Most income transactions are refunds"
```

**Business Expenses:**
```bash
spice patterns create --name "Cloud Services" \
  --merchant "AWS|Google Cloud|Azure|DigitalOcean" --regex \
  --category "Business Services" \
  --confidence 95 \
  --priority 10
```

### 11. Recategorize Transactions

Fix misclassified transactions or apply new pattern/vendor rules retroactively:

```bash
# Recategorize specific merchant transactions
spice recategorize --merchant "AMAZON"

# Recategorize all transactions in a category
spice recategorize --category "Miscellaneous"

# Recategorize by date range
spice recategorize --from 2024-01-01 --to 2024-12-31

# Combine filters
spice recategorize --merchant "AUTOMATIC PAYMENT" --from 2024-01-01

# Preview without making changes
spice recategorize --category "Other" --dry-run

# Skip confirmation prompt
spice recategorize --merchant "STARBUCKS" --force
```

**How recategorization works:**
1. Finds transactions matching your criteria
2. Clears their existing classifications
3. Re-runs AI classification on ONLY those transactions
4. Applies pattern rules first, then vendor rules and check patterns
5. Lets you review and approve new suggestions

**Common use cases:**
- **After adding pattern rules**: Create a pattern rule, then recategorize matching transactions
- **After adding vendor rules**: Create a rule, then recategorize past transactions
- **Fixing bulk mistakes**: When many transactions were wrongly categorized
- **Category reorganization**: After splitting or merging categories
- **Model improvements**: Apply better AI categorization to historical data

Example workflows:

**Pattern Rule Example:**
```bash
# Create pattern for Amazon refunds, then recategorize
$ spice patterns create --name "Amazon Refunds" --merchant "Amazon" \
    --direction income --category "Refund" --confidence 95
$ spice recategorize --merchant "Amazon"

🔄 Starting recategorization...
🤖 Running AI classification...

📊 Recategorization Summary:
  Total transactions: 24
  Auto-accepted: 15      # Income transactions now categorized as Refund
  Manually reviewed: 9   # Expense transactions for manual review

✓ Recategorization complete!
```

**Vendor Rule Example:**
```bash
# Realize all "AUTOMATIC PAYMENT - THANK" should be Credit Card Payments
$ spice vendors create "AUTOMATIC PAYMENT - THANK" "Credit Card Payments"
$ spice recategorize --merchant "AUTOMATIC PAYMENT - THANK"

🔄 Starting recategorization...
🤖 Running AI classification...

📊 Recategorization Summary:
  Total transactions: 12
  Auto-accepted: 12      # Now uses vendor rule
  Manually reviewed: 0

✓ Recategorization complete!
```

### Additional Commands

```bash
# Authentication
spice auth plaid                      # Connect bank accounts via Plaid
spice auth sheets                     # Authenticate with Google Sheets

# Manage vendor rules
spice vendors list                    # List all vendor rules
spice vendors add "Starbucks" "Food"  # Add manual rule
spice vendors remove "Starbucks"      # Remove rule

# Manage categories
spice categories list                 # List all categories with descriptions
spice categories add "Travel"         # Add with AI description
spice categories update 5 --regenerate # Update with new AI description
spice categories delete 5             # Soft delete category

# Manage pattern rules
spice patterns list                   # List all pattern rules
spice patterns show <id>              # Show detailed pattern info
spice patterns create                 # Create new pattern rule
spice patterns edit <id>              # Edit existing pattern
spice patterns delete <id>            # Delete pattern
spice patterns test                   # Test pattern matching

# Manage check patterns
spice checks list                     # List all check patterns
spice checks add                      # Create pattern interactively
spice checks edit <id>               # Edit existing pattern
spice checks delete <id>             # Delete pattern
spice checks test <amount>           # Test pattern matching

# Recategorize transactions
spice recategorize --merchant "AMAZON"   # Re-classify all Amazon transactions
spice recategorize --category "Other"    # Re-classify all "Other" transactions
spice recategorize --from 2024-01-01     # Re-classify transactions since date
spice recategorize --dry-run             # Preview what would be recategorized

# Database operations
spice migrate                         # Run database migrations
spice flow                           # Run full workflow (import → classify → export)

# Checkpoint management
spice checkpoint create               # Create timestamped checkpoint
spice checkpoint list                 # List all checkpoints
spice checkpoint restore <name>       # Restore from checkpoint
spice checkpoint diff <name>          # Compare with current state

# Bank information
spice institutions search "bank name" # Search for banks and see OAuth requirements

# Help
spice help                           # Show all commands
spice help [command]                 # Show help for specific command
```

## Architecture

The project follows clean architecture principles with interface-driven design:

```
cmd/spice/          # CLI commands using cobra
internal/
  model/            # Core domain models (Transaction, Category, Vendor)
  service/          # Service interfaces
  storage/          # SQLite storage with migrations
  llm/              # Multi-provider AI classification (OpenAI, Anthropic, Claude Code)
  plaid/            # Plaid API client
  sheets/           # Google Sheets export
  engine/           # Core orchestration logic
  pattern/          # Pattern-based classification system
  cli/              # CLI utilities and styling
```

Key design patterns:
- **Interface-First Design**: All services defined as interfaces for testability
- **Dependency Injection**: Clean separation of concerns
- **Repository Pattern**: Abstract data access through interfaces
- **Command Pattern**: CLI commands as self-contained units
- **Dynamic Categories**: Categories evolve based on usage patterns
- **Checkpoint System**: Database snapshots for safe experimentation
- **Pattern-Based Rules**: Flexible classification using merchant, amount, and direction patterns

## Development

### Running Tests

```bash
# Run unit tests
make test

# Run all tests (includes integration tests)
make test-all

# Run with race detection
go test -race ./...
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Auto-fix common issues
make fix
```

### Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`make test-all`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Project name inspired by Frank Herbert's Dune
- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- [Lipgloss](https://github.com/charmbracelet/lipgloss) for beautiful terminal UI
- [Plaid](https://plaid.com) for financial data access
- [OpenAI](https://openai.com) and [Anthropic](https://anthropic.com) for AI classification