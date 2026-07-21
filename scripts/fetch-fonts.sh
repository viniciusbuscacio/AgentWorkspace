#!/usr/bin/env bash
# fetch-fonts.sh — (re)download the bundled UI fonts from Google Fonts.
#
# Downloads the latin-subset .woff2 for weights 400 (normal) and 700 (bold) of
# every family in the FONTS list into frontend/src/assets/fonts/<id>-<w>.woff2.
# These files are committed to the repo and wired up in:
#   - frontend/src/theme/fonts.css        (@font-face declarations)
#   - frontend/src/lib/app-font.ts        (APP_FONT_OPTIONS)
#   - internal/infrastructure/tools/aw_app.go (awFontFamilies allowlist)
#
# To ADD a font: add an "id|Family Name" line below, run this script, then follow
# docs/FONTS.md to register it in the three files above. All families here are
# OFL-1.1 or Apache-2.0 licensed (free to redistribute).
set -euo pipefail
cd "$(dirname "$0")/.."

UA='Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36'
OUT="frontend/src/assets/fonts"
mkdir -p "$OUT"

# id | Google Fonts family name
FONTS=$(cat <<'EOF'
inter|Inter
roboto|Roboto
open-sans|Open Sans
lato|Lato
montserrat|Montserrat
poppins|Poppins
nunito|Nunito
work-sans|Work Sans
dm-sans|DM Sans
manrope|Manrope
rubik|Rubik
merriweather|Merriweather
lora|Lora
playfair-display|Playfair Display
jetbrains-mono|JetBrains Mono
fira-code|Fira Code
ibm-plex-mono|IBM Plex Mono
arimo|Arimo
tinos|Tinos
cousine|Cousine
gelasio|Gelasio
comic-neue|Comic Neue
inconsolata|Inconsolata
libre-franklin|Libre Franklin
jost|Jost
libre-baskerville|Libre Baskerville
EOF
)

fetch_weight () {
  local id="$1" fam="$2" wght="$3"
  local q="${fam// /+}" css url sz
  css=$(curl -fsS -A "$UA" "https://fonts.googleapis.com/css2?family=${q}:wght@${wght}&display=swap")
  # Isolate the exact "/* latin */" block (BSD awk has no 3-arg match), grab its woff2 URL.
  url=$(printf '%s\n' "$css" \
    | awk '/\/\* latin \*\//{f=1;next} /\/\* /{f=0} f' \
    | grep -oE 'https://[^)]+\.woff2' | head -1)
  if [ -z "$url" ]; then echo "  MISS w$wght for $fam"; return 1; fi
  curl -fsS -o "$OUT/${id}-${wght}.woff2" "$url"
  sz=$(stat -f%z "$OUT/${id}-${wght}.woff2" 2>/dev/null || stat -c%s "$OUT/${id}-${wght}.woff2")
  echo "  ok w$wght -> ${id}-${wght}.woff2 (${sz} bytes)"
}

while IFS='|' read -r id fam; do
  [ -z "$id" ] && continue
  echo "== $fam ($id) =="
  fetch_weight "$id" "$fam" 400
  fetch_weight "$id" "$fam" 700
done <<< "$FONTS"

echo "=== done: $(ls "$OUT"/*.woff2 | wc -l | tr -d ' ') files, $(du -sh "$OUT" | cut -f1) total ==="
