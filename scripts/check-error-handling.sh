#!/bin/bash
# Check for inconsistent error handling patterns
# Detects usage of fmt.Errorf with %w (should use pkgerrors.Wrap instead)

set -e

echo "Checking error handling patterns..."

# Find files using fmt.Errorf for wrapping (should use pkgerrors.Wrap)
echo ""
echo "Files using fmt.Errorf with %w (should use pkgerrors.Wrap):"
WRAP_VIOLATIONS=$(grep -rn "fmt\.Errorf.*%w" \
  --include="*.go" \
  --exclude-dir={gen,web,bin,vendor} \
  --exclude="*_test.go" \
  --exclude="pkg/errors/errors.go" \
  . || true)

if [ -n "$WRAP_VIOLATIONS" ]; then
  echo "$WRAP_VIOLATIONS"
  echo ""
  echo "❌ Found fmt.Errorf wrapping errors. Use pkgerrors.Wrap instead."
  echo ""
  echo "Example fix:"
  echo "  Before: return fmt.Errorf(\"failed to create tunnel: %w\", err)"
  echo "  After:  return pkgerrors.Wrap(err, \"failed to create tunnel\")"
  echo ""
  exit 1
else
  echo "✓ No fmt.Errorf wrapping violations found"
fi

echo ""
echo "✓ Error handling check passed!"
