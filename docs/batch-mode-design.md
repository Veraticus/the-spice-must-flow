# Batch Classification Mode Design

## Overview

This document explores making batch mode the default classification approach for `spice classify`, based on real-world usage showing it's more efficient without sacrificing accuracy.

## Current State

### Traditional Mode (Sequential)
- Processes merchants one by one
- Individual LLM API calls per merchant
- Immediate user interaction for low confidence items
- Supports resume from interruption
- Predictable but slow

### Batch Mode (Parallel)
- Groups merchants and processes in parallel
- Batches LLM calls (though currently still 1:1)
- Auto-accepts high confidence items
- Manual review phase at end
- 5-10x faster

## Performance Comparison

### API Efficiency
- **Traditional**: 1 API call per merchant (e.g., 300 merchants = 300 calls)
- **Batch**: Same currently, but architecture supports true batching
- **Potential**: Could batch 10-20 merchants per API call

### Processing Speed
- **Traditional**: ~2-5 seconds per merchant (sequential)
- **Batch**: ~1 second per merchant (5 parallel workers)
- **Speedup**: 5-10x faster for large datasets

### User Experience
- **Traditional**: Unpredictable interruptions (5-30 merchants between prompts)
- **Batch**: Predictable phases (auto-accept then review)
- **Cognitive Load**: Lower with batch mode

## Accuracy Analysis

The key insight: **Batch mode doesn't change classification accuracy**

- Same LLM model
- Same transaction data
- Same prompt engineering
- Same category matching logic

The only differences are:
1. When the API calls happen (parallel vs sequential)
2. When user reviews happen (end vs throughout)
3. How results are presented

## Proposed Changes

### 1. Make Batch Mode Default

```bash
# Current behavior
spice classify                    # Sequential mode
spice classify --batch           # Batch mode

# Proposed behavior  
spice classify                    # Batch mode (default)
spice classify --sequential      # Sequential mode (opt-in)
```

### 2. Smart Defaults

```go
// Default options optimized for most users
DefaultBatchOptions() BatchClassificationOptions {
    return BatchClassificationOptions{
        AutoAcceptThreshold: 0.95,    // Very high confidence only
        BatchSize:           20,       // Good balance
        ParallelWorkers:     5,        // Reasonable for most systems
        SkipManualReview:    false,    // Still review uncertain items
    }
}
```

### 3. Enhanced Batch Mode Features

#### Resume Support
Add checkpoint saving between phases:
- After parallel processing phase
- After auto-accept phase
- During manual review phase

#### True Batch API Calls
Implement `BatchSuggestCategories` to reduce API calls:
- Group similar transactions
- Send 10-20 at once
- Parse batch responses

#### Progressive Processing
Show live progress during parallel phase:
```
Processing merchants... [=====>    ] 127/300 (42%)
✓ Coffee shops: 45 transactions auto-classified
✓ Groceries: 23 transactions auto-classified
⚡ Currently processing: Airlines, Hotels, Car Rental...
```

## Migration Strategy

### Phase 1: Feature Parity
- [ ] Add resume support to batch mode
- [ ] Implement true batch API calls
- [ ] Add progress visualization

### Phase 2: Soft Launch
- [ ] Make batch mode prominent in help text
- [ ] Add performance comparison in docs
- [ ] Encourage batch mode in CLAUDE.md

### Phase 3: Default Switch
- [ ] Change default behavior
- [ ] Add --sequential flag
- [ ] Update all documentation

## Command Examples

### After Migration

```bash
# Default (batch mode, 95% auto-accept)
spice classify

# Auto-only mode (no manual review)
spice classify --auto-only

# Custom threshold
spice classify --auto-accept-threshold=0.90

# Maximum performance
spice classify --parallel-workers=10 --batch-size=50

# Legacy sequential mode
spice classify --sequential
```

## Benefits Summary

1. **5-10x faster** processing
2. **More predictable** user experience  
3. **Same accuracy** as sequential mode
4. **Better resource usage** (parallel processing)
5. **Reduced API costs** (with true batching)

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| User confusion from change | Clear migration messages, keep --sequential option |
| Missing resume support | Implement checkpoint system |
| API rate limits | Configurable parallelism, exponential backoff |
| Memory usage | Reasonable default batch sizes |

## Conclusion

Batch mode should become the default because it's:
- Significantly faster without accuracy loss
- More predictable for users
- Better architected for future improvements
- Already proven in production use

The traditional sequential mode would remain available for specific use cases requiring fine-grained control or resume support.