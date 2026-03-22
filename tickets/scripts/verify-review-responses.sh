#!/bin/bash
# Verify review responses are properly addressed before completion
# Usage: ./verify-review-responses.sh <ticket-id>
# Example: ./verify-review-responses.sh TKT-a2qn

set -e

TICKET_ID="${1:-}"
if [ -z "$TICKET_ID" ]; then
    echo "Usage: $0 <ticket-id>"
    exit 1
fi

echo "Checking review responses for: $TICKET_ID"
echo "---"

ERRORS=0
WARNINGS=0

# Find all review-response entities linked to this ticket
# This uses rela CLI directly
cd tickets

# Get relations from the ticket
RELATIONS=$(rela list --type has-review-response --from "$TICKET_ID" 2>/dev/null || echo "")

if [ -z "$RELATIONS" ]; then
    echo "No review responses linked to $TICKET_ID"
    echo "(This is OK if no code review was performed yet)"
    exit 0
fi

echo "Review responses found:"
echo "$RELATIONS"
echo "---"

# Check each review response
for rr_id in $(echo "$RELATIONS" | grep -oE 'RR-[a-z0-9]+'); do
    echo "Checking $rr_id..."

    # Get the review response details
    RR_FILE=$(find entities -name "${rr_id}.md" 2>/dev/null | head -1)
    if [ -z "$RR_FILE" ]; then
        echo "  WARNING: Could not find file for $rr_id"
        WARNINGS=$((WARNINGS + 1))
        continue
    fi

    # Extract status and severity from frontmatter
    STATUS=$(grep -E '^status:' "$RR_FILE" | head -1 | sed 's/status: *//' | tr -d ' ')
    SEVERITY=$(grep -E '^severity:' "$RR_FILE" | head -1 | sed 's/severity: *//' | tr -d ' ')

    echo "  Status: $STATUS, Severity: $SEVERITY"

    # Check critical/significant are not open
    if [ "$STATUS" = "open" ]; then
        if [ "$SEVERITY" = "critical" ]; then
            echo "  ERROR: Critical finding is still open!"
            ERRORS=$((ERRORS + 1))
        elif [ "$SEVERITY" = "significant" ]; then
            echo "  ERROR: Significant finding is still open!"
            ERRORS=$((ERRORS + 1))
        else
            echo "  WARNING: Minor/nit finding is still open"
            WARNINGS=$((WARNINGS + 1))
        fi
    fi

    # Check addressed items have resolution
    if [ "$STATUS" = "addressed" ]; then
        if ! grep -q '^resolution:' "$RR_FILE" || [ -z "$(grep '^resolution:' "$RR_FILE" | sed 's/resolution: *//')" ]; then
            echo "  ERROR: Addressed but no resolution documented"
            ERRORS=$((ERRORS + 1))
        fi
    fi

    # Check wont-fix/deferred have reason
    if [ "$STATUS" = "wont-fix" ] || [ "$STATUS" = "deferred" ]; then
        if ! grep -q '^reason:' "$RR_FILE" || [ -z "$(grep '^reason:' "$RR_FILE" | sed 's/reason: *//')" ]; then
            echo "  ERROR: $STATUS but no reason documented"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done

cd ..

echo "---"
echo "Results: $ERRORS errors, $WARNINGS warnings"

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "BLOCKED: Cannot complete ticket with unresolved critical/significant findings"
    exit 1
fi

if [ $WARNINGS -gt 0 ]; then
    echo ""
    echo "WARNING: Consider addressing remaining minor/nit findings"
fi

exit 0
