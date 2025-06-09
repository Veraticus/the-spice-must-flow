#!/usr/bin/env bash
# Script to fix common linting issues

set -euo pipefail

echo "🔧 Fixing common linting issues..."

# Fix errcheck issues for defer statements
echo "  → Adding error checks for defer statements..."

# Handle defer tx.Rollback() errors
find . -name "*.go" -type f -exec sed -i 's/defer tx\.Rollback()/defer func() { _ = tx.Rollback() }()/g' {} \;

# Handle defer rows.Close() errors
find . -name "*.go" -type f -exec sed -i 's/defer rows\.Close()/defer func() { _ = rows.Close() }()/g' {} \;

# Handle defer stmt.Close() errors
find . -name "*.go" -type f -exec sed -i 's/defer stmt\.Close()/defer func() { _ = stmt.Close() }()/g' {} \;

# Handle defer store.Close() errors
find . -name "*.go" -type f -exec sed -i 's/defer store\.Close()/defer func() { _ = store.Close() }()/g' {} \;

# Fix simple store.Close() calls (not in defer)
find . -name "*.go" -type f -exec sed -i 's/^\(\s*\)store\.Close()$/\1_ = store.Close()/g' {} \;

# Fix directory permissions
echo "  → Fixing directory permissions..."
find . -name "*.go" -type f -exec sed -i 's/os\.MkdirAll(dir, 0755)/os.MkdirAll(dir, 0750)/g' {} \;

# Run gofmt
echo "  → Running gofmt..."
gofmt -w .

echo "✅ Done!"