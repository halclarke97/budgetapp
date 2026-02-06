#!/bin/bash
set -e

HARNESS_DIR="$(dirname "$0")"
cd "$HARNESS_DIR/.."

echo "=========================================="
echo " BudgetApp Integration Test"
echo " $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo "=========================================="
echo ""

START_TIME=$(date +%s)

# Phase 1: Reset
echo ">>> PHASE 1: Reset repository"
./harness/reset.sh
echo ""

# Phase 2: Submit initial design
echo ">>> PHASE 2: Submit initial design task"
./harness/submit-design.sh design-tasks/01-initial.md
echo ""

# Phase 3: Wait for completion
echo ">>> PHASE 3: Wait for initial implementation"
./harness/wait-complete.sh || {
    echo "Warning: Some tasks failed during initial implementation"
}
echo ""

# Phase 4: Initial verification
echo ">>> PHASE 4: Run initial verification"
./harness/verify.sh || {
    echo "Warning: Verification had failures"
}
echo ""

# Phase 5: Submit enhancement
echo ">>> PHASE 5: Submit enhancement design task"
./harness/submit-design.sh design-tasks/02-enhancement.md
echo ""

# Phase 6: Wait for enhancement completion
echo ">>> PHASE 6: Wait for enhancement implementation"
./harness/wait-complete.sh || {
    echo "Warning: Some tasks failed during enhancement"
}
echo ""

# Phase 7: Final verification
echo ">>> PHASE 7: Run final verification"
./harness/verify.sh || {
    echo "Warning: Final verification had failures"
}
echo ""

# Phase 8: Generate report
echo ">>> PHASE 8: Generate report"
./harness/report.sh
echo ""

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "=========================================="
echo " Integration Test Complete"
echo " Duration: ${DURATION}s ($((DURATION / 60))m $((DURATION % 60))s)"
echo "=========================================="
