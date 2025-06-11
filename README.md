# the-spice-must-flow

> "He who controls the spice controls the universe." - Frank Herbert, Dune

A personal finance categorization engine that ingests financial transactions from Plaid, uses AI to intelligently categorize them, and exports reports to Google Sheets. Built with Go, it emphasizes testability, extensibility, and a delightful CLI experience.

## Features

- 🏦 **Plaid Integration**: Automatically import transactions from your bank accounts
- 🤖 **AI-Powered Categorization**: Uses OpenAI or Anthropic to intelligently categorize transactions
- 📊 **Google Sheets Export**: Export categorized transactions to Google Sheets for reporting
- 🔄 **Smart Deduplication**: SHA256 hashing prevents duplicate transaction processing
- 📚 **Learning System**: Remembers vendor categorizations to auto-categorize future transactions
- 🎯 **Batch Processing**: Groups transactions by merchant for efficient review
- 💾 **Resume Support**: Classification sessions can be paused and resumed
- 🎨 **Beautiful CLI**: Interactive command-line interface with progress indicators

## Installation

### Prerequisites

- Go 1.21 or higher
- SQLite3
- Make

### Building from Source

```bash
# Clone the repository
git clone https://github.com/joshsymonds/the-spice-must-flow.git
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

## Usage

### 1. Connect Your Bank Accounts

Connect your bank accounts using Plaid:

```bash
# Connect your first bank (e.g., Chase credit card)
spice auth plaid

# Connect additional banks (e.g., Ally checking)
spice auth plaid

# This will:
# - Open your browser to Plaid Link
# - Let you securely connect your bank
# - Save the access token for future use
```

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

### 3. Classify Transactions

Run the AI-powered classification workflow:

```bash
# Start interactive classification
spice classify

# The CLI will:
# - Group transactions by vendor
# - Use AI to suggest categories
# - Let you review and approve/modify suggestions
# - Learn from your decisions for future classifications
```

### 4. Export to Google Sheets

Export your categorized transactions:

```bash
# Export current month
spice export

# Export specific month
spice export --month 2024-01

# Export custom date range
spice export --from 2024-01-01 --to 2024-03-31
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

# Database operations
spice migrate                         # Run database migrations
spice flow                           # Run full workflow (import → classify → export)

# Help
spice help                           # Show all commands
spice help [command]                 # Show help for specific command
```

## Architecture

The project follows clean architecture principles with interface-driven design:

```
cmd/spice/          # CLI commands using cobra
internal/
  model/            # Core domain models
  service/          # Service interfaces
  storage/          # SQLite storage implementation
  llm/              # AI classification service
  plaid/            # Plaid API client
  sheets/           # Google Sheets export
  engine/           # Core orchestration logic
  cli/              # CLI utilities and styling
```

Key design patterns:
- **Interface-First Design**: All services defined as interfaces for testability
- **Dependency Injection**: Clean separation of concerns
- **Repository Pattern**: Abstract data access through interfaces
- **Command Pattern**: CLI commands as self-contained units

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