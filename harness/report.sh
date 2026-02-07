#!/bin/bash

cd "$(dirname "$0")"

REPORT_FILE="reports/$(date '+%Y%m%d-%H%M%S').md"
mkdir -p reports

echo "=== Generating report ==="

{
    echo "# BudgetApp Integration Test Report"
    echo ""
    echo "**Date:** $(date '+%Y-%m-%d %H:%M:%S %Z')"
    echo ""
    echo "## CC Status"
    echo ""
    echo '```'
    cc status
    echo '```'
    echo ""
    echo "## Task Summary"
    echo ""
    cc tasks --json | jq -r '.[] | select(.worker_config.repo // "" | contains("budgetapp")) | "- \(.id): \(.status)"'
    echo ""
    echo "## Verification"
    echo ""
    echo '```'
    ../harness/verify.sh 2>&1 || true
    echo '```'
} > "$REPORT_FILE"

echo "  Report saved to: $REPORT_FILE"
