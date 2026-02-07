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

# Extract first paragraph as description
DESCRIPTION=$(awk '/^[A-Za-z]/{p=1} p{print; if(/^$/)exit}' "$TASK_FILE" | head -5 | tr '\n' ' ' | sed 's/  */ /g')
if [[ -z "$DESCRIPTION" ]]; then
    DESCRIPTION="Design task from $TASK_FILE"
fi

echo "=== Submitting design task ==="
echo "  Title: $TITLE"
echo "  File: $TASK_FILE"
echo "  Repo: $REPO_PATH"

# Add the task with required fields
TASK_ID=$(cc add "$TITLE" \
    --type design \
    --worker codex \
    --repo "$REPO_PATH" \
    --description "$DESCRIPTION" \
    --paths SPEC.md .cc/tasks/ \
    -c "SPEC.md updated with feature design" \
    -c ".cc/tasks/*.json contains implementation task manifests" \
    2>&1 | tail -1)

if [[ -z "$TASK_ID" || "$TASK_ID" == *"error"* || "$TASK_ID" == *"Error"* ]]; then
    echo "Error: Failed to create task"
    echo "$TASK_ID"
    exit 1
fi

echo "  Created task: $TASK_ID"

# Execute the task
echo "  Executing..."
cc exec "$TASK_ID"

echo "=== Design task submitted and executing ==="
echo "  Task ID: $TASK_ID"
