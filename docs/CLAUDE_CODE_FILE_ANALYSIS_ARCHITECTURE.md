# Claude Code File-Based Analysis Architecture

## Overview

This document describes the technical architecture for handling large transaction datasets in the `spice analyze` feature when using Claude Code as the LLM provider. The architecture addresses command-line length limitations by using a file-based approach for passing transaction data to Claude Code.

## Problem Statement

### Current Limitations
1. **Command-line argument length limits**: Most operating systems limit command-line arguments to ~2MB
2. **Claude Code CLI constraints**:
   - No stdin support for input data
   - `--print` mode is single-turn only
   - No support for streaming large data through arguments
3. **Dataset size**: Financial analysis requires processing thousands of transactions
4. **Context window**: Need to fit both instructions and data within Claude's context limits

### Requirements
- Analyze 5000+ transactions in a single request
- Maintain structured JSON output for programmatic parsing
- Support "ultrathink" mode for deep analysis
- Handle errors gracefully with proper cleanup
- Provide debugging visibility

## Proposed Solution: File-Based Single-Turn Analysis

### Architecture Diagram

```
┌─────────────────────┐     ┌────────────────────────┐     ┌─────────────────────┐
│ Transaction Store   │────▶│ Temp File Writer       │────▶│ Claude Code CLI     │
│ - SQLite DB         │     │ - JSON marshaling      │     │ - Read file path    │
│ - 5000+ records     │     │ - ./tmp/spice-*.json   │     │ - Analyze data      │
│ - Categories        │     │ - Secure permissions   │     │ - Return JSON       │
│ - Patterns          │     └────────────────────────┘     └─────────────────────┘
└─────────────────────┘                                                │
                                                                       ▼
┌─────────────────────┐     ┌────────────────────────┐     ┌─────────────────────┐
│ Analysis Engine     │◀────│ Response Parser        │◀────│ JSON Response       │
│ - Process results   │     │ - Validate structure   │     │ - Coherence score   │
│ - Apply fixes       │     │ - Error recovery       │     │ - Issues found      │
│ - Generate report   │     └────────────────────────┘     │ - Recommendations   │
└─────────────────────┘                                     └─────────────────────┘
                            ┌────────────────────────┐
                            │ Cleanup Handler        │
                            │ - Delete temp files    │
                            │ - Log cleanup status   │
                            └────────────────────────┘
```

## Technical Implementation Details

### 1. Temporary File Management

#### Directory Structure
```
./tmp/                              # Git-ignored temporary directory
├── spice-analysis-{uuid}.json      # Transaction data file
└── .gitkeep                        # Ensure directory exists in repo
```

#### File Creation Process
```go
// Generate secure random filename
filename := fmt.Sprintf("spice-analysis-%s.json", uuid.New().String())
filepath := filepath.Join(".", "tmp", filename)

// Ensure tmp directory exists
if err := os.MkdirAll(filepath.Dir(filepath), 0755); err != nil {
    return fmt.Errorf("failed to create tmp directory: %w", err)
}

// Create file with restrictive permissions
file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
if err != nil {
    return fmt.Errorf("failed to create temp file: %w", err)
}
defer file.Close()

// Always cleanup on function exit
defer func() {
    if err := os.Remove(filepath); err != nil {
        slog.Warn("Failed to cleanup temp file", "path", filepath, "error", err)
    }
}()
```

### 2. Data File Format

The temporary JSON file contains both transaction data and analysis metadata:

```json
{
  "metadata": {
    "version": "1.0",
    "generated_at": "2024-06-29T19:54:01Z",
    "analysis_id": "3e53e01c-c3e9-44ab-a9c7-8410a5e10d9d",
    "total_transactions": 5234,
    "included_transactions": 5000,
    "date_range": {
      "start": "2024-01-01",
      "end": "2024-12-31"
    },
    "categories": [
      {
        "id": "groceries",
        "name": "Groceries",
        "type": "expense",
        "description": "Food and household supplies"
      }
    ],
    "pattern_rules": [
      {
        "id": "pattern_001",
        "merchant_pattern": "WHOLE FOODS.*",
        "default_category": "groceries",
        "priority": 100,
        "is_regex": true
      }
    ],
    "check_patterns": [
      {
        "pattern_name": "Rent Payment",
        "amount_min": 2000.00,
        "amount_max": 2500.00,
        "category": "rent"
      }
    ]
  },
  "transactions": [
    {
      "id": "txn_20240115_001",
      "date": "2024-01-15",
      "name": "STARBUCKS STORE #12345",
      "amount": 5.75,
      "type": "DEBIT",
      "category": ["coffee-shops"],
      "account_id": "acc_checking_001",
      "plaid_id": "plaid_txn_abc123",
      "confidence": 0.95
    }
  ]
}
```

### 3. Prompt Engineering

#### System Prompt Structure
```text
You are an AI assistant specialized in financial transaction analysis. Your task is to analyze transaction categorization patterns and provide detailed insights.

IMPORTANT: Please ultrathink through this analysis carefully, examining patterns and inconsistencies thoroughly before responding.

CRITICAL INSTRUCTIONS:
1. Read the transaction data from the file: ./tmp/spice-analysis-{uuid}.json
2. The file contains both metadata and transaction records
3. Analyze ALL transactions in the file, not just a sample
4. Focus on patterns across the entire dataset

You MUST respond with ONLY a valid JSON object that matches the provided schema. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON.
```

#### Analysis Prompt Template
```text
Analyze the financial transactions in ./tmp/spice-analysis-{uuid}.json

Your analysis should:
1. Calculate an overall coherence score (0-100) for categorization consistency
2. Identify specific issues with current categorizations
3. Suggest new pattern rules for frequently miscategorized merchants
4. Recommend category improvements
5. Provide actionable fixes for identified issues

Output your analysis as a JSON object matching this exact schema:
{schema}
```

### 4. Claude Code Integration

#### Modified Command Execution
```go
func (c *claudeCodeClient) AnalyzeWithFile(ctx context.Context, dataPath string, analysisPrompt string) (string, error) {
    // Build command arguments
    args := []string{
        "--print",                      // Non-interactive mode
        "--output-format", "json",      // Structured output
        "--model", c.model,             // Use opus for analysis
        "--add-dir", filepath.Dir(dataPath), // Allow access to temp directory
    }
    
    // Add the prompt as the last argument
    args = append(args, analysisPrompt)
    
    // Create command with extended timeout
    cmd := exec.CommandContext(ctx, c.cliPath, args...)
    
    // Set working directory to ensure relative paths work
    cmd.Dir = "."
    
    // Execute and capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("claude code execution failed: %w, output: %s", err, output)
    }
    
    return string(output), nil
}
```

### 5. Error Handling and Recovery

#### Comprehensive Error Handling
```go
type AnalysisError struct {
    Phase   string
    Message string
    Cause   error
}

func (e AnalysisError) Error() string {
    return fmt.Sprintf("analysis failed during %s: %s", e.Phase, e.Message)
}

// Error recovery stages:
1. File creation errors → Return early with clear error
2. JSON marshaling errors → Log data sample, return error
3. Claude execution errors → Log stdout/stderr, retry with backoff
4. Response parsing errors → Attempt JSON correction, fallback to partial analysis
5. Cleanup errors → Log warning but don't fail the operation
```

### 6. Performance Considerations

#### Memory Optimization
- Stream large transaction sets to file instead of loading all in memory
- Use buffered writes for file creation
- Clear transaction data from memory after file write

#### Timeout Management
```go
// Timeout scales with data size
timeout := 3*time.Minute + time.Duration(transactionCount/1000)*time.Minute
ctx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
```

### 7. Security Considerations

1. **File Permissions**: Set 0600 (owner read/write only) on temp files
2. **Filename Randomness**: Use cryptographically secure UUIDs
3. **Cleanup Guarantee**: Use defer to ensure cleanup even on panic
4. **Path Validation**: Ensure paths are within expected directories
5. **Data Sanitization**: Escape special characters in transaction names

### 8. Debugging and Observability

#### Debug Logging
```go
slog.Debug("Creating analysis temp file",
    "path", filepath,
    "transaction_count", len(transactions),
    "file_size_estimate", estimatedSize,
)

slog.Debug("Claude Code analysis request",
    "model", model,
    "timeout", timeout,
    "data_file", filepath,
    "prompt_length", len(analysisPrompt),
)

slog.Debug("Analysis completed",
    "duration", duration,
    "response_size", len(response),
    "cleanup_success", cleanupErr == nil,
)
```

#### Diagnostic Information
- Log file paths for manual inspection if needed
- Include timing information for each phase
- Track memory usage for large datasets
- Record Claude Code exit codes and signals

## Migration Path

### Phase 1: Implement File-Based Approach
1. Create temp file management utilities
2. Modify LLM adapter to write transaction data to files
3. Update prompts to reference file paths
4. Add cleanup handlers

### Phase 2: Optimize for Large Datasets  
1. Implement streaming writes for very large datasets
2. Add compression for extremely large files
3. Consider chunked analysis for datasets > 10,000 transactions

### Phase 3: Enhanced Features
1. Add progress callbacks during long analyses
2. Implement partial result recovery
3. Support incremental analysis updates

## Testing Strategy

### Unit Tests
- File creation and cleanup
- JSON marshaling of large datasets
- Error handling paths
- Timeout behavior

### Integration Tests
- End-to-end analysis with real Claude Code
- Various dataset sizes (100, 1000, 5000, 10000 transactions)
- Error recovery scenarios
- Concurrent analysis requests

### Performance Tests
- Measure memory usage with large datasets
- Benchmark file I/O operations
- Profile CPU usage during JSON marshaling

## Conclusion

This file-based architecture provides a robust solution for analyzing large transaction datasets with Claude Code. It works within the constraints of the CLI tool while maximizing the amount of data that can be analyzed in a single request. The approach is scalable, secure, and maintainable, with clear paths for future enhancements.