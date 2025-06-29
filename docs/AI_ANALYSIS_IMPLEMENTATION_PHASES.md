# AI Analysis Implementation Phases

This document breaks down the AI-powered transaction analysis feature into discrete implementation phases that can be completed in individual sessions. Each phase emphasizes Go best practices: dependency injection, testability, clean structs, and small interfaces.

## Phase 1: Core Types and Interfaces
**Estimated Time**: 2-3 hours  
**Dependencies**: None

### Goals
- Define all domain types with strong typing (no `interface{}`)
- Create small, focused interfaces (1-3 methods each)
- Set up dependency injection structure

### Checklist
- [ ] Create `internal/analysis/types.go`
  ```go
  // Core types: AnalysisReport, Issue, SuggestedPattern, etc.
  // All fields strongly typed with proper JSON tags
  // Validation methods on each type
  ```

- [ ] Create `internal/analysis/interfaces.go`
  ```go
  // Small interfaces:
  // - AnalysisService (1 method: Analyze)
  // - SessionStore (3 methods: Create, Get, Update)
  // - ReportValidator (2 methods: Validate, ExtractError)
  // - FixApplier (3 methods: ApplyPatternFixes, ApplyCategoryFixes, ApplyRecategorizations)
  // - ReportFormatter (3 methods: FormatSummary, FormatIssue, FormatInteractive)
  ```

- [ ] Create `internal/analysis/deps.go`
  ```go
  // AnalysisDeps struct for clean dependency injection
  // Constructor: NewAnalysisEngine(deps AnalysisDeps) *AnalysisEngine
  ```

### Testing Focus
- Type validation tests (table-driven)
- Interface compliance tests
- Mock generation for all interfaces

### Deliverables
- Fully typed domain model
- Clean interface definitions
- 100% test coverage on types

---

## Phase 2: Prompt Builder and LLM Integration
**Estimated Time**: 3-4 hours  
**Dependencies**: Phase 1 (types and interfaces)

### Goals
- Build maintainable prompt generation using text/template
- Create LLM wrapper with retry logic
- Ensure testability with interface-based design

### Checklist
- [ ] Create `internal/analysis/prompt_builder.go`
  ```go
  type PromptBuilder struct {
      templates map[string]*template.Template
  }
  
  func (pb *PromptBuilder) BuildAnalysisPrompt(
      transactions []model.Transaction,
      categories []model.Category,
      patterns []model.PatternRule,
  ) (string, error)
  ```

- [ ] Create `internal/analysis/templates/`
  - `analysis_prompt.tmpl` - Main analysis prompt
  - `json_schema.tmpl` - JSON structure examples
  - `correction_prompt.tmpl` - Validation correction prompt

- [ ] Create `internal/analysis/llm_adapter.go`
  ```go
  // Wrapper around service.LLMClient
  // Adds analysis-specific methods
  // Includes retry with exponential backoff
  ```

### Testing Focus
- Prompt template rendering tests
- Mock LLM responses
- Retry logic validation

### Deliverables
- Template-based prompt system
- Robust LLM integration
- Comprehensive test suite

---

## Phase 3: JSON Validation and Session Management
**Estimated Time**: 4-5 hours  
**Dependencies**: Phase 1 (types)

### Goals
- Implement robust JSON validation with error extraction
- Build session persistence with state management
- Create database migrations for analysis tables

### Checklist
- [ ] Create `internal/analysis/validator.go`
  ```go
  type JSONValidator struct {
      schema json.RawMessage
  }
  
  func (v *JSONValidator) Validate(data []byte) (*AnalysisReport, error)
  func (v *JSONValidator) ExtractBadSection(data []byte, err error) (string, int, int)
  ```

- [ ] Create `internal/analysis/session_store.go`
  ```go
  type SQLiteSessionStore struct {
      db service.Storage
  }
  
  // Implements SessionStore interface
  // Handles state transitions
  // Automatic cleanup of old sessions
  ```

- [ ] Add to `internal/storage/migrations.go`
  ```sql
  CREATE TABLE analysis_sessions (
      id TEXT PRIMARY KEY,
      started_at TIMESTAMP NOT NULL,
      status TEXT NOT NULL,
      -- etc.
  );
  ```

### Testing Focus
- Malformed JSON handling (table-driven)
- Session state transitions
- Concurrent session access

### Deliverables
- Bulletproof JSON validation
- Persistent session management
- Database schema updates

---

## Phase 4: Analysis Engine Core
**Estimated Time**: 4-5 hours  
**Dependencies**: Phases 1-3

### Goals
- Implement main analysis orchestration
- Add validation recovery loop
- Include progress callbacks

### Checklist
- [ ] Create `internal/analysis/engine.go`
  ```go
  type AnalysisEngine struct {
      deps AnalysisDeps
  }
  
  func (e *AnalysisEngine) Analyze(
      ctx context.Context, 
      opts AnalysisOptions,
  ) (*AnalysisReport, error)
  ```

- [ ] Implement validation recovery
  ```go
  // Maximum 3 attempts
  // Different prompts each time
  // Session state updates
  ```

- [ ] Add progress callbacks
  ```go
  type ProgressCallback func(stage string, percent int)
  ```

### Testing Focus
- Full flow integration tests
- Error injection and recovery
- Context cancellation handling

### Deliverables
- Complete analysis engine
- Robust error recovery
- Progress tracking

---

## Phase 5: Report Formatter and Display
**Estimated Time**: 3-4 hours  
**Dependencies**: Phase 1 (types)

### Goals
- Create beautiful, actionable report display
- Use lipgloss for consistent styling
- Build interactive menu system

### Checklist
- [ ] Create `internal/analysis/formatter.go`
  ```go
  type CLIFormatter struct {
      styles *Styles
  }
  
  // Implements ReportFormatter interface
  // Uses lipgloss for styling
  ```

- [ ] Create `internal/analysis/styles.go`
  ```go
  // Consistent styling definitions
  // Reusable components
  ```

- [ ] Implement display helpers
  - Coherence score visualization
  - Issue severity indicators
  - Pattern impact tables

### Testing Focus
- Snapshot tests for output
- Different terminal widths
- Color/no-color modes

### Deliverables
- Beautiful report formatting
- Interactive menu system
- Consistent styling

---

## Phase 6: Fix Application System
**Estimated Time**: 4-5 hours  
**Dependencies**: Phases 1, 4

### Goals
- Apply analysis recommendations safely
- Include transaction-safe operations
- Add rollback capability

### Checklist
- [ ] Create `internal/analysis/fixer.go`
  ```go
  type TransactionalFixApplier struct {
      storage service.Storage
      patternEngine *engine.PatternClassifier
  }
  
  // All operations in transactions
  // Validation before application
  // Rollback on any error
  ```

- [ ] Implement fix preview
  ```go
  func (f *TransactionalFixApplier) PreviewFix(
      ctx context.Context,
      fix Fix,
  ) (*FixPreview, error)
  ```

- [ ] Add dry-run support
  ```go
  // Shows what would change
  // No actual modifications
  ```

### Testing Focus
- Transaction rollback scenarios
- Concurrent fix application
- Dry-run accuracy

### Deliverables
- Safe fix application
- Preview capabilities
- Rollback support

---

## Phase 7: CLI Command Integration
**Estimated Time**: 3-4 hours  
**Dependencies**: All previous phases

### Goals
- Create seamless CLI experience
- Add all necessary flags and options
- Include session continuation support

### Checklist
- [ ] Create `cmd/spice/analyze.go`
  ```go
  var analyzeCmd = &cobra.Command{
      Use:   "analyze",
      Short: "AI-powered transaction analysis",
      RunE:  runAnalyze,
  }
  ```

- [ ] Implement flags
  - `--year`, `--from`, `--to`
  - `--focus` (coherence|patterns|categories)
  - `--dry-run`, `--auto-apply`
  - `--continue-session`

- [ ] Add interrupt handling
  ```go
  // Graceful shutdown
  // Session saving
  // Clear user messaging
  ```

### Testing Focus
- CLI integration tests
- Flag validation
- User flow testing

### Deliverables
- Complete CLI command
- All flags working
- Smooth user experience

---

## Phase 8: Testing and Polish
**Estimated Time**: 4-6 hours  
**Dependencies**: All previous phases

### Goals
- Achieve >90% test coverage
- Optimize performance
- Complete documentation

### Checklist
- [ ] Unit test coverage
  - All packages >90%
  - Critical paths 100%

- [ ] Integration tests
  - Full analysis flow
  - Error scenarios
  - Large datasets

- [ ] Performance optimization
  - Memory profiling
  - Prompt size optimization
  - Parallel processing

- [ ] Documentation
  - Code examples
  - User guide
  - Troubleshooting

### Testing Focus
- Fuzz testing JSON validation
- Benchmark performance
- Load testing with large datasets

### Deliverables
- Production-ready code
- Complete test suite
- Full documentation

---

## Implementation Order Recommendation

1. **Phase 1** - Foundation (required for all others)
2. **Phase 3** - Session management (can be done in parallel with Phase 2)
3. **Phase 2** - Prompt building
4. **Phase 4** - Engine core (requires 1-3)
5. **Phase 5** - Formatting (can be done anytime after Phase 1)
6. **Phase 6** - Fix application (requires Phase 4)
7. **Phase 7** - CLI integration (requires all others)
8. **Phase 8** - Testing and polish

## Key Go Best Practices Throughout

1. **No Type Casting**: All types are explicit, no `interface{}`
2. **Small Interfaces**: Each interface has 1-3 methods maximum
3. **Dependency Injection**: All dependencies passed via constructors
4. **Error Wrapping**: Use `fmt.Errorf` with `%w` for error chains
5. **Context Usage**: All operations accept `context.Context`
6. **Table-Driven Tests**: Comprehensive test cases in tables
7. **Early Returns**: Reduce nesting with early error returns
8. **Named Returns**: Avoid them except for deferred cleanup