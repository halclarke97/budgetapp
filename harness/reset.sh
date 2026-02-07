#!/bin/bash
set -e

cd "$(dirname "$0")/.."
HARNESS_DIR="$(pwd)/harness"

echo "=== Resetting budgetapp to initial state ==="

# Step 1: Clean up CC tasks and releases for budgetapp
echo "  Cleaning CC tasks and releases..."
cc tasks --json 2>/dev/null | jq -r '.[] | select(.status != "done") | select(.worker_config.repo // "" | contains("budgetapp")) | .id' | while read id; do
    cc kill "$id" 2>/dev/null || true
    cc edit "$id" --status done 2>/dev/null || true
done
cc release list --json 2>/dev/null | jq -r '.[] | select(.status != "closed") | select(.project | contains("budgetapp")) | .id' | while read id; do
    cc release cancel "$id" 2>/dev/null || true
done

# Step 2: Clean up worktrees
echo "  Cleaning worktrees..."
git worktree prune 2>/dev/null || true
for wt in ~/.command-center/worktrees/*budgetapp* ~/.command-center/worktrees/*implement-budget*; do
    [ -d "$wt" ] && rm -rf "$wt"
done

# Step 3: Backup harness to temp location
echo "  Backing up harness..."
BACKUP_DIR=$(mktemp -d)
cp -r "$HARNESS_DIR" "$BACKUP_DIR/"

# Step 4: Remove all local branches except main
echo "  Removing local branches..."
git checkout main 2>/dev/null || git checkout -b main
git branch | grep -v '^\* main$' | grep -v '^  main$' | xargs -r git branch -D 2>/dev/null || true

# Step 5: Create orphan branch
echo "  Creating clean orphan branch..."
git checkout --orphan reset-temp
git rm -rf . 2>/dev/null || true

# Step 6: Restore harness from backup
echo "  Restoring harness..."
cp -r "$BACKUP_DIR/harness" .

# Step 7: Create minimal SPEC.md
cat > SPEC.md << 'EOF'
# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

## Overview

BudgetApp helps users track daily expenses with quick entry, category-based organization, and spending insights.

## Requirements

_To be designed by implementation task._
EOF

# Step 8: Create minimal README.md
cat > README.md << 'EOF'
# BudgetApp

CC integration test project.
EOF

# Step 9: Commit initial state
git add -A
git commit -m "Initial commit - blank slate for integration test"

# Step 10: Replace main with this orphan branch
echo "  Replacing main branch..."
git branch -D main 2>/dev/null || true
git branch -m main

# Step 11: Force push to remote
echo "  Force pushing to remote..."
git push origin main --force

# Step 12: Clean up remote branches
echo "  Cleaning remote branches..."
git fetch origin --prune
git branch -r | grep -v 'origin/main' | grep -v 'origin/HEAD' | sed 's/origin\///' | while read branch; do
    git push origin --delete "$branch" 2>/dev/null || true
done

# Step 13: Cleanup backup
rm -rf "$BACKUP_DIR"

echo "=== Reset complete ==="
