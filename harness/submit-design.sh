#!/bin/bash
set -e

TASK_FILE="$1"
REPO_PATH="/Users/hal/projects/budgetapp"

if [[ -z "$TASK_FILE" ]]; then
    echo "Usage: submit-design.sh <task-file>"
    echo "Example: submit-design.sh design-tasks/03-budget-limits.md"
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

# Extract description from Overview section or first paragraph
DESCRIPTION=$(awk '/^## Overview/,/^## /{if(/^## / && !/^## Overview/)exit; if(!/^## Overview/ && !/^$/)print}' "$TASK_FILE" | head -3 | tr '\n' ' ')
if [[ -z "$DESCRIPTION" ]]; then
    DESCRIPTION=$(awk 'NR>2 && /^[A-Za-z]/{print; exit}' "$TASK_FILE")
fi

echo "=== Submitting design task ==="
echo "  Title: $TITLE"
echo "  File: $TASK_FILE"
echo "  Repo: $REPO_PATH"

# Create the design task with full details
TASK_ID=$(cc add "$TITLE" \
    --type design \
    --repo "$REPO_PATH" \
    --description "$DESCRIPTION" \
    --paths SPEC.md .cc/tasks/ \
    -c "SPEC.md updated with feature requirements" \
    -c ".cc/tasks/*.json contains implementation task manifests" \
    --json 2>/dev/null | jq -r '.id // empty')

if [[ -z "$TASK_ID" ]]; then
    # Fallback: try without --json
    TASK_ID=$(cc add "$TITLE" \
        --type design \
        --repo "$REPO_PATH" \
        --description "$DESCRIPTION" \
        --paths SPEC.md .cc/tasks/ \
        -c "SPEC.md updated with feature requirements" \
        -c ".cc/tasks/*.json contains implementation task manifests" 2>&1 | tail -1)
fi

if [[ -z "$TASK_ID" ]]; then
    echo "Error: Failed to create task"
    exit 1
fi

echo "  Created task: $TASK_ID"

# Execute the task
echo "  Executing..."
cc exec "$TASK_ID"

echo "=== Design task submitted and executing ==="
echo "  Task ID: $TASK_ID"
