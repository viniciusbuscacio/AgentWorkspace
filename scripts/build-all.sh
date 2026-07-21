#!/usr/bin/env bash
# Build BOTH macOS app bundles in one shot:
#   - build/bin/Agent Workspace.app      (production)
#   - build/bin/Agent Workspace-DEV.app  (dev — devunlock vault "1234", REST :9311)
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"

# A running instance holds a browser port and makes the build gate's
# TestChromeEndToEnd flaky — stop Agent Workspace first.
pkill -f "MacOS/Agent Workspace-DEV" 2>/dev/null || true
pkill -f "MacOS/Agent Workspace" 2>/dev/null || true
# Backward-compatible cleanup for old bundle names.
pkill -f "MacOS/aw-DEV" 2>/dev/null || true
pkill -f "MacOS/aw" 2>/dev/null || true
sleep 1

echo "==> [1/2] building production (Agent Workspace.app)…"
if ! wails build; then
  echo "✗ production build failed"
  exit 1
fi
PROD_APP="build/bin/Agent Workspace.app"
PROD_PL="$PROD_APP/Contents/Info.plist"
if [ -f "$PROD_PL" ]; then
  plutil -replace CFBundleName        -string "Agent Workspace"                 "$PROD_PL"
  plutil -replace CFBundleDisplayName -string "Agent Workspace"                 "$PROD_PL" 2>/dev/null || true
  plutil -replace CFBundleIdentifier  -string "net.buscacio.agent-workspace"    "$PROD_PL"
  # shellcheck source=scripts/sign-macos-app.sh
  source "$(dirname "$0")/sign-macos-app.sh"
  sign_macos_app "$PROD_APP"
  touch -m "$PROD_APP"
fi
echo "✅ production build/bin/Agent Workspace.app built"

echo "==> [2/2] building dev variant (Agent Workspace-DEV.app)…"
./scripts/build-dev.sh

echo ""
echo "🎉 Both apps are fresh:"
echo "   • build/bin/Agent Workspace.app      (production)"
echo "   • build/bin/Agent Workspace-DEV.app  (dev — vault 1234, REST :9311)"
