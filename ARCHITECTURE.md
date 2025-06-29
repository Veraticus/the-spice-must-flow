# **the-spice-must-flow: Personal Finance Categorization Engine**

## **1. Core Mission & Guiding Principles**

**Mission:** To create a robust command-line application that ingests a year's worth of financial transactions from Plaid, intelligently categorizes them with minimal user intervention, and exports a clean, accountant-ready report to Google Sheets.

**Guiding Principles:**
* **Testability First:** The architecture will be driven by interfaces and dependency injection, enabling comprehensive unit testing with minimal friction.
* **Extensibility:** Components will be decoupled, making it easy to add new data sources, classifiers, or output formats in the future.
* **Resilience:** The application will gracefully handle API failures, interruptions, and data inconsistencies.
* **Idiomatic Go:** We will adhere to Go community best practices for project layout, error handling, concurrency, and style.
* **User-Friendly:** Progress can be saved and resumed, similar transactions can be reviewed in batches, and all operations provide clear feedback.
* **Delightful:** While maintaining professionalism, the tool should have personality and be enjoyable to use.

---

## **2. Core Types: Interfaces & Structs**

### **Core Models**

```go
package model

import (
    "time"
    "crypto/sha256"
    "fmt"
)

// Transaction represents a single financial transaction from Plaid.
type Transaction struct {
    ID           string
    Date         time.Time
    Name         string    // Raw name from the statement
    MerchantName string    // Plaid's cleaned merchant name
    Amount       float64
    PlaidCategory []string // Plaid's categorization hint
    AccountID    string    // To track which account this came from
    Hash         string    // SHA256 hash for deduplication
}

// GenerateHash creates a unique hash for duplicate detection
func (t *Transaction) GenerateHash() string {
    data := fmt.Sprintf("%s:%.2f:%s:%s", 
        t.Date.Format("2006-01-02"), 
        t.Amount, 
        t.MerchantName,
        t.AccountID)
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash)
}

// Vendor represents a known merchant with a user-confirmed category.
type Vendor struct {
    Name         string // The cleaned merchant name (primary key)
    Category     string
    LastUpdated  time.Time
    UseCount     int // Track how often this rule is used
}

// ClassificationStatus indicates how a transaction was categorized.
type ClassificationStatus string

const (
    StatusUnclassified      ClassificationStatus = "UNCLASSIFIED"
    StatusClassifiedByRule  ClassificationStatus = "CLASSIFIED_BY_RULE"
    StatusClassifiedByAI    ClassificationStatus = "CLASSIFIED_BY_AI"
    StatusUserModified      ClassificationStatus = "USER_MODIFIED"
)

// Category represents a valid expense category.
type Category struct {
    ID           int
    Name         string
    CreatedAt    time.Time
    IsActive     bool
}

// Classification represents a transaction after processing.
type Classification struct {
    Transaction  Transaction
    Category     string
    Status       ClassificationStatus
    Confidence   float64   // AI confidence score (0-1)
    ClassifiedAt time.Time
    Notes        string    // e.g., "75% of this expense is for business"
}

// PendingClassification represents a transaction awaiting user confirmation
type PendingClassification struct {
    Transaction       Transaction
    SuggestedCategory string
    Confidence        float64
    SimilarCount      int // Number of similar transactions
}

// ClassificationProgress tracks where we are in a classification run
type ClassificationProgress struct {
    LastProcessedID   string
    LastProcessedDate time.Time
    TotalProcessed    int
    StartedAt         time.Time
}
```

### **Service Interfaces**

```go
package service

import (
    "context"
    "time"
    "thespicemustflow/internal/model"
)

// TransactionFetcher defines the contract for fetching data from Plaid.
type TransactionFetcher interface {
    GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error)
    GetAccounts(ctx context.Context) ([]string, error) // For account filtering
}

// Storage defines the contract for our persistence layer.
type Storage interface {
    // Transaction operations
    SaveTransactions(ctx context.Context, transactions []model.Transaction) error
    GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error)
    GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error)
    
    // Vendor operations
    GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error)
    SaveVendor(ctx context.Context, vendor *model.Vendor) error
    GetAllVendors(ctx context.Context) ([]model.Vendor, error)
    
    // Category operations
    GetCategories(ctx context.Context) ([]model.Category, error)
    GetCategoryByName(ctx context.Context, name string) (*model.Category, error)
    CreateCategory(ctx context.Context, name string) (*model.Category, error)
    
    // Classification operations
    SaveClassification(ctx context.Context, classification *model.Classification) error
    GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error)
    
    // Progress tracking
    SaveProgress(ctx context.Context, progress *model.ClassificationProgress) error
    GetLatestProgress(ctx context.Context) (*model.ClassificationProgress, error)
    
    // Database management
    Migrate(ctx context.Context) error
    BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
    Commit() error
    Rollback() error
    // Include all Storage methods for use within transaction
    Storage
}

// LLMClassifier defines the contract for AI-based categorization.
type LLMClassifier interface {
    // SuggestCategory returns a category suggestion with confidence.
    // If confidence < 0.9, the category may be a new suggestion.
    // The categories parameter contains all valid existing categories.
    SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (category string, confidence float64, isNew bool, err error)
    BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]LLMSuggestion, error)
}

// LLMSuggestion represents a single classification suggestion
type LLMSuggestion struct {
    TransactionID     string
    Category          string
    Confidence        float64
    IsNewCategory     bool   // true if this is a suggested new category
}

// UserPrompter defines the contract for user interaction.
type UserPrompter interface {
    ConfirmClassification(merchantName, suggestedCategory string, confidence float64) (category string, confirmed bool)
    BatchReview(classifications []model.PendingClassification) ([]model.Classification, error)
    ShowProgress(current, total int, currentMerchant string)
    ShowCompletion(stats CompletionStats)
}

// CompletionStats shows the results of a classification run
type CompletionStats struct {
    TotalTransactions   int
    AutoClassified      int
    UserClassified      int
    NewVendorRules      int
    Duration            time.Duration
}

// ReportWriter defines the contract for output generation.
type ReportWriter interface {
    Write(ctx context.Context, classifications []model.Classification, summary *ReportSummary) error
}

// ReportSummary contains aggregate information for the report
type ReportSummary struct {
    DateRange     DateRange
    TotalAmount   float64
    ByCategory    map[string]CategorySummary
    ClassifiedBy  map[model.ClassificationStatus]int
}

type DateRange struct {
    Start time.Time
    End   time.Time
}

type CategorySummary struct {
    Count  int
    Amount float64
}

// Retry defines a common interface for retryable operations
type Retryable interface {
    WithRetry(ctx context.Context, operation func() error, opts RetryOptions) error
}

type RetryOptions struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
}
```

---

## **3. Project Structure**

```
thespicemustflow/
├── cmd/
│   └── spice/
│       ├── main.go              # Entry point, DI setup
│       ├── classify.go          # Classify command implementation
│       ├── vendors.go           # Vendor management commands
│       ├── flow.go              # Flow report command
│       └── migrate.go           # Database migration command
├── internal/
│   ├── model/                   # Core domain models
│   │   ├── transaction.go
│   │   ├── vendor.go
│   │   └── classification.go
│   ├── service/                 # Service interfaces
│   │   └── interfaces.go
│   ├── plaid/                   # Plaid client implementation
│   │   ├── client.go
│   │   └── client_test.go
│   ├── storage/                 # SQLite storage implementation
│   │   ├── sqlite.go
│   │   ├── migrations.go
│   │   ├── transactions.go
│   │   ├── vendors.go
│   │   └── storage_test.go
│   ├── llm/                     # LLM classifier implementation
│   │   ├── openai.go
│   │   ├── prompts.go
│   │   └── llm_test.go
│   ├── cli/                     # CLI user interaction
│   │   ├── prompter.go
│   │   ├── batch_review.go
│   │   ├── progress.go
│   │   └── styles.go           # Consistent styling
│   ├── sheets/                  # Google Sheets report writer
│   │   ├── writer.go
│   │   └── formatter.go
│   ├── engine/                  # Core classification engine
│   │   ├── classifier.go
│   │   ├── batch_processor.go
│   │   ├── merchant_patterns.go
│   │   └── engine_test.go
│   ├── pattern/                 # Pattern-based classification
│   │   ├── interfaces.go
│   │   ├── matcher.go
│   │   ├── validator.go
│   │   └── suggester.go
│   ├── analysis/                # AI-powered analysis system
│   │   ├── engine.go
│   │   ├── session.go
│   │   ├── validator.go
│   │   ├── types.go
│   │   └── formatter.go
│   └── common/                  # Shared utilities
│       ├── retry.go
│       ├── logger.go
│       └── errors.go
├── pkg/
│   └── ratelimit/              # Rate limiting utilities
│       └── limiter.go
├── testdata/                    # Test fixtures
│   ├── transactions.json
│   └── vendors.csv
├── go.mod
├── go.sum
├── config.yaml                  # Default configuration
├── Makefile                     # Build and test automation
└── README.md
```

---

## **4. User Experience Specification**

### **Command Structure**

The `spice` CLI follows a verb-noun pattern with intuitive shortcuts:

```bash
# Primary commands
spice classify    # Categorize transactions
spice vendors     # Manage vendor rules  
spice patterns    # Manage pattern-based rules
spice analyze     # AI-powered transaction analysis
spice flow        # View spending flow reports
spice migrate     # Run database migrations

# Common flags
--year, -y       # Specify year (e.g., 2024)
--month, -m      # Specify month (e.g., 2024-01)
--resume, -r     # Resume interrupted session
--dry-run        # Preview without saving
```

### **Classification User Flow**

#### **Initial Run Experience**
```bash
$ spice classify --year 2024
🌶️  Starting transaction categorization...
✓ Connected to Plaid
✓ Fetched 1,247 transactions from 3 accounts
✓ Found 423 unique merchants
✓ 89 merchants have existing rules

Beginning classification...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 0% | 0/1247 | Initializing...
```

#### **Batch Review Interface**

When encountering a new merchant with multiple transactions:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 15% | 187/1247 | Starbucks

╭─ Review: Starbucks ─────────────────────────────────────────╮
│ Found 23 transactions                                       │
│ Total: $127.45                                             │
│ Date range: Jan 3 - Dec 28, 2024                          │
│                                                            │
│ 🤖 AI suggests: Coffee & Dining (92% confidence)          │
╰────────────────────────────────────────────────────────────╯

[A]ccept all  [C]ustom category  [R]eview each  [S]kip → a

✓ Created rule: Starbucks → Coffee & Dining
✓ Categorized 23 transactions
```

#### **High-Variance Merchant Detection**

For merchants like Amazon with varying purchase types:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 17% | 210/1247 | Amazon

╭─ Review: Amazon ────────────────────────────────────────────╮
│ Found 47 transactions                                       │
│ Total: $2,847.23                                           │
│ Amount range: $3.99 - $487.23 (122x difference)           │
│                                                            │
│ ⚠️  Large variance detected                                │
│ These may be different types of purchases                  │
╰────────────────────────────────────────────────────────────╯

[A]ccept "Shopping" for all  [R]eview each  [S]kip → r

┌─ Transaction 1 of 47 ───────────────────────────────────────┐
│ Date: 2024-01-15           Amount: $487.23                 │
│ Description: AMAZON.COM*RT4Y7HG2                           │
│                                                            │
│ 🤖 AI suggests: Computer & Electronics (85%)               │
└─────────────────────────────────────────────────────────────┘

[A]ccept  [C]ustom  [S]kip → a
✓ Computer & Electronics
```

#### **Smart Pattern Detection**

After several similar categorizations:

```
┌─ Pattern Detected ──────────────────────────────────────────┐
│ 🔍 The last 5 Amazon transactions under $50 were all       │
│    categorized as "Office Supplies"                        │
│                                                            │
│ Apply "Office Supplies" to the remaining 31 transactions   │
│ under $50?                                                 │
└─────────────────────────────────────────────────────────────┘

[Y]es  [N]o, continue individually → y

✓ Applied "Office Supplies" to 31 transactions
  11 transactions over $50 remaining for review...
```

#### **Interruption and Resume**

```bash
# User presses Ctrl+C
^C
⚠️  Classification interrupted
✓ Progress saved automatically
  • Processed: 487 of 1,247 transactions
  • Last merchant: "Whole Foods Market"
  • Time elapsed: 12m 34s

Resume with: spice classify --resume

# Later...
$ spice classify --resume
🌶️  Resuming previous session...
✓ Found saved progress from 2 hours ago
  • Continuing from transaction 488 of 1,247
  • 89 vendor rules available (+3 new from last session)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 39% | 487/1247 | Whole Foods Market
```

#### **Completion Summary**

```bash
✅ Classification complete!

╭─ Summary ───────────────────────────────────────────────────╮
│ Total transactions:      1,247                              │
│ Auto-classified:         1,089 (87.3%)                     │
│ User-classified:           158 (12.7%)                     │
│                                                            │
│ New vendor rules:           34                             │
│ Time taken:              23m 17s                           │
│ Time saved:              ~3 hours (estimated)              │
│                                                            │
│ Ready for export: spice flow --export                      │
╰────────────────────────────────────────────────────────────╯
```

### **Vendor Management Flow**

```bash
$ spice vendors list
┌─────────────────────────┬──────────────────────┬───────────┬──────────┐
│ Merchant                │ Category             │ Used      │ Updated  │
├─────────────────────────┼──────────────────────┼───────────┼──────────┤
│ Starbucks              │ Coffee & Dining      │ 23 times  │ Today    │
│ Amazon                 │ (Multiple)           │ 47 times  │ Today    │
│ Shell Oil              │ Transportation       │ 18 times  │ Today    │
│ Netflix                │ Entertainment        │ 12 times  │ Dec 20   │
└─────────────────────────┴──────────────────────┴───────────┴──────────┘
Showing 4 of 89 vendor rules

$ spice vendors search whole foods
Found 1 vendor:
• Whole Foods Market → Groceries (used 52 times)

$ spice vendors edit "Whole Foods Market"
Current category: Groceries
New category: Food & Grocery
✓ Updated vendor rule
ℹ️  This affects future classifications only
  To reclassify existing: spice classify --merchant "Whole Foods Market"
```

### **Flow Reports**

```bash
$ spice flow --year 2024
🌶️  Analyzing your financial flow...

╭─ 2024 Financial Flow ───────────────────────────────────────╮
│                                                            │
│ Total outflow: $67,234.89                                  │
│ Transactions: 1,247                                        │
│ Categories: 23                                             │
│                                                            │
│ Top Categories:                                            │
│ ┌──────────────────────┬──────────┬─────────┬──────────┐ │
│ │ Category             │ Amount   │ Trans   │ % Total  │ │
│ ├──────────────────────┼──────────┼─────────┼──────────┤ │
│ │ 🏠 Housing           │ $24,000  │ 12      │ 35.7%    │ │
│ │ 🍽️  Food & Dining    │ $8,235   │ 234     │ 12.3%    │ │
│ │ 🚗 Transportation    │ $4,123   │ 89      │ 6.1%     │ │
│ │ 🛍️  Shopping         │ $3,987   │ 156     │ 5.9%     │ │
│ │ 💳 Subscriptions     │ $2,341   │ 48      │ 3.5%     │ │
│ └──────────────────────┴──────────┴─────────┴──────────┘ │
│                                                            │
│ Export to Google Sheets: spice flow --export               │
╰────────────────────────────────────────────────────────────╯
```

### **Error States and Messaging**

```bash
# Connection errors
❌ Unable to connect to Plaid
   Please check your internet connection and API credentials

# API limits
⚠️  AI categorization temporarily unavailable (rate limit)
   Switching to manual mode...
   Enter category for "Target":

# Invalid input
❌ Category cannot be empty
   Please enter a valid category or press [S] to skip:

# Duplicate detection
ℹ️  Skipped 3 duplicate transactions already processed

# No transactions
ℹ️  No new transactions found for January 2025
   Your financial records are up to date! 🎉
```

---

## **5. Dynamic Category System**

### **Overview**

The categorization system uses a dynamic, learning-based approach where categories evolve based on actual usage:

1. **Database-Driven Categories**: Categories are stored in the database, not hardcoded
2. **Confidence-Based Evolution**: LLM suggests new categories when confidence < 85%
3. **User-Controlled Growth**: Users decide whether to create new categories
4. **Natural Taxonomy**: Categories emerge from actual spending patterns

### **How It Works**

```
1. Known merchant → Use existing vendor rule
2. New merchant → Ask LLM with current categories
3. High confidence (≥85%) → Suggest existing category
4. Low confidence (<85%) → Suggest NEW category
5. User decides → Create new or map to existing
```

### **Category Descriptions**

Categories now include AI-generated descriptions to improve classification accuracy:

```go
type Category struct {
    ID          int
    Name        string
    Description string    // AI-generated explanation of what belongs here
    CreatedAt   time.Time
    IsActive    bool
}
```

When creating categories via CLI:
- Descriptions are automatically generated using the configured LLM
- Users can override with custom descriptions
- Descriptions help both humans and AI understand category boundaries

### **Example Flow**

```bash
# First transaction for a new type
Classifying: PELOTON SUBSCRIPTION
Current categories: [Entertainment, Shopping, Utilities, Food & Dining]
LLM confidence: 72% (below threshold)

Suggested NEW category: Fitness & Health
[N] Create new category "Fitness & Health"
[E] Pick from existing categories
[C] Enter custom name
> N
✓ Created category "Fitness & Health"

# Later transaction
Classifying: PLANET FITNESS
Current categories: [..., Fitness & Health]  
Suggested: Fitness & Health (97% confidence) ✓
```

### **Benefits**

- **No Predefined Limits**: Categories match your actual spending
- **Intelligent Suggestions**: LLM understands context
- **Clean Data**: High confidence threshold prevents mis-categorization
- **User Control**: Always have final say on new categories

### **Initial Categories**

The system can be seeded with basic categories on first run:
- Food & Dining
- Transportation  
- Shopping
- Utilities
- Entertainment
- Other Expenses

Or start empty and let categories emerge naturally.

---

## **6. LLM Integration**

### **Overview**

The application supports multiple LLM providers for intelligent transaction classification and category description generation:

- **OpenAI** (GPT-3.5, GPT-4)
- **Anthropic** (Claude 3 Opus, Claude 3 Sonnet)
- **Claude Code** (Local Claude CLI integration)

### **LLM Client Architecture**

```go
// Client interface for all LLM providers
type Client interface {
    Classify(ctx context.Context, prompt string) (ClassificationResponse, error)
    GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error)
}

// Unified configuration
type Config struct {
    Provider       string  // "openai", "anthropic", or "claudecode"
    APIKey         string
    Model          string
    Temperature    float64
    MaxTokens      int
    ClaudeCodePath string  // Path to claude CLI for claudecode provider
}
```

### **Features**

1. **Unified Interface**: Single interface supports all providers
2. **Intelligent Caching**: Reduces API calls and costs
3. **Rate Limiting**: Prevents API throttling
4. **Retry Logic**: Handles transient failures gracefully
5. **Cost Tracking**: Logs API usage for cost monitoring

### **Claude Code Integration**

Special support for local Claude CLI:
```yaml
llm:
  provider: claudecode
  claude_code_path: /path/to/claude  # Custom CLI path
  model: sonnet
```

Benefits:
- No API keys required
- Works offline with Claude Code
- Supports custom Claude installations

---

## **7. Category Management System**

### **CLI Commands**

Complete CRUD operations for categories:

```bash
# List all categories with descriptions
spice categories list

# Add new category with AI description
spice categories add "Healthcare"

# Add with custom description
spice categories add "Pets" --description "Pet supplies, vet visits, grooming"

# Skip AI description generation
spice categories add "Misc" --no-description

# Update category
spice categories update <id> --name "New Name"
spice categories update <id> --description "New description"
spice categories update <id> --regenerate  # New AI description

# Delete category (soft delete)
spice categories delete <id> [--force]
```

### **Storage Interface**

```go
type Storage interface {
    // Category operations
    GetCategories(ctx context.Context) ([]model.Category, error)
    GetCategoryByName(ctx context.Context, name string) (*model.Category, error)
    CreateCategory(ctx context.Context, name, description string) (*model.Category, error)
    UpdateCategory(ctx context.Context, id int, name, description string) error
    DeleteCategory(ctx context.Context, id int) error
}
```

### **Features**

1. **AI-Generated Descriptions**: Automatic helpful descriptions for new categories
2. **Soft Deletes**: Categories marked inactive, not removed
3. **Usage Protection**: Can't delete categories with classified transactions
4. **Description Regeneration**: Update descriptions with latest AI models

---

## **8. Database Checkpoint System**

### **Overview**

A comprehensive checkpoint system provides database backups, rollback capabilities, and configuration sharing:

```
~/.local/share/spice/
├── spice.db              # Active database
└── checkpoints/
    ├── before-import.db
    ├── before-import.meta.json
    └── checkpoint-2024-06-20-1430.db
```

### **Checkpoint Commands**

```bash
# Create checkpoint
spice checkpoint create --tag "before-year-end"
spice checkpoint create  # Auto-named: checkpoint-YYYY-MM-DD-HHMM

# List checkpoints with metadata
spice checkpoint list
NAME                CREATED              SIZE    TRANSACTIONS  CATEGORIES
before-import      2024-06-20 14:30:00  1.2MB   1,234         15
year-end-backup    2024-12-31 23:59:00  2.1MB   3,456         22

# Restore checkpoint
spice checkpoint restore before-import
> This will replace your current database. Continue? (y/N)

# Compare checkpoints
spice checkpoint diff before-import current
+ 222 new transactions
+ 3 new categories
~ 45 recategorized transactions

# Export for sharing
spice checkpoint export before-import --output my-categories.spice

# Import shared checkpoint
spice checkpoint import colleague-categories.spice
```

### **Auto-Checkpoint Integration**

Automatic checkpoints before risky operations:
```bash
spice import --auto-checkpoint     # Creates checkpoint before import
spice classify --reset-vendors     # Auto-checkpoint before reset
```

### **Checkpoint Metadata**

Each checkpoint stores:
```json
{
  "created_at": "2024-06-20T14:30:00Z",
  "description": "Before Q2 import",
  "file_size": 1258291,
  "row_counts": {
    "transactions": 1234,
    "categories": 15,
    "vendors": 89,
    "classifications": 1234
  },
  "schema_version": 6,
  "is_auto": false
}
```

### **Configuration**

```yaml
checkpoint:
  auto_checkpoint: true      # Enable auto-checkpoints
  retention_days: 30        # Auto-cleanup old checkpoints
  max_checkpoints: 10       # Limit checkpoint count
```

### **Use Cases**

1. **Before Major Imports**: Safely import new data
2. **Category Experiments**: Try new categorization schemes
3. **Sharing Configurations**: Export/import category setups
4. **Disaster Recovery**: Restore from corruption
5. **Historical Analysis**: Compare spending over time

---

## **9. Google Sheets Export Format**

### **Overview**

The Google Sheets export creates a comprehensive financial report optimized for tax preparation and financial analysis. The multi-tab structure separates income and expenses, provides vendor and category summaries, and calculates business expense deductions.

### **Tab Structure**

#### **Tab 1: Expenses**
All expense transactions excluding income, with columns:
- Date (YYYY-MM-DD format)
- Amount (always positive)
- Vendor
- Category
- Business % (0-100)
- Notes

Sorted by date descending (newest first).

#### **Tab 2: Income**
All income transactions, with columns:
- Date
- Amount
- Source (vendor/payer)
- Category
- Notes

#### **Tab 3: Vendor Summary**
Aggregated vendor data showing:
- Vendor Name
- Associated Category
- Total Amount
- Transaction Count

Sorted by total amount descending.

#### **Tab 4: Category Summary**
Category-level analysis including:
- Category Name
- Type (Income/Expense)
- Total Amount
- Transaction Count
- Business % (for expense categories)
- Month-by-month breakdown (Jan-Dec columns)

Separate sections for Income and Expense categories.

#### **Tab 5: Business Expenses**
Tax-focused view showing only business-related expenses:
- Date
- Vendor
- Category
- Original Amount
- Business %
- Deductible Amount (Amount × Business%)
- Notes

Grouped by category with subtotals and grand total for Schedule C.

#### **Tab 6: Monthly Flow**
Cash flow analysis with:
- Month
- Total Income
- Total Expenses
- Net Flow (Income - Expenses)
- Running Balance

Includes yearly totals and averages.

### **Data Validation**

Before export, the system ensures:
1. **No unclassified transactions**: All transactions in the date range must be categorized
2. **Data continuity**: Warns about gaps of 30+ days without transactions
3. **Amount integrity**: All amounts stored as positive values (direction indicated by category type)

### **Formatting Applied**

- **Currency**: All amounts formatted as $#,##0.00
- **Headers**: Bold section headers and column titles
- **Frozen rows**: Header rows frozen for scrolling
- **Column sizing**: Auto-sized for optimal readability
- **Conditional formatting**: Negative values in Monthly Flow highlighted
- **Borders**: Section separators for visual clarity

### **Implementation Details**

The export process:
1. Validates transaction completeness
2. Aggregates data by vendor and category
3. Calculates business expense deductions
4. Generates month-by-month breakdowns
5. Writes to sheets in batches (1000 rows) to avoid API limits
6. Applies formatting in a single batch operation

### **API Considerations**

- **Batch operations**: Minimize API calls by batching writes
- **Retry logic**: Exponential backoff for transient failures
- **Clear before write**: Removes existing data to prevent duplicates
- **Progress feedback**: Updates user on export progress

---

## **11. AI-Powered Analysis System**

### **Overview**

The AI-powered analysis system examines an entire year of transaction data to identify categorization inconsistencies, discover patterns, and suggest optimizations. This system moves beyond simple transaction-by-transaction classification to provide holistic insights about financial categorization quality.

### **Core Capabilities**

1. **Coherence Analysis**: Identifies merchants split across multiple categories inappropriately
2. **Pattern Discovery**: Finds behavioral patterns that can be codified into rules
3. **Category Optimization**: Suggests merging redundant categories or creating missing ones
4. **Anomaly Detection**: Flags unusual transactions or potential errors

### **Architecture**

The analysis system (`internal/analysis/`) consists of:
- **AnalysisEngine**: Orchestrates the analysis workflow
- **SessionManager**: Maintains LLM context across validation rounds
- **JSONValidator**: Ensures well-formed output from LLM
- **ReportFormatter**: Presents actionable insights to users

### **Integration with Pattern System**

The analysis system generates pattern rules as output, not input:
```
User Data → AI Analysis → Discovered Patterns → Pattern Rules → Future Classifications
```

This creates a learning loop where the system improves over time based on actual usage patterns.

### **Key Design Decisions**

1. **Opus-Only Analysis**: Uses Claude 3 Opus exclusively for complex reasoning
2. **Full-Year Context**: Analyzes entire years at once (fits within 200k token window)
3. **JSON Output with Validation**: Structured output with recovery mechanisms
4. **Session Persistence**: Allows recovery from failures during long analyses

For detailed design documentation, see `docs/AI_ANALYSIS_DESIGN.md`.

---

## **12. Implementation Details**

### **Retry Logic Implementation**

```go
// internal/common/retry.go
package common

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "time"
)

var (
    ErrRateLimit = errors.New("rate limit exceeded")
    ErrMaxRetries = errors.New("max retries exceeded")
)

type RetryableError struct {
    Err error
    Retryable bool
}

func (e *RetryableError) Error() string {
    return e.Err.Error()
}

func WithRetry(ctx context.Context, operation func() error, opts RetryOptions) error {
    if opts.MaxAttempts <= 0 {
        opts.MaxAttempts = 3
    }
    if opts.InitialDelay <= 0 {
        opts.InitialDelay = 100 * time.Millisecond
    }
    if opts.MaxDelay <= 0 {
        opts.MaxDelay = 30 * time.Second
    }
    if opts.Multiplier <= 0 {
        opts.Multiplier = 2.0
    }

    delay := opts.InitialDelay
    
    for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        // Check if error is retryable
        var retryableErr *RetryableError
        if errors.As(err, &retryableErr) && !retryableErr.Retryable {
            return err
        }
        
        // Special handling for rate limits
        if errors.Is(err, ErrRateLimit) {
            delay = opts.MaxDelay
        }
        
        if attempt == opts.MaxAttempts {
            return fmt.Errorf("%w after %d attempts: %v", ErrMaxRetries, opts.MaxAttempts, err)
        }
        
        slog.Warn("Operation failed, retrying",
            "attempt", attempt,
            "max_attempts", opts.MaxAttempts,
            "delay", delay,
            "error", err)
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
            // Exponential backoff with jitter
            delay = time.Duration(float64(delay) * opts.Multiplier)
            if delay > opts.MaxDelay {
                delay = opts.MaxDelay
            }
        }
    }
    
    return ErrMaxRetries
}
```

### **Database Schema & Migrations**

```go
// internal/storage/migrations.go
package storage

import (
    "database/sql"
    "fmt"
)

type Migration struct {
    Version     int
    Description string
    Up          func(*sql.Tx) error
}

var migrations = []Migration{
    {
        Version:     1,
        Description: "Initial schema",
        Up: func(tx *sql.Tx) error {
            queries := []string{
                `CREATE TABLE IF NOT EXISTS transactions (
                    id TEXT PRIMARY KEY,
                    hash TEXT UNIQUE NOT NULL,
                    date DATETIME NOT NULL,
                    name TEXT NOT NULL,
                    merchant_name TEXT,
                    amount REAL NOT NULL,
                    plaid_categories TEXT,
                    account_id TEXT,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
                )`,
                `CREATE INDEX idx_transactions_date ON transactions(date)`,
                `CREATE INDEX idx_transactions_merchant ON transactions(merchant_name)`,
                `CREATE INDEX idx_transactions_hash ON transactions(hash)`,
                
                `CREATE TABLE IF NOT EXISTS categories (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    name TEXT UNIQUE NOT NULL,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    is_active BOOLEAN DEFAULT 1
                )`,
                
                `CREATE TABLE IF NOT EXISTS vendors (
                    name TEXT PRIMARY KEY,
                    category TEXT NOT NULL,
                    last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
                    use_count INTEGER DEFAULT 0,
                    FOREIGN KEY (category) REFERENCES categories(name)
                )`,
                
                `CREATE TABLE IF NOT EXISTS classifications (
                    transaction_id TEXT PRIMARY KEY,
                    category TEXT NOT NULL,
                    status TEXT NOT NULL,
                    confidence REAL DEFAULT 0,
                    classified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    notes TEXT,
                    FOREIGN KEY (transaction_id) REFERENCES transactions(id)
                )`,
                `CREATE INDEX idx_classifications_category ON classifications(category)`,
                
                `CREATE TABLE IF NOT EXISTS progress (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    last_processed_id TEXT,
                    last_processed_date DATETIME,
                    total_processed INTEGER DEFAULT 0,
                    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
                )`,
            }
            
            for _, query := range queries {
                if _, err := tx.Exec(query); err != nil {
                    return fmt.Errorf("failed to execute query: %w", err)
                }
            }
            return nil
        },
    },
    {
        Version:     2,
        Description: "Add classification history for auditing",
        Up: func(tx *sql.Tx) error {
            return tx.QueryRow(`
                CREATE TABLE IF NOT EXISTS classification_history (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    transaction_id TEXT NOT NULL,
                    category TEXT NOT NULL,
                    status TEXT NOT NULL,
                    confidence REAL DEFAULT 0,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    FOREIGN KEY (transaction_id) REFERENCES transactions(id)
                )
            `).Err()
        },
    },
}

func (s *SQLiteStorage) Migrate(ctx context.Context) error {
    // Get current version
    var currentVersion int
    err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&currentVersion)
    if err != nil {
        return fmt.Errorf("failed to get schema version: %w", err)
    }
    
    // Apply migrations
    for _, migration := range migrations {
        if migration.Version <= currentVersion {
            continue
        }
        
        tx, err := s.db.BeginTx(ctx, nil)
        if err != nil {
            return fmt.Errorf("failed to begin transaction: %w", err)
        }
        
        if err := migration.Up(tx); err != nil {
            tx.Rollback()
            return fmt.Errorf("migration %d failed: %w", migration.Version, err)
        }
        
        // Update version
        if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", migration.Version)); err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to update schema version: %w", err)
        }
        
        if err := tx.Commit(); err != nil {
            return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
        }
        
        slog.Info("Applied migration",
            "version", migration.Version,
            "description", migration.Description)
    }
    
    return nil
}
```

### **Classification Engine with Progress Tracking**

```go
// internal/engine/classifier.go
package engine

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "thespicemustflow/internal/model"
    "thespicemustflow/internal/service"
    "thespicemustflow/internal/common"
    "golang.org/x/time/rate"
)

type ClassificationEngine struct {
    storage      service.Storage
    llm          service.LLMClassifier
    prompter     service.UserPrompter
    rateLimiter  *rate.Limiter
    vendorCache  map[string]*model.Vendor
    cacheExpiry  time.Time
}

func New(storage service.Storage, llm service.LLMClassifier, prompter service.UserPrompter) *ClassificationEngine {
    return &ClassificationEngine{
        storage:     storage,
        llm:         llm,
        prompter:    prompter,
        rateLimiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10), // 10 requests/second burst
        vendorCache: make(map[string]*model.Vendor),
    }
}

func (e *ClassificationEngine) Classify(ctx context.Context, startDate, endDate time.Time) error {
    // Set up signal handling for graceful interruption
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    defer signal.Stop(sigChan)
    
    go func() {
        <-sigChan
        slog.Info("Received interrupt signal, saving progress...")
        cancel()
    }()
    
    // Load previous progress
    progress, err := e.storage.GetLatestProgress(ctx)
    if err != nil && !errors.Is(err, sql.ErrNoRows) {
        return fmt.Errorf("failed to load progress: %w", err)
    }
    
    var fromDate *time.Time
    if progress != nil && !progress.LastProcessedDate.IsZero() {
        fromDate = &progress.LastProcessedDate
        slog.Info("Resuming from previous run", 
            "last_processed_date", fromDate,
            "total_processed", progress.TotalProcessed)
    }
    
    // Begin transaction for atomic operations
    tx, err := e.storage.BeginTx(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()
    
    // Get transactions to classify
    transactions, err := tx.GetTransactionsToClassify(ctx, fromDate)
    if err != nil {
        return fmt.Errorf("failed to get transactions: %w", err)
    }
    
    if len(transactions) == 0 {
        slog.Info("No transactions to classify")
        return nil
    }
    
    slog.Info("Starting classification", 
        "total_transactions", len(transactions),
        "date_range", fmt.Sprintf("%s to %s", startDate, endDate))
    
    // Warm up vendor cache
    if err := e.warmVendorCache(ctx); err != nil {
        slog.Warn("Failed to warm vendor cache", "error", err)
    }
    
    // Group transactions by merchant for batch processing
    merchantGroups := e.groupByMerchant(transactions)
    
    totalProcessed := 0
    autoClassified := 0
    userClassified := 0
    newVendorRules := 0
    startTime := time.Now()
    
    if progress != nil {
        totalProcessed = progress.TotalProcessed
    }
    
    for merchant, txns := range merchantGroups {
        select {
        case <-ctx.Done():
            // Save progress before exiting
            if err := e.saveProgress(ctx, txns[0].ID, txns[0].Date, totalProcessed); err != nil {
                slog.Error("Failed to save progress", "error", err)
            }
            return ctx.Err()
        default:
        }
        
        // Show progress
        e.prompter.ShowProgress(totalProcessed, len(transactions), merchant)
        
        // Process merchant group
        classifications, wasAutomatic, err := e.processMerchantGroup(ctx, merchant, txns)
        if err != nil {
            slog.Error("Failed to process merchant group", 
                "merchant", merchant,
                "error", err)
            continue
        }
        
        // Track statistics
        if wasAutomatic {
            autoClassified += len(classifications)
        } else {
            userClassified += len(classifications)
            newVendorRules++
        }
        
        // Save classifications
        for _, classification := range classifications {
            if err := tx.SaveClassification(ctx, &classification); err != nil {
                slog.Error("Failed to save classification",
                    "transaction_id", classification.Transaction.ID,
                    "error", err)
            }
        }
        
        totalProcessed += len(classifications)
    }
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    // Clear final progress
    if err := e.clearProgress(ctx); err != nil {
        slog.Warn("Failed to clear progress", "error", err)
    }
    
    // Show completion summary
    e.prompter.ShowCompletion(service.CompletionStats{
        TotalTransactions: len(transactions),
        AutoClassified:    autoClassified,
        UserClassified:    userClassified,
        NewVendorRules:    newVendorRules,
        Duration:          time.Since(startTime),
    })
    
    return nil
}

func (e *ClassificationEngine) processMerchantGroup(ctx context.Context, merchant string, txns []model.Transaction) ([]model.Classification, bool, error) {
    // Check vendor cache first
    if vendor := e.getCachedVendor(merchant); vendor != nil {
        return e.classifyByRule(txns, vendor), true, nil
    }
    
    // Check database
    vendor, err := e.storage.GetVendor(ctx, merchant)
    if err == nil && vendor != nil {
        e.cacheVendor(vendor)
        // Update use count
        vendor.UseCount += len(txns)
        e.storage.SaveVendor(ctx, vendor)
        return e.classifyByRule(txns, vendor), true, nil
    }
    
    // Need AI classification
    classifications, err := e.classifyByAI(ctx, merchant, txns)
    return classifications, false, err
}

func (e *ClassificationEngine) classifyByRule(txns []model.Transaction, vendor *model.Vendor) []model.Classification {
    classifications := make([]model.Classification, len(txns))
    for i, txn := range txns {
        classifications[i] = model.Classification{
            Transaction:  txn,
            Category:     vendor.Category,
            Status:       model.StatusClassifiedByRule,
            Confidence:   1.0,
            ClassifiedAt: time.Now(),
        }
    }
    return classifications
}

func (e *ClassificationEngine) classifyByAI(ctx context.Context, merchant string, txns []model.Transaction) ([]model.Classification, error) {
    // Rate limit AI requests
    if err := e.rateLimiter.Wait(ctx); err != nil {
        return nil, err
    }
    
    // Get current categories
    categories, err := e.storage.GetCategories(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get categories: %w", err)
    }
    
    categoryNames := make([]string, len(categories))
    for i, cat := range categories {
        categoryNames[i] = cat.Name
    }
    
    // Get AI suggestion for the first transaction
    representative := txns[0]
    
    var category string
    var confidence float64
    var isNew bool
    
    err = common.WithRetry(ctx, func() error {
        var err error
        category, confidence, isNew, err = e.llm.SuggestCategory(ctx, representative, categoryNames)
        return err
    }, common.RetryOptions{
        MaxAttempts:  3,
        InitialDelay: 500 * time.Millisecond,
    })
    
    if err != nil {
        return nil, fmt.Errorf("AI classification failed: %w", err)
    }
    
    // Check if this is a high-variance merchant
    if e.hasHighVariance(txns) {
        // Review individually
        return e.reviewIndividually(ctx, merchant, txns)
    }
    
    // Prepare for batch review
    pending := []model.PendingClassification{
        {
            Transaction:       representative,
            SuggestedCategory: category,
            Confidence:        confidence,
            SimilarCount:      len(txns),
        },
    }
    
    classifications, err := e.prompter.BatchReview(pending)
    if err != nil {
        return nil, err
    }
    
    // Save vendor rule if confirmed
    if len(classifications) > 0 {
        vendor := &model.Vendor{
            Name:        merchant,
            Category:    classifications[0].Category,
            LastUpdated: time.Now(),
            UseCount:    len(txns),
        }
        
        if err := e.storage.SaveVendor(ctx, vendor); err != nil {
            slog.Warn("Failed to save vendor rule", "error", err)
        }
        
        e.cacheVendor(vendor)
    }
    
    return classifications, nil
}

func (e *ClassificationEngine) hasHighVariance(txns []model.Transaction) bool {
    if len(txns) < 5 {
        return false
    }
    
    var min, max float64
    for i, txn := range txns {
        if i == 0 || txn.Amount < min {
            min = txn.Amount
        }
        if i == 0 || txn.Amount > max {
            max = txn.Amount
        }
    }
    
    // If max is more than 10x min, consider it high variance
    return max > min*10
}

func (e *ClassificationEngine) groupByMerchant(transactions []model.Transaction) map[string][]model.Transaction {
    groups := make(map[string][]model.Transaction)
    for _, txn := range transactions {
        merchant := txn.MerchantName
        if merchant == "" {
            merchant = txn.Name // Fallback to raw name
        }
        groups[merchant] = append(groups[merchant], txn)
    }
    return groups
}

func (e *ClassificationEngine) warmVendorCache(ctx context.Context) error {
    vendors, err := e.storage.GetAllVendors(ctx)
    if err != nil {
        return err
    }
    
    for _, vendor := range vendors {
        e.vendorCache[vendor.Name] = &vendor
    }
    
    e.cacheExpiry = time.Now().Add(5 * time.Minute)
    slog.Info("Warmed vendor cache", "vendor_count", len(vendors))
    return nil
}

func (e *ClassificationEngine) getCachedVendor(name string) *model.Vendor {
    if time.Now().After(e.cacheExpiry) {
        e.vendorCache = make(map[string]*model.Vendor)
        return nil
    }
    return e.vendorCache[name]
}

func (e *ClassificationEngine) cacheVendor(vendor *model.Vendor) {
    e.vendorCache[vendor.Name] = vendor
}
```

### **CLI User Interface Implementation**

```go
// internal/cli/prompter.go
package cli

import (
    "bufio"
    "fmt"
    "os"
    "strings"
    "time"
    
    "github.com/charmbracelet/lipgloss"
    "github.com/schollz/progressbar/v3"
    "thespicemustflow/internal/model"
    "thespicemustflow/internal/service"
)

type CLIPrompter struct {
    scanner     *bufio.Scanner
    progressBar *progressbar.ProgressBar
    styles      *Styles
}

type Styles struct {
    Title       lipgloss.Style
    Box         lipgloss.Style
    Success     lipgloss.Style
    Warning     lipgloss.Style
    Error       lipgloss.Style
    Info        lipgloss.Style
    Prompt      lipgloss.Style
}

func NewStyles() *Styles {
    return &Styles{
        Title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6B6B")),
        Box:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2),
        Success: lipgloss.NewStyle().Foreground(lipgloss.Color("#4ECDC4")),
        Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFE66D")),
        Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")),
        Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("#95E1D3")),
        Prompt:  lipgloss.NewStyle().Bold(true),
    }
}

func NewCLIPrompter() *CLIPrompter {
    return &CLIPrompter{
        scanner: bufio.NewScanner(os.Stdin),
        styles:  NewStyles(),
    }
}

func (p *CLIPrompter) ShowProgress(current, total int, currentMerchant string) {
    if p.progressBar == nil {
        p.progressBar = progressbar.NewOptions(total,
            progressbar.OptionSetDescription("Processing..."),
            progressbar.OptionSetWidth(40),
            progressbar.OptionShowCount(),
            progressbar.OptionShowIts(),
            progressbar.OptionSetTheme(progressbar.Theme{
                Saucer:        "━",
                SaucerPadding: "━",
                BarStart:      "",
                BarEnd:        "",
            }),
        )
    }
    
    p.progressBar.Set(current)
    p.progressBar.Describe(fmt.Sprintf("%d%% | %d/%d | %s", 
        current*100/total, current, total, currentMerchant))
}

func (p *CLIPrompter) BatchReview(pending []model.PendingClassification) ([]model.Classification, error) {
    // Clear progress bar line
    fmt.Print("\r\033[K")
    
    txn := pending[0].Transaction
    
    // Build review box
    content := fmt.Sprintf(
        "Found %d transactions\nTotal: $%.2f\nDate range: %s - %s\n\n🤖 AI suggests: %s (%.0f%% confidence)",
        pending[0].SimilarCount,
        p.calculateTotal(pending),
        p.getDateRange(pending),
        pending[0].SuggestedCategory,
        pending[0].Confidence*100,
    )
    
    box := p.styles.Box.Render(content)
    title := p.styles.Title.Render(fmt.Sprintf("Review: %s", txn.MerchantName))
    
    fmt.Printf("\n%s\n%s\n\n", title, box)
    
    // Show options
    fmt.Print(p.styles.Prompt.Render("[A]ccept all  [C]ustom category  [R]eview each  [S]kip → "))
    
    p.scanner.Scan()
    choice := strings.ToLower(strings.TrimSpace(p.scanner.Text()))
    
    switch choice {
    case "a":
        // Accept AI suggestion for all
        classifications := make([]model.Classification, len(pending))
        for i, pc := range pending {
            classifications[i] = model.Classification{
                Transaction:  pc.Transaction,
                Category:     pc.SuggestedCategory,
                Status:       model.StatusClassifiedByAI,
                Confidence:   pc.Confidence,
                ClassifiedAt: time.Now(),
            }
        }
        
        fmt.Printf("\n%s Created rule: %s → %s\n", 
            p.styles.Success.Render("✓"),
            txn.MerchantName,
            pending[0].SuggestedCategory)
        fmt.Printf("%s Categorized %d transactions\n\n",
            p.styles.Success.Render("✓"),
            len(pending))
        
        return classifications, nil
        
    case "c":
        // Custom category for all
        fmt.Print(p.styles.Prompt.Render("Enter category: "))
        p.scanner.Scan()
        category := strings.TrimSpace(p.scanner.Text())
        
        if category == "" {
            return nil, fmt.Errorf("category cannot be empty")
        }
        
        classifications := make([]model.Classification, len(pending))
        for i, pc := range pending {
            classifications[i] = model.Classification{
                Transaction:  pc.Transaction,
                Category:     category,
                Status:       model.StatusUserModified,
                Confidence:   1.0,
                ClassifiedAt: time.Now(),
            }
        }
        
        fmt.Printf("\n%s Created rule: %s → %s\n",
            p.styles.Success.Render("✓"),
            txn.MerchantName,
            category)
        
        return classifications, nil
        
    case "r":
        // Review individually
        return p.reviewIndividually(pending)
        
    case "s":
        // Skip
        fmt.Printf("\n%s Skipped %d transactions\n\n",
            p.styles.Info.Render("ℹ️"),
            len(pending))
        return nil, nil
        
    default:
        return nil, fmt.Errorf("invalid choice")
    }
}

func (p *CLIPrompter) ShowCompletion(stats service.CompletionStats) {
    fmt.Print("\r\033[K") // Clear progress bar
    
    fmt.Printf("\n%s Classification complete!\n\n", p.styles.Success.Render("✅"))
    
    timeSaved := float64(stats.TotalTransactions) * 30 / 60 // Assume 30 seconds per manual transaction
    efficiency := float64(stats.AutoClassified) / float64(stats.TotalTransactions) * 100
    
    summary := fmt.Sprintf(
        `Total transactions:      %d
Auto-classified:         %d (%.1f%%)
User-classified:         %d (%.1f%%)

New vendor rules:        %d
Time taken:              %s
Time saved:              ~%.0f hours (estimated)

Ready for export: spice flow --export`,
        stats.TotalTransactions,
        stats.AutoClassified, efficiency,
        stats.UserClassified, 100-efficiency,
        stats.NewVendorRules,
        stats.Duration.Round(time.Second),
        timeSaved/60,
    )
    
    box := p.styles.Box.Render(summary)
    title := p.styles.Title.Render("Summary")
    
    fmt.Printf("%s\n%s\n\n", title, box)
}

func (p *CLIPrompter) calculateTotal(pending []model.PendingClassification) float64 {
    var total float64
    for _, pc := range pending {
        total += pc.Transaction.Amount
    }
    return total
}

func (p *CLIPrompter) getDateRange(pending []model.PendingClassification) string {
    if len(pending) == 0 {
        return ""
    }
    
    minDate := pending[0].Transaction.Date
    maxDate := pending[0].Transaction.Date
    
    for _, pc := range pending[1:] {
        if pc.Transaction.Date.Before(minDate) {
            minDate = pc.Transaction.Date
        }
        if pc.Transaction.Date.After(maxDate) {
            maxDate = pc.Transaction.Date
        }
    }
    
    if minDate.Equal(maxDate) {
        return minDate.Format("Jan 2, 2006")
    }
    
    return fmt.Sprintf("%s - %s", 
        minDate.Format("Jan 2"),
        maxDate.Format("Jan 2, 2006"))
}
```

---

## **6. Tooling & Libraries**

* **CLI Framework:** `github.com/spf13/cobra` - For command/subcommand and flag parsing
* **Configuration:** `github.com/spf13/viper` - For config files and environment variables
* **Testing:** `github.com/stretchr/testify` - For assertions and mocking
* **Logging:** `log/slog` - Go's structured logging (with console and JSON handlers)
* **Database:** `github.com/mattn/go-sqlite3` - SQLite driver
* **Plaid:** `github.com/plaid/plaid-go/v24/plaid` - Official Plaid SDK
* **Rate Limiting:** `golang.org/x/time/rate` - Token bucket rate limiter
* **Progress Bar:** `github.com/schollz/progressbar/v3` - For visual progress
* **UI Styling:** `github.com/charmbracelet/lipgloss` - For beautiful terminal UI
* **Table Output:** `github.com/olekukonko/tablewriter` - For formatted tables

---

## **7. Pattern-Based Classification System**

### **Overview**

The pattern-based classification system provides intelligent, rule-based categorization that considers multiple transaction attributes beyond just merchant name. This system addresses the limitation of simple vendor rules that don't account for transaction direction or amount patterns.

### **Key Components**

- **Pattern Rules**: Flexible rules combining merchant patterns, amount conditions, and transaction direction
- **Pattern Matcher**: Evaluates transactions against all active patterns
- **Transaction Validator**: Ensures category assignments match transaction direction
- **Category Suggester**: Provides intelligent suggestions with confidence scoring

### **Pattern Rule Capabilities**

1. **Merchant Patterns**: Exact match or regex (e.g., "AMAZON.*" for all Amazon variants)
2. **Amount Conditions**: lt, le, eq, ge, gt, range (e.g., "< $10" for small purchases)
3. **Direction Filtering**: Income vs expense patterns for the same merchant
4. **Priority System**: Higher priority patterns override lower ones
5. **Confidence Scoring**: Patterns include confidence levels for nuanced classification

### **Integration**

Pattern rules are checked before vendor rules during classification, providing more accurate categorization while maintaining backward compatibility. The AI analysis system can generate new pattern rules based on discovered patterns in historical data.

---

## **8. Phased Development Plan**

### **Phase 0: Foundation & Setup**
* **Goal:** Establish project structure, dependencies, and basic CLI shell.
* **Tasks:**
    1. Initialize Go module: `go mod init thespicemustflow`
    2. Create directory structure as specified
    3. Set up Cobra CLI with commands: `classify`, `vendors`, `flow`, `migrate`
    4. Integrate Viper for configuration management
    5. Set up structured logging with slog
    6. Create all interfaces and models in `internal/service` and `internal/model`
    7. Implement retry logic in `internal/common/retry.go`
    8. Create Makefile with targets: `build`, `test`, `lint`, `run`
    9. Set up basic CLI styles using lipgloss
* **Testing:** Verify CLI commands parse correctly, config loads
* **Outcome:** A runnable application with proper structure and foundational utilities.

### **Phase 1: Storage Layer with Migrations**
* **Goal:** Create a robust, versioned storage layer with duplicate detection.
* **Tasks:**
    1. Implement SQLite storage with migration support
    2. Create all database methods with proper transaction handling
    3. Implement transaction hash generation and duplicate detection
    4. Add progress tracking tables and methods
    5. Create `migrate` command to run database migrations
    6. Add vendor caching logic
* **Testing:** Comprehensive unit tests using in-memory SQLite, test duplicate detection edge cases
* **Outcome:** A fully tested persistence layer that prevents duplicate transactions.

### **Phase 2: Plaid Integration**
* **Goal:** Reliable transaction fetching with proper error handling.
* **Tasks:**
    1. Implement PlaidClient with retry logic
    2. Add transaction hash generation on fetch
    3. Create `test-plaid` subcommand for connection verification
    4. Implement batch transaction saving with duplicate detection
    5. Handle Plaid-specific errors gracefully
* **Testing:** Mock Plaid client tests, integration tests with sandbox API
* **Outcome:** Ability to fetch and store transactions reliably.

### **Phase 3: Classification Engine Core**
* **Goal:** Implement the main classification logic with progress tracking.
* **Tasks:**
    1. Create ClassificationEngine with dependency injection
    2. Implement merchant grouping logic with pattern detection
    3. Add vendor caching with TTL
    4. Implement graceful shutdown with automatic progress saving
    5. Create mock LLM classifier for testing
    6. Add high-variance merchant detection
* **Testing:** Table-driven tests for all classification scenarios, interrupt handling tests
* **Outcome:** A resumable classification engine that intelligently groups transactions.

### **Phase 4: User Interface & Experience**
* **Goal:** Implement the delightful CLI experience with batch processing.
* **Tasks:**
    1. Create CLIPrompter with styled output using lipgloss
    2. Implement single transaction review flow
    3. Implement batch review with high-variance detection
    4. Add smart pattern detection (e.g., "last 5 were Office Supplies")
    5. Create progress visualization with time estimates
    6. Add interrupt handling with friendly messages
    7. Implement completion summary with statistics
* **Testing:** Manual testing of all interaction flows, automated tests with expect
* **Outcome:** A user interface that makes categorization efficient and enjoyable.

### **Phase 5: LLM Integration**
* **Goal:** Integrate real AI classification with smart prompting.
* **Tasks:**
    1. Implement OpenAI/Anthropic client with structured prompts
    2. Design prompts that include transaction context for better accuracy
    3. Add rate limiting to prevent API throttling
    4. Implement confidence scoring and thresholds
    5. Create fallback for when AI is unavailable
    6. Add cost tracking/estimation in logs
* **Testing:** Tests with recorded API responses, rate limit testing
* **Outcome:** Intelligent classification that reduces manual work.

### **Phase 6: Vendor Management & Reporting**
* **Goal:** Complete vendor commands and final reporting.
* **Tasks:**
    1. Implement `spice vendors list/search/edit/delete` commands
    2. Add vendor statistics (use count, last updated)
    3. Create `spice flow` command for spending analysis
    4. Implement Google Sheets export with proper formatting
    5. Add monthly/yearly comparison reports
    6. Polish all error messages and help text
* **Testing:** End-to-end test with full year of data
* **Outcome:** Complete application ready for tax season.

---

## **9. Testing Strategy**

### **Unit Testing Guidelines**
* **Coverage Target:** >90% for business logic, >80% overall
* **Approach:** Table-driven tests, extensive use of interfaces for mocking
* **Key Areas:**
    - Retry logic with various failure scenarios
    - Duplicate detection with edge cases
    - Progress saving and resuming
    - Vendor caching behavior
    - High-variance detection
    - Smart pattern detection

### **Integration Testing**
```go
//go:build integration

func TestClassificationEngineIntegration(t *testing.T) {
    // Use real SQLite with test data
    db := setupTestDatabase(t)
    defer db.Close()
    
    // Use recorded API responses
    llm := NewRecordedLLMClient("testdata/llm_responses.json")
    prompter := NewAutomatedPrompter(map[string]string{
        "Starbucks": "Coffee & Dining",
        "Amazon":    "Shopping",
    })
    
    // Test full classification flow
    engine := engine.New(db, llm, prompter)
    err := engine.Classify(ctx, startDate, endDate)
    require.NoError(t, err)
    
    // Verify results
    classifications := getClassifications(db)
    assert.Len(t, classifications, expectedCount)
    
    // Test interrupt and resume
    // ... additional test scenarios
}
```

### **User Experience Testing**
Create a test harness that can automatically drive the CLI:
```go
func TestUserFlows(t *testing.T) {
    tests := []struct {
        name     string
        scenario string
        inputs   []string
        expected []string
    }{
        {
            name:     "Accept batch suggestion",
            scenario: "batch_review",
            inputs:   []string{"a"},
            expected: []string{"Created rule:", "Categorized 23 transactions"},
        },
        // ... more test cases
    }
}
```

---

## **10. Configuration Reference**

```yaml
# ~/.config/spice/config.yaml
plaid:
  client_id: ${PLAID_CLIENT_ID}
  secret: ${PLAID_SECRET}
  environment: sandbox  # sandbox, development, or production
  accounts:             # Optional: filter specific accounts
    - checking_main
    - credit_card_1

database:
  path: ~/.local/share/spice/spice.db
  
llm:
  provider: openai      # or anthropic
  api_key: ${OPENAI_API_KEY}
  model: gpt-4
  max_tokens: 150
  temperature: 0.3
  timeout: 30s
  
sheets:
  credentials_path: ~/.config/spice/sheets-creds.json
  spreadsheet_id: ${SHEETS_SPREADSHEET_ID}
  
classification:
  batch_size: 50        # Max transactions to review at once
  variance_threshold: 10 # Consider high variance if max/min > this
  cache_ttl: 5m         # Vendor cache duration
  auto_accept_confidence: 0.95 # Auto-accept if AI confidence >= this
  
ui:
  style: emoji          # emoji or ascii
  progress_bar: true
  colors: true
  
logging:
  level: info           # debug, info, warn, error
  format: console       # console or json
  file: ~/.local/share/spice/spice.log
```

---

## **13. Performance Considerations**

* **Batch Operations:** All database operations use prepared statements and batch inserts
* **Connection Management:** Single SQLite connection with proper transaction boundaries
* **Memory Efficiency:** Process transactions in chunks to handle large datasets
* **Caching Strategy:** 
    - In-memory vendor cache reduces database queries by ~80%
    - 5-minute TTL balances freshness with performance
* **Rate Limiting:** Prevents API throttling and manages costs
* **Progress Tracking:** Enables processing years of data across multiple sessions

### **Benchmarks to Target**
- Classification speed: >100 transactions/second (excluding user input)
- Memory usage: <100MB for 10,000 transactions
- Startup time: <100ms
- Vendor cache hit rate: >80% after warmup

---

## **14. Error Handling Philosophy**

The application follows these error handling principles:

1. **User-Friendly Messages:** Technical errors are logged, users see helpful messages
2. **Graceful Degradation:** If AI fails, fall back to manual mode
3. **Data Integrity:** Use transactions, never leave data in inconsistent state
4. **Progress Preservation:** Always save progress before exiting
5. **Actionable Errors:** Tell users how to fix problems

Example error transformations:
```go
// Technical error (logged)
"pq: duplicate key value violates unique constraint"

// User sees
"This transaction has already been processed"

// Technical error (logged)
"rate limit exceeded: 429 Too Many Requests"

// User sees
"⚠️  AI categorization temporarily unavailable (rate limit)
   Switching to manual mode..."
```

---

## **15. Future Extensibility**

The architecture supports these future enhancements:

* **Multiple Data Sources:** CSV import, other bank APIs
* **Plugin System:** Custom categorization rules as plugins
* **Web UI:** REST API server mode for web interface
* **Multi-User:** Tenant isolation, user management
* **Receipt Matching:** Attach receipts to transactions
* **Budget Integration:** Compare spending to budgets
* **Recurring Transaction Detection:** Identify subscriptions
* **Split Transactions:** Allocate single transaction to multiple categories
* **Export Formats:** QIF, OFX, CSV in addition to Google Sheets
* **Real-time Analysis:** Continuous learning from classification decisions
* **Predictive Categorization:** Anticipate categories before user confirmation

The interface-driven architecture makes these extensions straightforward to add without disrupting the core engine.

---

## **Conclusion**

the-spice-must-flow provides a delightful, efficient way to categorize financial transactions for tax preparation. By combining intelligent defaults, batch processing, and a polished CLI experience, it transforms a tedious task into something almost enjoyable. The robust architecture ensures reliability while maintaining flexibility for future enhancements.

Remember: The spice must flow! 🌶️
