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

echo "=== Submitting design task ==="
echo "  Title: $TITLE"
echo "  File: $TASK_FILE"
echo "  Repo: $REPO_PATH"

# Add the task
TASK_ID=$(cc add "$TITLE" --type design --worker codex --repo "$REPO_PATH" --json | jq -r '.id')

if [[ -z "$TASK_ID" || "$TASK_ID" == "null" ]]; then
    echo "Error: Failed to create task"
    exit 1
fi

echo "  Created task: $TASK_ID"

# Execute the task
echo "  Executing..."
cc exec "$TASK_ID"

echo "=== Design task submitted and executing ==="
echo "  Task ID: $TASK_ID"
