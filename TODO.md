# TODO: Database Checkpoint System

## Overview

Implement a comprehensive checkpoint system that allows users to save and restore database states. This enables safe experimentation, easy rollback, and sharing of category configurations.

## Phase 1: Core Checkpoint Infrastructure

### 1.1 Database Schema
- [ ] Create checkpoint metadata table via migration:
  ```sql
  CREATE TABLE checkpoint_metadata (
      id TEXT PRIMARY KEY,  -- checkpoint name/tag
      created_at DATETIME NOT NULL,
      description TEXT,
      file_size INTEGER,
      row_counts TEXT,  -- JSON with counts per table
      schema_version INTEGER,
      is_auto BOOLEAN DEFAULT 0,
      parent_checkpoint TEXT  -- for tracking relationships
  );
  ```

### 1.2 Storage Structure
- [ ] Create checkpoint directory management in `internal/storage/checkpoint.go`
- [ ] Implement directory structure:
  ```
  ~/.local/share/spice/
  ├── spice.db
  └── checkpoints/
      ├── before-import.db
      ├── before-import.meta.json
      └── checkpoint-2024-06-20-1430.db
  ```

### 1.3 Core Checkpoint Operations
- [ ] Implement `CreateCheckpoint(ctx, tag, description string) error`
- [ ] Implement `ListCheckpoints(ctx) ([]CheckpointInfo, error)`
- [ ] Implement `RestoreCheckpoint(ctx, tag string) error`
- [ ] Implement `DeleteCheckpoint(ctx, tag string) error`
- [ ] Add integrity verification using SQLite's `PRAGMA integrity_check`

## Phase 2: CLI Commands

### 2.1 Basic Commands
- [ ] Create `cmd/spice/checkpoint.go` with subcommands
- [ ] Implement `spice checkpoint create [--tag name] [--description text]`
  - Auto-generate name if not provided: `checkpoint-YYYY-MM-DD-HHMM`
  - Capture row counts and file size
- [ ] Implement `spice checkpoint list`
  - Show formatted table with metadata
  - Human-readable file sizes and relative dates
- [ ] Implement `spice checkpoint restore <name>`
  - Require confirmation unless `--force` flag
  - Verify integrity before restore
- [ ] Implement `spice checkpoint delete <name>`
  - Require confirmation unless `--force` flag

### 2.2 Advanced Commands
- [ ] Implement `spice checkpoint diff <checkpoint1> [checkpoint2]`
  - Compare with current if checkpoint2 not specified
  - Show transaction/category/vendor differences
- [ ] Implement `spice checkpoint export <name> [--output file.spice]`
  - Export checkpoint with metadata for sharing
- [ ] Implement `spice checkpoint import <file.spice>`
  - Import shared checkpoint

## Phase 3: Auto-Checkpoint Integration

### 3.1 Risky Operation Hooks
- [ ] Add auto-checkpoint to `spice import` command
  - Create checkpoint before importing new transactions
  - Name: `auto-import-YYYY-MM-DD-HHMM`
- [ ] Add auto-checkpoint to `spice classify --reset-vendors`
  - Checkpoint before clearing vendor rules
- [ ] Add auto-checkpoint to database migrations
  - Checkpoint before applying new migrations

### 3.2 Configuration
- [ ] Add checkpoint settings to config:
  ```yaml
  checkpoint:
    auto_checkpoint: true
    retention_days: 30
    max_checkpoints: 10
  ```
- [ ] Implement retention policy enforcement
- [ ] Add `--no-checkpoint` flag to disable auto-checkpoint

## Phase 4: Testing

### 4.1 Unit Tests
- [ ] Test checkpoint creation with various database states
- [ ] Test restore operations and rollback scenarios
- [ ] Test diff algorithm accuracy
- [ ] Test metadata collection and storage
- [ ] Test integrity verification

### 4.2 Integration Tests
- [ ] Test full checkpoint/restore cycle
- [ ] Test auto-checkpoint triggers
- [ ] Test concurrent checkpoint operations
- [ ] Test large database handling
- [ ] Test checkpoint cleanup/retention

### 4.3 Error Scenarios
- [ ] Test handling of corrupted checkpoints
- [ ] Test disk space exhaustion
- [ ] Test interrupted checkpoint operations
- [ ] Test permission issues

## Phase 5: Documentation

### 5.1 User Documentation
- [ ] Add checkpoint section to README.md
- [ ] Document common use cases:
  - Before major imports
  - Before experimenting with categories
  - Creating shareable category templates
- [ ] Add troubleshooting guide

### 5.2 Architecture Documentation
- [ ] Update ARCHITECTURE.md with checkpoint design
- [ ] Document checkpoint file format
- [ ] Document metadata schema

## Implementation Order

1. **Start with Phase 1**: Core infrastructure (foundation)
2. **Then Phase 2.1**: Basic CLI commands (MVP functionality)
3. **Then Phase 3**: Auto-checkpoint integration (safety)
4. **Then Phase 2.2**: Advanced commands (power features)
5. **Continuous**: Testing throughout development
6. **Finally**: Documentation

## Success Criteria

- [ ] Users can create and restore checkpoints reliably
- [ ] Auto-checkpoints prevent data loss
- [ ] Performance impact < 100ms for auto-checkpoint
- [ ] Checkpoints are portable between systems
- [ ] Clear error messages guide users
- [ ] 95%+ test coverage for checkpoint code

## Future Enhancements (Post-MVP)

- Incremental checkpoints (only store diffs)
- Cloud backup integration
- Checkpoint branching (git-like model)
- Scheduled automatic checkpoints
- Compression for large databases
- Checkpoint signatures for verification