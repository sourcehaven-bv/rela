#!/bin/bash
# Verify that modified Go files have corresponding test files
# Usage: ./verify-tests-exist.sh [base-branch]
# Example: ./verify-tests-exist.sh develop

set -e

BASE_BRANCH="${1:-develop}"

echo "Checking test coverage for changes against $BASE_BRANCH..."
echo "---"

ERRORS=0
WARNINGS=0

# Get list of modified Go files (excluding tests and vendor)
MODIFIED_FILES=$(git diff --name-only "$BASE_BRANCH"...HEAD -- '*.go' | \
    grep -v '_test\.go$' | \
    grep -v '^vendor/' | \
    grep -v '^cmd/' || true)

if [ -z "$MODIFIED_FILES" ]; then
    echo "No Go source files modified (excluding tests, vendor, cmd)"
    exit 0
fi

echo "Modified source files:"
echo "$MODIFIED_FILES"
echo "---"

while IFS= read -r file; do
    [ -z "$file" ] && continue

    # Derive expected test file name
    DIR=$(dirname "$file")
    BASENAME=$(basename "$file" .go)
    TEST_FILE="${DIR}/${BASENAME}_test.go"

    # Also check for package-level test file
    PKG_TEST_FILE="${DIR}/$(basename "$DIR")_test.go"

    # Check if any test file exists in the package
    PKG_TESTS=$(ls "${DIR}"/*_test.go 2>/dev/null | wc -l || echo "0")

    if [ "$PKG_TESTS" -eq 0 ]; then
        echo "ERROR: No test files found in package: $DIR"
        echo "  Modified: $file"
        echo "  Expected: ${TEST_FILE} or similar"
        ERRORS=$((ERRORS + 1))
    elif [ ! -f "$TEST_FILE" ]; then
        echo "WARNING: No specific test file for: $file"
        echo "  Expected: $TEST_FILE"
        echo "  (Package has $PKG_TESTS test file(s))"
        WARNINGS=$((WARNINGS + 1))
    else
        echo "OK: $file has corresponding test file"
    fi
done <<< "$MODIFIED_FILES"

echo "---"
echo "Results: $ERRORS errors, $WARNINGS warnings"

# Check for new functions without tests
echo ""
echo "Checking for new exported functions..."
NEW_FUNCS=$(git diff "$BASE_BRANCH"...HEAD -- '*.go' | \
    grep -E '^\+func [A-Z]' | \
    grep -v '_test\.go' || true)

if [ -n "$NEW_FUNCS" ]; then
    echo "New exported functions added:"
    echo "$NEW_FUNCS"
    echo ""
    echo "Verify these have test coverage!"
fi

if [ $ERRORS -gt 0 ]; then
    exit 1
fi

exit 0
