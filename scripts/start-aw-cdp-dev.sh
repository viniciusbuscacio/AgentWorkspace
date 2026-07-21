#!/bin/sh
# Starts `wails dev` plus a headless Chrome with a fixed CDP port so
# scripts/aw-cdp-smoke.mjs can drive the React frontend against the real Go
# backend. Uses a throwaway aw_DATA_DIR so the smoke never touches a real
# vault. macOS equivalent of start-aw-cdp-dev.ps1.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"

aw_DATA_DIR="${aw_SMOKE_DATA_DIR:-$(mktemp -d /tmp/aw-smoke-data.XXXXXX)}"
export aw_DATA_DIR
CDP_PORT="${aw_CDP_PORT:-9225}"
CHROME="${aw_CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
CHROME_PROFILE="$(mktemp -d /tmp/aw-smoke-chrome.XXXXXX)"
# aw_CHROME_HEADLESS=0 opens a visible Chrome window to watch the smoke run.
HEADLESS_FLAG="--headless=new"
if [ "${aw_CHROME_HEADLESS:-1}" = "0" ]; then
  HEADLESS_FLAG=""
fi

echo "aw_DATA_DIR=$aw_DATA_DIR"
echo "CDP=http://127.0.0.1:$CDP_PORT"

cleanup() {
  kill "$WAILS_PID" "$CHROME_PID" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

cd "$ROOT"
wails dev &
WAILS_PID=$!

# shellcheck disable=SC2086
"$CHROME" \
  $HEADLESS_FLAG \
  --remote-debugging-port="$CDP_PORT" \
  --user-data-dir="$CHROME_PROFILE" \
  --no-first-run \
  --use-fake-ui-for-media-stream \
  --use-fake-device-for-media-stream \
  about:blank &
CHROME_PID=$!

wait "$WAILS_PID"
