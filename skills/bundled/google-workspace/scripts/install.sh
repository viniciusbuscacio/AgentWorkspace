#!/usr/bin/env bash
# Installs the Google Workspace CLI (npm package @googleworkspace/cli, binary
# `gws`) that Agent Workspace's gws.* actions shell out to.
#
# Idempotent and sudo-free: safe for the in-app agent to run (with the user's
# OK) or for the user to run in a terminal. macOS/Linux; on Windows run
# `npm install -g @googleworkspace/cli` directly.
set -u

# newest_match <glob...> — print the most recently modified existing file.
newest_match() {
  local best="" path
  for path in "$@"; do
    [ -x "$path" ] || continue
    if [ -z "$best" ] || [ "$path" -nt "$best" ]; then best="$path"; fi
  done
  printf '%s' "$best"
}

find_gws() {
  if command -v gws >/dev/null 2>&1; then command -v gws; return; fi
  newest_match /opt/homebrew/bin/gws /usr/local/bin/gws "$HOME"/.nvm/versions/node/*/bin/gws
}

find_npm() {
  if command -v npm >/dev/null 2>&1; then command -v npm; return; fi
  newest_match /opt/homebrew/bin/npm /usr/local/bin/npm "$HOME"/.nvm/versions/node/*/bin/npm
}

existing="$(find_gws)"
if [ -n "$existing" ]; then
  echo "OK: gws is already installed at: $existing"
  "$existing" --version 2>/dev/null | head -1 || true
  echo "Next: authenticate if needed (gws auth setup --login once; automates the OAuth client, needs gcloud)"
  echo "and verify inside Agent Workspace with the aw action gws.status."
  exit 0
fi

npm_bin="$(find_npm)"
if [ -z "$npm_bin" ]; then
  echo "ERROR: Node.js/npm not found (checked PATH, Homebrew and nvm locations)." >&2
  echo "Install Node.js first — https://nodejs.org or via nvm — then re-run this script." >&2
  exit 1
fi

echo "Installing @googleworkspace/cli with: $npm_bin install -g @googleworkspace/cli"
if ! "$npm_bin" install -g @googleworkspace/cli; then
  echo "ERROR: npm install failed — see the output above." >&2
  exit 1
fi

installed="$(find_gws)"
if [ -z "$installed" ]; then
  # npm succeeded but the bin dir is not among the known locations.
  installed="$(dirname "$npm_bin")/gws"
fi
echo "OK: gws installed at: $installed"
"$installed" --version 2>/dev/null | head -1 || true
echo
echo "Next steps:"
echo "  1. gws auth setup --login   (first time only; automates the GCP project + OAuth client, needs the gcloud CLI)"
echo "  2. Verify inside Agent Workspace with the aw action gws.status."
