#!/usr/bin/env bash
# Fix all remaining lint issues

set -euo pipefail

echo "🔧 Fixing all lint issues..."

# Fix viper.BindPFlag in flow.go
echo "  → Fixing viper.BindPFlag error checks..."
sed -i 's/viper\.BindPFlag(\([^)]*\))/_ = viper.BindPFlag(\1)/g' cmd/spice/flow.go

# Fix tx.Rollback in migrations.go
echo "  → Fixing tx.Rollback error checks..."
sed -i 's/tx\.Rollback()/_ = tx.Rollback()/g' internal/storage/migrations.go

# Fix error checks in test setup functions
echo "  → Fixing test setup error checks..."
sed -i 's/^\(\s*\)s\.SaveTransactions(ctx, txns)$/\1_ = s.SaveTransactions(ctx, txns)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.SaveClassification(ctx, classification)$/\1_ = s.SaveClassification(ctx, classification)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.SaveVendor(ctx, initial)$/\1_ = s.SaveVendor(ctx, initial)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.SaveVendor(ctx, v)$/\1_ = s.SaveVendor(ctx, v)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.SaveClassification(ctx, classification)$/\1_ = s.SaveClassification(ctx, classification)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.SaveProgress(ctx, initial)$/\1_ = s.SaveProgress(ctx, initial)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.SaveProgress(ctx, progress)$/\1_ = s.SaveProgress(ctx, progress)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)s\.ClearProgress(ctx)$/\1_ = s.ClearProgress(ctx)/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)store1\.Close()$/\1_ = store1.Close()/g' internal/storage/sqlite_test.go
sed -i 's/^\(\s*\)store2\.Close()$/\1_ = store2.Close()/g' internal/storage/sqlite_test.go

# Fix return store, func() { store.Close() }
sed -i 's/return store, func() { store\.Close() }/return store, func() { _ = store.Close() }/g' internal/storage/sqlite_test.go

# Fix tx.Rollback() in test
sed -i 's/^\(\s*\)tx\.Rollback()$/\1_ = tx.Rollback()/g' internal/storage/sqlite_test.go

echo "✅ Done!"