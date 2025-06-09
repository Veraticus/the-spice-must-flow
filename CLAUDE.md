# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

the-spice-must-flow is a personal finance categorization engine that ingests financial transactions from Plaid, uses AI to intelligently categorize them, and exports reports to Google Sheets. The project is written in Go and emphasizes testability, extensibility, and a delightful CLI experience.

**Module name**: `github.com/joshsymonds/the-spice-must-flow` (follows Go conventions for GitHub-hosted modules)

## Development Commands

```bash
# Build the binary
make build

# Run unit tests
make test

# Run comprehensive tests (includes race detection, integration tests, cross-platform builds)
make test-all

# Format code
make fmt

# Run linter and verification
make lint

# Auto-fix formatting and common issues
make fix

# Install required development tools
make install-tools
```

## Architecture

The project follows interface-driven design with dependency injection for testability. Key components:

- `cmd/spice/` - CLI commands using cobra
- `internal/model/` - Core domain models (Transaction, Category, Vendor, etc.)
- `internal/service/` - Service interfaces for all major components
- `internal/storage/` - SQLite-based storage implementation
- `internal/llm/` - AI classification service
- `internal/plaid/` - Plaid API client
- `internal/sheets/` - Google Sheets export
- `internal/engine/` - Core classification orchestration

## Key Design Patterns

1. **Interface-First**: All services defined as interfaces before implementation
2. **Transaction Deduplication**: SHA256 hashing prevents duplicate processing
3. **Batch Processing**: Transactions grouped by merchant for efficient review
4. **Progress Tracking**: Classification sessions can be resumed if interrupted
5. **Vendor Rules**: System learns from user decisions to auto-categorize

## Testing Guidelines

- Business logic should have >90% test coverage
- Use table-driven tests for comprehensive scenarios
- Mock external dependencies using interfaces
- Integration tests go in `_integration_test.go` files

## Configuration

The application uses environment variables for sensitive data:
- Plaid credentials
- OpenAI/Anthropic API keys
- Google Sheets credentials

Configuration management uses viper, with configs stored in appropriate XDG directories.

## Development Status

Phase 0 (Foundation & Setup) ✅ Complete:
- Go module initialized
- Directory structure created
- Cobra CLI with all commands
- Viper configuration integration
- Structured logging with slog
- All interfaces and models defined
- Retry logic implemented
- Makefile configured
- Basic CLI styling with lipgloss

Next: Phase 1 - Storage Layer with Migrations