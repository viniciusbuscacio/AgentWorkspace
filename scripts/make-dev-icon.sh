#!/usr/bin/env bash
# Generate the DEV app icon: overlays the word "DEV" (white, bold) below the
# glasses on build/appicon.png and emits build/appicon-dev.icns. Pure macOS
# tooling (swift + sips + iconutil) — no ImageMagick/PIL needed.
#
# build-dev.sh calls this and injects the .icns into the DEV bundle before
# signing so the dock/Finder icon is clearly distinct from production.
set -euo pipefail
cd "$(dirname "$0")/.."

SRC="build/appicon.png"
OUT_PNG="build/appicon-dev.png"
OUT_ICNS="build/appicon-dev.icns"

[ -f "$SRC" ] || { echo "✗ $SRC not found" >&2; exit 1; }

echo "==> rendering DEV badge onto icon…"
swift "scripts/make-dev-icon.swift" "$SRC" "$OUT_PNG"

WORK="$(mktemp -d)/dev.iconset"
mkdir -p "$WORK"
# All the sizes iconutil expects for a macOS .icns.
sips -z 16 16     "$OUT_PNG" --out "$WORK/icon_16x16.png"      >/dev/null
sips -z 32 32     "$OUT_PNG" --out "$WORK/icon_16x16@2x.png"   >/dev/null
sips -z 32 32     "$OUT_PNG" --out "$WORK/icon_32x32.png"      >/dev/null
sips -z 64 64     "$OUT_PNG" --out "$WORK/icon_32x32@2x.png"   >/dev/null
sips -z 128 128   "$OUT_PNG" --out "$WORK/icon_128x128.png"    >/dev/null
sips -z 256 256   "$OUT_PNG" --out "$WORK/icon_128x128@2x.png" >/dev/null
sips -z 256 256   "$OUT_PNG" --out "$WORK/icon_256x256.png"    >/dev/null
sips -z 512 512   "$OUT_PNG" --out "$WORK/icon_256x256@2x.png" >/dev/null
sips -z 512 512   "$OUT_PNG" --out "$WORK/icon_512x512.png"    >/dev/null
cp "$OUT_PNG" "$WORK/icon_512x512@2x.png"

iconutil -c icns "$WORK" -o "$OUT_ICNS"
echo "✅ wrote $OUT_ICNS"
