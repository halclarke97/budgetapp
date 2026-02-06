#!/bin/bash
set -e

cd "$(dirname "$0")/.."

echo "=== Running verification suite ==="

PASS=0
FAIL=0
RESULTS=""

check() {
    local name="$1"
    local cmd="$2"
    
    printf "  %-40s" "$name"
    
    if eval "$cmd" > /tmp/verify-output.txt 2>&1; then
        echo "[PASS]"
        RESULTS="${RESULTS}PASS: $name\n"
        PASS=$((PASS + 1))
    else
        echo "[FAIL]"
        RESULTS="${RESULTS}FAIL: $name\n"
        FAIL=$((FAIL + 1))
        # Show error output
        head -5 /tmp/verify-output.txt | sed 's/^/       /'
    fi
}

echo ""
echo "--- Backend Checks ---"

if [[ -d "backend" ]]; then
    check "Backend directory exists" "true"
    check "Go module exists" "[[ -f backend/go.mod ]]"
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

# Only run API checks if backend builds
if [[ -f "/tmp/budgetapp-server" ]]; then
    # Start server in background
    /tmp/budgetapp-server &
    SERVER_PID=$!
    sleep 2
    
    check "Server starts" "kill -0 $SERVER_PID 2>/dev/null"
    check "GET /api/expenses returns 200" "curl -sf http://localhost:8080/api/expenses > /dev/null"
    check "GET /api/categories returns 200" "curl -sf http://localhost:8080/api/categories > /dev/null"
    check "POST /api/expenses works" "curl -sf -X POST http://localhost:8080/api/expenses -H 'Content-Type: application/json' -d '{\"amount\":10,\"category\":\"food\",\"note\":\"test\"}' > /dev/null"
    check "GET /api/stats returns 200" "curl -sf http://localhost:8080/api/stats > /dev/null"
    
    # Stop server
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
else
    echo "  (skipped - backend did not build)"
fi

echo ""
echo "=== Verification Summary ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo ""

# Export results for report
export VERIFY_PASS=$PASS
export VERIFY_FAIL=$FAIL
export VERIFY_RESULTS="$RESULTS"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
exit 0
