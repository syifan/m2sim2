#!/bin/bash
# Bootstrap script - reset agent system to clean state
# Usage: ./bootstrap.sh

set -e
cd "$(dirname "$0")"

echo "=== M2Sim Agent Bootstrap ==="

# 1. Stop orchestrator (but NOT monitor - user may be running bootstrap from monitor UI)
echo "Stopping orchestrator..."
pkill -f "node orchestrator.js" 2>/dev/null && echo "  Killed orchestrator" || echo "  No orchestrator running"

# 2. Remove temporary files
echo "Removing temporary files..."
rm -f state.json && echo "  Removed state.json" || true
rm -f orchestrator.log && echo "  Removed orchestrator.log" || true
rm -f nohup.out && echo "  Removed nohup.out" || true
rm -f monitor/monitor.log && echo "  Removed monitor/monitor.log" || true
rm -f monitor/server.log && echo "  Removed monitor/server.log" || true
rm -rf .DS_Store && echo "  Removed .DS_Store" || true

# 3. Remove all workers
echo "Removing workers..."
if [ -d "workers" ]; then
  rm -rf workers/*
  echo "  Cleared workers/"
else
  mkdir -p workers
  echo "  Created empty workers/"
fi

# 4. Remove workspace
echo "Removing workspace..."
if [ -d "../workspace" ]; then
  rm -rf ../workspace
  echo "  Removed workspace/"
else
  echo "  No workspace/ to remove"
fi

# 5. Remove messages
echo "Removing messages..."
rm -rf messages/* 2>/dev/null && echo "  Cleared messages/" || mkdir -p messages && echo "  Created empty messages/"

# 6. Create new tracker issue
echo "Creating new tracker issue..."
ISSUE_URL=$(gh issue create --title "Agent Tracker" --body "# Agent Tracker

## 📋 Task Queues

_(No workers hired yet — Athena will build the team)_

## 📊 Status
- **Action count:** 0
- **Last cycle:** Not started
")
ISSUE_NUM=$(echo "$ISSUE_URL" | grep -oE '[0-9]+$')
echo "  Created issue #$ISSUE_NUM"

# 7. Update config with new issue number
echo "Updating config.yaml..."
sed -i '' "s/^trackerIssue:.*/trackerIssue: $ISSUE_NUM/" config.yaml
echo "  Set trackerIssue: $ISSUE_NUM"

# 8. Recreate necessary folders and files
echo "Recreating folders and files..."
mkdir -p workers
echo "  Created workers/"
mkdir -p messages
echo "  Created messages/"
mkdir -p workspace/athena workspace/apollo workspace/hermes
echo "  Created workspace/ with manager subdirs"
echo '{"cycleCount":0,"currentAgentIndex":0,"managersRun":[],"isPaused":false}' > state.json
echo "  Created state.json"
touch orchestrator.log
echo "  Created orchestrator.log"

echo ""
echo "=== Bootstrap complete ==="
echo "Tracker issue: $ISSUE_URL"
echo "To start fresh: ./run.sh"
