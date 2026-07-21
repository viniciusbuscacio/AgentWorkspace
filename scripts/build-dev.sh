#!/usr/bin/env bash
# Build the DEV variant as a SEPARATE macOS app bundle:
#   build/bin/Agent Workspace-DEV.app
#
# Wails writes the production bundle name from wails.json. This script moves any
# existing production bundle aside, builds with the devunlock tag, renames the
# fresh bundle to the DEV name, gives it a distinct executable/bundle id, then
# restores production untouched.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"

BIN="build/bin"
PROD="$BIN/Agent Workspace.app"
DEV="$BIN/Agent Workspace-DEV.app"
BACKUP="$(mktemp -d)/Agent Workspace.app"
PROD_EXE="Agent Workspace"
DEV_EXE="Agent Workspace-DEV"

# A running instance holds a browser port and makes TestChromeEndToEnd flaky.
pkill -f "MacOS/Agent Workspace-DEV" 2>/dev/null || true
pkill -f "MacOS/Agent Workspace" 2>/dev/null || true
# Backward-compatible cleanup for old bundle names.
pkill -f "MacOS/aw-DEV" 2>/dev/null || true
pkill -f "MacOS/aw" 2>/dev/null || true
sleep 1

[ -d "$PROD" ] && mv "$PROD" "$BACKUP"

echo "==> building dev (devunlock)…"
if ! wails build -tags devunlock; then
  echo "✗ dev build failed"
  [ -d "$BACKUP" ] && mv "$BACKUP" "$PROD"
  exit 1
fi

rm -rf "$DEV"
mv "$PROD" "$DEV"
mv "$DEV/Contents/MacOS/$PROD_EXE" "$DEV/Contents/MacOS/$DEV_EXE"
PL="$DEV/Contents/Info.plist"
plutil -replace CFBundleName       -string "Agent Workspace Dev"                     "$PL"
plutil -replace CFBundleDisplayName -string "Agent Workspace Dev"                    "$PL" 2>/dev/null || true
plutil -replace CFBundleExecutable -string "$DEV_EXE"                                "$PL"
plutil -replace CFBundleIdentifier -string "net.buscacio.agent-workspace.dev"        "$PL"
# Distinct dock/Finder icon (glasses + "DEV") so the dev build is unmistakable.
# Pre-generated and committed at build/appicon-dev.icns — just copy it in
# (before signing, so the signature seals it). To regenerate after the base
# icon changes, run scripts/make-dev-icon.sh.
if [ -f "build/appicon-dev.icns" ]; then
  cp "build/appicon-dev.icns" "$DEV/Contents/Resources/iconfile.icns"
  echo "✅ dev icon applied"
else
  echo "⚠️  build/appicon-dev.icns missing — run scripts/make-dev-icon.sh; keeping default icon"
fi
# shellcheck source=scripts/sign-macos-app.sh
source "$(dirname "$0")/sign-macos-app.sh"
sign_macos_app "$DEV"
touch -m "$DEV"

[ -d "$BACKUP" ] && mv "$BACKUP" "$PROD"
[ -d "$PROD" ] && touch -m "$PROD"

echo "✅ built $DEV  (auto-unlock vault 1234, REST :9311)"
if [ -d "$PROD" ]; then
  echo "✅ production $PROD preserved"
else
  echo "ℹ️  no production Agent Workspace.app yet — run 'wails build' to create one"
fi
