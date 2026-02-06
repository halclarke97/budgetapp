#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Running verification suite ==="

PASS=0
FAIL=0
RESULTS=""

VERIFY_OUTPUT_FILE="/tmp/verify-output.txt"
GO_CACHE_DIR="${GO_CACHE_DIR:-/tmp/budgetapp-go-cache}"
mkdir -p "$GO_CACHE_DIR"
export GOCACHE="$GO_CACHE_DIR"

check() {
    local name="$1"
    local cmd="$2"

    printf "  %-40s" "$name"

    if eval "$cmd" >"$VERIFY_OUTPUT_FILE" 2>&1; then
        echo "[PASS]"
        RESULTS="${RESULTS}PASS: $name\n"
        PASS=$((PASS + 1))
    else
        echo "[FAIL]"
        RESULTS="${RESULTS}FAIL: $name\n"
        FAIL=$((FAIL + 1))
        head -5 "$VERIFY_OUTPUT_FILE" | sed 's/^/       /'
    fi
}

echo ""
echo "--- Backend Checks ---"

if [[ -d "backend" ]]; then
    check "Backend directory exists" "true"
    check "Go module exists" "[[ -f backend/go.mod ]]"
    check "Backend unit tests pass" "(cd backend && go test ./...)"
    check "Backend builds" "(cd backend && go build -o /tmp/budgetapp-server ./...)"
    check "Backend has main.go" "[[ -f backend/main.go || -f backend/cmd/server/main.go ]]"
else
    check "Backend directory exists" "false"
fi

echo ""
echo "--- Frontend Checks ---"

if [[ -d "frontend" ]]; then
    check "Frontend directory exists" "true"
    check "package.json exists" "[[ -f frontend/package.json ]]"
    check "Node modules installed" "[[ -d frontend/node_modules ]] || (cd frontend && npm install)"
    check "Frontend builds" "(cd frontend && npm run build)"
else
    check "Frontend directory exists" "false"
fi

echo ""
echo "--- API Checks ---"

check "CRUD/stats API regression flow" "(cd backend && go test -run '^TestExpenseAPIFlow$' ./...)"
check "Recurring API flow is idempotent" "(cd backend && go test -run '^TestRecurringFlowAPIRegression$' ./...)"

echo ""
echo "=== Verification Summary ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo ""

export VERIFY_PASS=$PASS
export VERIFY_FAIL=$FAIL
export VERIFY_RESULTS="$RESULTS"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
exit 0
