#!/bin/bash
set -e

REPO_PATH="/Users/hal/projects/budgetapp"
POLL_INTERVAL=30
TIMEOUT=7200  # 2 hours

echo "=== Waiting for all budgetapp tasks to complete ==="
echo "  Repo: $REPO_PATH"
echo "  Poll interval: ${POLL_INTERVAL}s"
echo "  Timeout: ${TIMEOUT}s (2 hours)"
echo ""

START_TIME=$(date +%s)

while true; do
    ELAPSED=$(($(date +%s) - START_TIME))
    
    if [[ $ELAPSED -gt $TIMEOUT ]]; then
        echo "Timeout reached after ${TIMEOUT}s"
        exit 1
    fi
    
    # Get task counts for budgetapp tasks
    TASKS=$(cc tasks --json 2>/dev/null | jq -r '[.[] | select(.worker_config.repo // "" | contains("budgetapp"))] | {
        todo: [.[] | select(.status == "todo")] | length,
        in_progress: [.[] | select(.status == "in-progress")] | length,
        review: [.[] | select(.status == "review")] | length,
        done: [.[] | select(.status == "done")] | length,
        failed: [.[] | select(.status == "failed")] | length
    }')
    
    TODO=$(echo "$TASKS" | jq -r '.todo')
    IN_PROGRESS=$(echo "$TASKS" | jq -r '.in_progress')
    REVIEW=$(echo "$TASKS" | jq -r '.review')
    DONE=$(echo "$TASKS" | jq -r '.done')
    FAILED=$(echo "$TASKS" | jq -r '.failed')
    
    echo "[${ELAPSED}s] todo:$TODO in-progress:$IN_PROGRESS review:$REVIEW done:$DONE failed:$FAILED"
    
    # Check if any failed
    if [[ "$FAILED" -gt 0 ]]; then
        echo "Error: $FAILED task(s) failed"
        exit 1
    fi
    
    # Check if all complete (nothing pending)
    PENDING=$((TODO + IN_PROGRESS + REVIEW))
    if [[ "$PENDING" -eq 0 && "$DONE" -gt 0 ]]; then
        echo "All tasks complete!"
        exit 0
    fi
    
    sleep $POLL_INTERVAL
done
