#!/bin/bash
set -e

HARNESS_DIR="$(dirname "$0")"
cd "$HARNESS_DIR/.."

DESIGN_FILE="$1"

if [[ -z "$DESIGN_FILE" ]]; then
    echo "Usage: run-design-test.sh <design-file>"
    echo "Example: run-design-test.sh design-tasks/03-budget-limits.md"
    echo ""
    echo "Available designs:"
    ls -1 harness/design-tasks/
    exit 1
fi

if [[ ! -f "harness/$DESIGN_FILE" ]]; then
    echo "Error: Design file not found: harness/$DESIGN_FILE"
    exit 1
fi

# Extract title from first H1 heading
TITLE=$(grep -m1 '^# ' "harness/$DESIGN_FILE" | sed 's/^# //')
# Create release ID from title
RELEASE_ID=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | sed 's/[^a-z0-9-]//g')

echo "=========================================="
echo " BudgetApp Design Integration Test"
echo " $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo "=========================================="
echo " Design: $TITLE"
echo " Release: $RELEASE_ID"
echo "=========================================="
echo ""

START_TIME=$(date +%s)

# Phase 1: Create release
echo ">>> PHASE 1: Create release"
cc release create "$TITLE" --project /Users/hal/projects/budgetapp || {
    echo "Release may already exist, continuing..."
}
echo ""

# Phase 2: Submit design task
echo ">>> PHASE 2: Submit design task"
./harness/submit-design.sh "$DESIGN_FILE"
echo ""

# Phase 3: Wait for completion
echo ">>> PHASE 3: Wait for all tasks to complete"
./harness/wait-complete.sh || {
    echo "Warning: Some tasks failed"
}
echo ""

# Phase 4: Verification
echo ">>> PHASE 4: Run verification"
./harness/verify.sh || {
    echo "Warning: Verification had failures"
}
echo ""

# Phase 5: Report
echo ">>> PHASE 5: Generate report"
./harness/report.sh
echo ""

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "=========================================="
echo " Design Integration Test Complete"
echo " Duration: ${DURATION}s ($((DURATION / 60))m $((DURATION % 60))s)"
echo "=========================================="
