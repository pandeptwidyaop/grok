#!/bin/bash
# Check for inconsistent logging patterns
# Detects usage of fmt.Printf/Println in production code (should use logger instead)

set -e

echo "Checking logging patterns..."

# Find files using fmt.Printf in production code (should use logger)
echo ""
echo "Files using fmt.Printf/Println in production code:"
LOGGING_VIOLATIONS=$(grep -rn "fmt\.Printf\|fmt\.Println" \
  --include="*.go" \
  --exclude-dir={gen,web,bin,vendor} \
  --exclude="*_test.go" \
  internal/ pkg/ \
  | grep -v "// nolint" || true)

if [ -n "$LOGGING_VIOLATIONS" ]; then
  echo "$LOGGING_VIOLATIONS"
  echo ""
  echo "❌ Found fmt.Printf/Println in production code. Use logger instead."
  echo ""
  echo "Example fix:"
  echo "  Before: fmt.Printf(\"Tunnel created: %s\\n\", tunnelID)"
  echo "  After:  logger.InfoEvent().Str(\"tunnel_id\", tunnelID).Msg(\"Tunnel created\")"
  echo ""
  echo "Note: fmt.Printf is allowed in cmd/ directory for CLI output"
  exit 1
else
  echo "✓ No logging violations found in production code"
fi

echo ""
echo "✓ Logging check passed!"
