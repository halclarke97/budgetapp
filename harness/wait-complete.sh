#!/bin/bash
set -e

REPO_PATH="/Users/hal/projects/budgetapp"
POLL_INTERVAL=30
TIMEOUT_SECONDS=7200  # 2 hours
START_TIME=$(date +%s)

echo "=== Waiting for all budgetapp tasks to complete ==="
echo "  Repo: $REPO_PATH"
echo "  Poll interval: ${POLL_INTERVAL}s"
echo "  Timeout: ${TIMEOUT_SECONDS}s (2 hours)"
echo ""

while true; do
    ELAPSED=$(($(date +%s) - START_TIME))
    
    if [[ $ELAPSED -ge $TIMEOUT_SECONDS ]]; then
        echo "Error: Timeout after ${TIMEOUT_SECONDS}s"
        exit 1
    fi
    
    # Get task counts for this repo
    # cc tasks filters by repo in title/description - use grep
    TODO=$(cc tasks --status todo --json 2>/dev/null | jq -r --arg repo "$REPO_PATH" '[.[] | select(.repo == $repo)] | length')
    IN_PROGRESS=$(cc tasks --status in-progress --json 2>/dev/null | jq -r --arg repo "$REPO_PATH" '[.[] | select(.repo == $repo)] | length')
    REVIEW=$(cc tasks --status review --json 2>/dev/null | jq -r --arg repo "$REPO_PATH" '[.[] | select(.repo == $repo)] | length')
    FAILED=$(cc tasks --status failed --json 2>/dev/null | jq -r --arg repo "$REPO_PATH" '[.[] | select(.repo == $repo)] | length')
    DONE=$(cc tasks --status done --json 2>/dev/null | jq -r --arg repo "$REPO_PATH" '[.[] | select(.repo == $repo)] | length')
    
    ACTIVE=$((TODO + IN_PROGRESS + REVIEW))
    
    printf "\r[%ds] todo:%s in-progress:%s review:%s done:%s failed:%s" \
        "$ELAPSED" "$TODO" "$IN_PROGRESS" "$REVIEW" "$DONE" "$FAILED"
    
    # Check for failures
    if [[ "$FAILED" -gt 0 ]]; then
        echo ""
        echo "Error: $FAILED task(s) failed"
        cc tasks --status failed --json | jq -r --arg repo "$REPO_PATH" '.[] | select(.repo == $repo) | "  - \(.id): \(.title)"'
        exit 1
    fi
    
    # All done?
    if [[ "$ACTIVE" -eq 0 && "$DONE" -gt 0 ]]; then
        echo ""
        echo "=== All tasks complete ==="
        echo "  Total completed: $DONE"
        exit 0
    fi
    
    sleep "$POLL_INTERVAL"
done
