#!/usr/bin/env bash
# Launch the DEV app (Agent Workspace-DEV.app) with the dev environment:
# auto-unlock the throwaway vault "1234" and isolate all data under ~/aw-dev-data
# (separate config.json / profiles.json / vaults from production). REST serves on :9311.
set -euo pipefail
cd "$(dirname "$0")/.."

DEV="build/bin/Agent Workspace-DEV.app/Contents/MacOS/Agent Workspace-DEV"
[ -x "$DEV" ] || { echo "build first: ./scripts/build-dev.sh"; exit 1; }

pkill -f "MacOS/Agent Workspace-DEV" 2>/dev/null || true
pkill -f "MacOS/aw-DEV" 2>/dev/null || true
sleep 1

# Launch through LaunchServices (open), NOT by exec'ing the binary: a child
# of the terminal gets its TCC permissions (microphone, ...) attributed to the
# TERMINAL as the responsible process — silent denials, no prompt. Via open,
# the app is its own responsible process and macOS prompts normally.
open -n "build/bin/Agent Workspace-DEV.app" \
  --env aw_DEV_UNLOCK=1 --env aw_DATA_DIR="$HOME/aw-dev-data"
echo "✅ Agent Workspace-DEV launched (via open)  data=$HOME/aw-dev-data  REST :9311"
echo "   app log: ~/aw-dev-data (Logs module)   unlock: ~/aw-dev-data/dev-unlock.log"
