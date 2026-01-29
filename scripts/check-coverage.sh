#!/bin/bash
set -e

# Coverage threshold checker
# Ensures code coverage meets minimum requirements

COVERAGE_FILE="${1:-coverage.out}"

if [ ! -f "$COVERAGE_FILE" ]; then
    echo "❌ Coverage file not found: $COVERAGE_FILE"
    echo "Run 'just test-coverage' first"
    exit 1
fi

echo "📊 Checking coverage thresholds..."
echo ""

# Generate coverage report
go tool cover -func="$COVERAGE_FILE" > coverage.txt

# Extract total coverage percentage
TOTAL_COV=$(go tool cover -func="$COVERAGE_FILE" | grep total | awk '{print $3}' | sed 's/%//')
echo "📈 Total coverage: ${TOTAL_COV}%"
echo ""

# Overall minimum threshold
MIN_TOTAL=45.0
if awk "BEGIN {exit !($TOTAL_COV < $MIN_TOTAL)}"; then
    echo "❌ Total coverage ${TOTAL_COV}% is below minimum threshold ${MIN_TOTAL}%"
    exit 1
fi
echo "✅ Total coverage ${TOTAL_COV}% meets minimum threshold ${MIN_TOTAL}%"
echo ""

# Check per-package thresholds for critical packages
echo "🔍 Checking per-package coverage thresholds..."
echo ""

check_package() {
    local package=$1
    local min_threshold=$2
    local display_name=$3

    # Get package coverage from go test output
    local test_output=$(go test "./${package}" -coverprofile=/dev/null -covermode=atomic 2>&1)
    local cov=$(echo "$test_output" | grep -o 'coverage: [0-9.]*%' | grep -o '[0-9.]*')

    if [ -z "$cov" ] || [ "$cov" = "" ]; then
        echo "⚠️  Package ${display_name} not found in coverage report"
        return 0
    fi

    printf "%-40s %6s%% (min: %5s%%) " "$display_name" "$cov" "$min_threshold"

    # Use awk for floating point comparison (more portable than bc)
    if awk "BEGIN {exit !($cov < $min_threshold)}"; then
        echo "❌"
        return 1
    fi
    echo "✅"
    return 0
}

FAILED=0

# Critical packages with high thresholds
check_package "internal/model" 95.0 "internal/model" || FAILED=1
check_package "internal/errors" 95.0 "internal/errors" || FAILED=1
check_package "internal/output" 90.0 "internal/output" || FAILED=1
check_package "internal/project" 85.0 "internal/project" || FAILED=1
check_package "internal/markdown" 85.0 "internal/markdown" || FAILED=1
check_package "internal/filter" 85.0 "internal/filter" || FAILED=1
check_package "internal/graph" 75.0 "internal/graph" || FAILED=1
check_package "internal/metamodel" 65.0 "internal/metamodel" || FAILED=1
check_package "internal/importer" 65.0 "internal/importer" || FAILED=1

echo ""

if [ $FAILED -eq 1 ]; then
    echo "❌ One or more packages failed coverage thresholds"
    echo ""
    echo "To see detailed coverage:"
    echo "  just coverage-html"
    echo ""
    exit 1
fi

echo "✅ All coverage thresholds passed!"
echo ""
echo "📊 Coverage by package:"
go test ./... -coverprofile="$COVERAGE_FILE" -covermode=atomic 2>&1 | \
    grep "coverage:" | \
    grep -v "^[[:space:]]*github" | \
    sort -k2 -t: -n

# Clean up
rm -f coverage.txt

exit 0
