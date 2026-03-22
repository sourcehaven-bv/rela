#!/bin/bash
# Verify planning documentation for common issues
# Usage: ./verify-planning-syntax.sh <checklist-id>
# Example: ./verify-planning-syntax.sh PLAN-ut02

set -e

CHECKLIST_ID="${1:-}"
if [ -z "$CHECKLIST_ID" ]; then
    echo "Usage: $0 <checklist-id>"
    exit 1
fi

# Find the checklist file
CHECKLIST_FILE=$(find tickets/entities -name "${CHECKLIST_ID}.md" 2>/dev/null | head -1)
if [ -z "$CHECKLIST_FILE" ]; then
    echo "ERROR: Could not find checklist file for ${CHECKLIST_ID}"
    exit 1
fi

echo "Checking: $CHECKLIST_FILE"
echo "---"

ERRORS=0
WARNINGS=0

# Check 1: Invalid interpolation syntax
echo "Checking for invalid interpolation syntax..."
INVALID_INTERP=$(grep -n '{{[^}]*\.[^}]*}}' "$CHECKLIST_FILE" 2>/dev/null | grep -v '{{new\.' | grep -v '{{entity\.' | grep -v '{{today}}' || true)
if [ -n "$INVALID_INTERP" ]; then
    echo "ERROR: Invalid interpolation syntax found:"
    echo "$INVALID_INTERP"
    echo "  Valid patterns: {{new.property}}, {{entity.id}}, {{today}}"
    ERRORS=$((ERRORS + 1))
fi

# Check 2: Empty sections (just checkboxes, no substance)
echo "Checking for substance in sections..."
# Look for sections that only have unchecked items with no other content
SECTIONS=("Understanding" "Approach" "Risk Assessment")
for section in "${SECTIONS[@]}"; do
    SECTION_START=$(grep -n "## $section" "$CHECKLIST_FILE" 2>/dev/null | cut -d: -f1 || echo "")
    if [ -n "$SECTION_START" ]; then
        # Get next 20 lines after section header
        SECTION_CONTENT=$(tail -n "+$SECTION_START" "$CHECKLIST_FILE" | head -20 | grep -v "^## " | grep -v "^$" | head -10)
        # Check if section has any non-checkbox content
        NON_CHECKBOX=$(echo "$SECTION_CONTENT" | grep -v "^- \[" | grep -v "^#" || true)
        if [ -z "$NON_CHECKBOX" ]; then
            echo "WARNING: Section '$section' appears to have only checkboxes, no documentation"
            WARNINGS=$((WARNINGS + 1))
        fi
    fi
done

# Check 3: Files to modify should be specific paths
echo "Checking for specific file paths..."
if grep -q "Files to modify" "$CHECKLIST_FILE" 2>/dev/null; then
    FILES_SECTION=$(grep -A 10 "Files to modify" "$CHECKLIST_FILE" | grep -E "^\s*-" || true)
    if [ -z "$FILES_SECTION" ]; then
        echo "WARNING: 'Files to modify' section has no file list"
        WARNINGS=$((WARNINGS + 1))
    fi
fi

# Check 4: Acceptance criteria should be testable
echo "Checking for acceptance criteria..."
if ! grep -qi "acceptance" "$CHECKLIST_FILE" 2>/dev/null; then
    if ! grep -qi "criteria" "$CHECKLIST_FILE" 2>/dev/null; then
        echo "WARNING: No acceptance criteria found"
        WARNINGS=$((WARNINGS + 1))
    fi
fi

echo "---"
echo "Results: $ERRORS errors, $WARNINGS warnings"

if [ $ERRORS -gt 0 ]; then
    exit 1
fi

exit 0
