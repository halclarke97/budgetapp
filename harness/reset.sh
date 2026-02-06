#!/bin/bash
set -e

cd "$(dirname "$0")/.."

echo "=== Resetting budgetapp to initial state ==="

# Keep: .git, harness/, SPEC.md, README.md
# Remove everything else

for item in *; do
    case "$item" in
        harness|SPEC.md|README.md)
            echo "  keeping: $item"
            ;;
        *)
            echo "  removing: $item"
            rm -rf "$item"
            ;;
    esac
done

# Also remove hidden files except .git
for item in .*; do
    case "$item" in
        .|..|.git)
            ;;
        *)
            echo "  removing: $item"
            rm -rf "$item"
            ;;
    esac
done

echo "=== Reset complete ==="
