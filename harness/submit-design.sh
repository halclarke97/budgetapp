#!/bin/bash
set -e

TASK_FILE="$1"
REPO_PATH="/Users/hal/projects/budgetapp"

if [[ -z "$TASK_FILE" ]]; then
    echo "Usage: submit-design.sh <task-file>"
    echo "Example: submit-design.sh design-tasks/01-initial.md"
    exit 1
fi

cd "$(dirname "$0")"

if [[ ! -f "$TASK_FILE" ]]; then
    echo "Error: Task file not found: $TASK_FILE"
    exit 1
fi

# Extract title from first H1 heading
TITLE=$(grep -m1 '^# ' "$TASK_FILE" | sed 's/^# //')

if [[ -z "$TITLE" ]]; then
    echo "Error: No H1 heading found in $TASK_FILE"
    exit 1
fi

# Build description with clear constraints
DESCRIPTION="Design task: Update docs/SPEC.md with feature requirements and create implementation task manifests in .cc/tasks/.

IMPORTANT: This is a DESIGN task only. Do NOT write implementation code.
- Update docs/SPEC.md with detailed requirements
- Create .cc/tasks/*.json with implementation task manifests (status: todo)
- Do NOT create or modify backend/, frontend/, or any implementation files
- Implementation will be done by separate tasks after this design merges"

echo "=== Submitting design task ==="
echo "  Title: $TITLE"
echo "  File: $TASK_FILE"
echo "  Repo: $REPO_PATH"

# Add the task with path constraints (docs/ and .cc/ only)
TASK_ID=$(cc add "$TITLE" \
    --type design \
    --worker codex \
    --repo "$REPO_PATH" \
    --description "$DESCRIPTION" \
    --paths docs/ .cc/tasks/ \
    -c "docs/SPEC.md updated with feature design and requirements" \
    -c ".cc/tasks/*.json contains implementation task manifests with status:todo" \
    2>&1 | tail -1)

if [[ -z "$TASK_ID" || "$TASK_ID" == *"error"* || "$TASK_ID" == *"Error"* ]]; then
    echo "Error: Failed to create task"
    echo "$TASK_ID"
    exit 1
fi

echo "  Created task: $TASK_ID"

# Execute the task - use timeout and ignore failure (task starts async)
echo "  Executing..."
timeout 30 cc exec "$TASK_ID" 2>&1 || {
    sleep 2
    STATUS=$(cc tasks --json | jq -r ".[] | select(.id == \"$TASK_ID\") | .status")
    if [[ "$STATUS" == "in-progress" ]]; then
        echo "  Task started (exec timed out but task is running)"
    else
        echo "  Warning: exec may have failed, status: $STATUS"
    fi
}

echo "=== Design task submitted ==="
echo "  Task ID: $TASK_ID"
