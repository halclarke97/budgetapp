#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "=== Building frontend ==="
cd frontend
npm install
npm run build
cd ..

echo "=== Building backend ==="
cd backend
go build -o budgetapp .
cd ..

echo "=== Restarting service ==="
launchctl bootout gui/$(id -u)/com.hal.budgetapp 2>/dev/null || true
sleep 1
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.hal.budgetapp.plist
sleep 2

echo "=== Health check ==="
if curl -sf http://localhost:8081/healthz > /dev/null; then
    echo "✓ Service healthy"
else
    echo "✗ Health check failed"
    exit 1
fi

echo "=== Deploy complete ==="
