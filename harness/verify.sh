#!/bin/bash

cd "$(dirname "$0")/.."

PASS=0
FAIL=0

check() {
    local name="$1"
    local cmd="$2"
    
    printf "  %-40s" "$name"
    if eval "$cmd" > /dev/null 2>&1; then
        echo "[PASS]"
        ((PASS++))
    else
        echo "[FAIL]"
        ((FAIL++))
    fi
}

echo "=== Running verification suite ==="
echo ""

echo "--- Backend Checks ---"
check "Backend directory exists" "[ -d backend ]"
check "Go module exists" "[ -f backend/go.mod ]"
check "Backend builds" "cd backend && go build -o /dev/null . 2>/dev/null"
check "Backend has main.go" "[ -f backend/main.go ]"

echo ""
echo "--- Frontend Checks ---"
check "Frontend directory exists" "[ -d frontend ]"
check "package.json exists" "[ -f frontend/package.json ]"
check "Node modules installed" "[ -d frontend/node_modules ] || (cd frontend && npm install)"
check "Frontend builds" "cd frontend && npm run build 2>/dev/null"

echo ""
echo "--- API Checks ---"

# Start server in background
cd backend
go build -o budgetapp-test . 2>/dev/null
./budgetapp-test &
SERVER_PID=$!
cd ..
sleep 2

check "Server starts" "curl -sf http://localhost:8080/healthz"
check "jq available for API checks" "command -v jq"
check "GET /api/expenses returns 200" "curl -sf http://localhost:8080/api/expenses"
check "GET /api/categories returns 200" "curl -sf http://localhost:8080/api/categories"
check "POST /api/expenses works" "curl -sf -X POST http://localhost:8080/api/expenses -H 'Content-Type: application/json' -d '{\"amount\":10,\"category\":\"food\",\"note\":\"test\"}'"
check "GET /api/stats returns 200" "curl -sf http://localhost:8080/api/stats"

# Kill server
kill $SERVER_PID 2>/dev/null || true
rm -f backend/budgetapp-test

echo ""
echo "=== Verification Summary ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
