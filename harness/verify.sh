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

run_recurring_flow_check() {
    local start_date before_stats before_total create_response recurring_pattern_id
    local expenses_response recurring_count stats_response after_total upcoming_response upcoming_count

    start_date=$(python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc) - timedelta(days=21)).strftime("%Y-%m-%d"))
PY
)

    before_stats=$(curl -sf "$API_BASE_URL/api/stats?period=month")
    before_total=$(echo "$before_stats" | jq -r '.total_expenses')
    if [[ -z "$before_total" ]]; then
        return 1
    fi

    create_response=$(curl -sf -X POST "$API_BASE_URL/api/expenses" \
        -H 'Content-Type: application/json' \
        -d "{\"amount\":7.25,\"category\":\"food\",\"note\":\"harness recurring flow\",\"date\":\"$start_date\",\"recurring\":{\"enabled\":true,\"frequency\":\"weekly\"}}")
    recurring_pattern_id=$(echo "$create_response" | jq -r '.recurring_pattern_id // empty')
    if [[ -z "$recurring_pattern_id" ]]; then
        return 1
    fi

    expenses_response=$(curl -sf "$API_BASE_URL/api/expenses")
    recurring_count=$(echo "$expenses_response" | jq -r --arg id "$recurring_pattern_id" '[.[] | select(.recurring_pattern_id == $id)] | length')
    if [[ -z "$recurring_count" || "$recurring_count" -le 1 ]]; then
        return 1
    fi

    stats_response=$(curl -sf "$API_BASE_URL/api/stats?period=month")
    after_total=$(echo "$stats_response" | jq -r '.total_expenses')
    if [[ -z "$after_total" || "$after_total" -lt $((before_total + recurring_count)) ]]; then
        return 1
    fi

    upcoming_response=$(curl -sf "$API_BASE_URL/api/recurring-expenses/upcoming?days=30")
    upcoming_count=$(echo "$upcoming_response" | jq -r --arg id "$recurring_pattern_id" '[.[] | select(.recurring_pattern_id == $id)] | length')
    if [[ -z "$upcoming_count" || "$upcoming_count" -lt 1 ]]; then
        return 1
    fi
}

echo ""
echo "--- Backend Checks ---"

if [[ -d "backend" ]]; then
    GO_CACHE_DIR="${TMPDIR:-/tmp}/budgetapp-go-cache"
    GO_MOD_CACHE_DIR="${TMPDIR:-/tmp}/budgetapp-go-mod-cache"
    mkdir -p "$GO_CACHE_DIR" "$GO_MOD_CACHE_DIR"

    check "Backend directory exists" "true"
    check "Go module exists" "[[ -f backend/go.mod ]]"
    check "Backend builds" "(cd backend && GOCACHE=\"$GO_CACHE_DIR\" GOMODCACHE=\"$GO_MOD_CACHE_DIR\" go build -o /tmp/budgetapp-server ./...)"
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
    API_PORT=18080
    API_BASE_URL="http://localhost:${API_PORT}"
    VERIFY_DATA_FILE="/tmp/budgetapp-verify-${PPID}-$$.json"
    rm -f "$VERIFY_DATA_FILE"

    # Start server in background
    PORT="$API_PORT" DATA_FILE="$VERIFY_DATA_FILE" /tmp/budgetapp-server &
    SERVER_PID=$!
    sleep 2
    
    check "Server starts" "kill -0 $SERVER_PID 2>/dev/null"
    check "jq available for API checks" "command -v jq >/dev/null 2>&1"
    check "GET /api/expenses returns 200" "curl -sf \"$API_BASE_URL/api/expenses\" > /dev/null"
    check "GET /api/categories returns 200" "curl -sf \"$API_BASE_URL/api/categories\" > /dev/null"
    check "POST /api/expenses works" "curl -sf -X POST \"$API_BASE_URL/api/expenses\" -H 'Content-Type: application/json' -d '{\"amount\":10,\"category\":\"food\",\"note\":\"test\"}' > /dev/null"
    check "GET /api/stats returns 200" "curl -sf \"$API_BASE_URL/api/stats\" > /dev/null"
    check "Recurring flow generates + updates stats" "run_recurring_flow_check"
    
    # Stop server
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
    rm -f "$VERIFY_DATA_FILE"
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
