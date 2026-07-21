#!/usr/bin/env bash
# build-awd.sh — build the headless awd daemon for release.
#
# go:embed is relative to the embedding file's directory and cannot reach
# ../../frontend/dist, so a release build stages the frontend next to
# cmd/awd/main.go and compiles with -tags awd_embed for a single-file binary.
# Without staging, `go build ./cmd/awd` still works (assets load from disk via
# AW_WEB_ASSETS_DIR or a "dist" dir beside the binary).
#
# Usage:
#   scripts/build-awd.sh                 # current OS/arch, embedded frontend
#   GOOS=linux GOARCH=amd64 scripts/build-awd.sh
#   AWD_TRAY=1 scripts/build-awd.sh      # also build the systray companion tag
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DIST="frontend/dist"
STAGED="cmd/awd/dist"
OUT="${OUT:-build/bin/awd}"

if [[ ! -d "$DIST" ]]; then
  echo "frontend not built: run 'cd frontend && npm run build' first" >&2
  exit 1
fi

echo "==> staging frontend into $STAGED"
rm -rf "$STAGED"
cp -r "$DIST" "$STAGED"
trap 'rm -rf "$STAGED"' EXIT

TAGS="awd_embed"
if [[ "${AWD_TRAY:-0}" == "1" ]]; then
  TAGS="$TAGS awd_tray"
fi

echo "==> building $OUT (tags: $TAGS, GOOS=${GOOS:-$(go env GOOS)} GOARCH=${GOARCH:-$(go env GOARCH)})"
mkdir -p "$(dirname "$OUT")"
go build -tags "$TAGS" -o "$OUT" ./cmd/awd
echo "==> done: $OUT"
