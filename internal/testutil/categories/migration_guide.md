# Test Migration Guide: Dynamic Categories

This guide helps migrate existing tests to use the new dynamic category system.

## Quick Migration Patterns

### 1. Simple Category Usage

**Before:**
```go
func TestFeature(t *testing.T) {
    store, cleanup := createTestStorage(t)
    defer cleanup()
    // Test assumes "Groceries" exists...
}
```

**After:**
```go
func TestFeature(t *testing.T) {
    db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
        return b.WithCategory(categories.CategoryGroceries)
    })
    store := db.Storage
    // "Groceries" is guaranteed to exist
}
```

### 2. Multiple Categories

**Before:**
```go
// Test uses "Food", "Shopping", "Transportation"
store, cleanup := createTestStorage(t)
defer cleanup()
```

**After:**
```go
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithCategories(
        categories.CategoryFoodDining,
        categories.CategoryShopping,
        categories.CategoryTransportation,
    )
})
store := db.Storage
```

### 3. Using Fixtures for Common Sets

**After:**
```go
// For basic tests
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithFixture(categories.FixtureStandard)
})

// For comprehensive tests
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithFixture(categories.FixtureComprehensive)
})
```

### 4. Custom Test Categories

**After:**
```go
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithCategories(
        "Custom Category 1",
        "Custom Category 2",
        "Edge Case Category",
    )
})
```

### 5. Combining Fixtures and Custom Categories

**After:**
```go
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.
        WithBasicCategories().              // Standard set
        WithCategory("Special Test Case").  // Additional custom
        WithFixture(categories.FixtureTestingOnly) // Test-specific set
})
```

## Common Test Scenarios

### Vendor Rule Tests
```go
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithFixture(categories.FixtureVendorTesting)
})
```

### Classification Tests
```go
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithCategories(
        categories.CategoryInitial,
        categories.CategoryUserCorrected,
        categories.CategoryFinal,
    )
})
```

### Integration Tests
```go
db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
    return b.WithFixture(categories.FixtureComprehensive)
})
```

## Finding Required Categories

1. **Run the test** - It will fail with "category not found" or similar
2. **Check the error** - Note which category name is missing
3. **Add to builder** - Either use a predefined constant or add as string
4. **Verify** - Run test again to ensure it passes

## Best Practices

1. **Use Constants** - Prefer `categories.CategoryGroceries` over `"Groceries"`
2. **Use Fixtures** - For consistency across related tests
3. **Document Custom Categories** - Add comments explaining why custom categories are needed
4. **Minimal Sets** - Only add categories actually used in the test
5. **Avoid Duplication** - Builder handles duplicate categories gracefully

## Troubleshooting

### "Category not found" errors
- The test is trying to use a category that wasn't seeded
- Add the missing category to your builder

### Test isolation issues
- Each test gets its own in-memory database
- Categories don't leak between tests
- Use t.Parallel() safely

### Performance concerns
- Category creation is fast (in-memory SQLite)
- Fixtures are reusable but create fresh data each time
- No cleanup needed - database is destroyed after test