#!/usr/bin/env bash
# Sign a built macOS .app with the stable local code-signing identity when it is
# installed, falling back to an ad-hoc signature when it is absent.
#
# macOS TCC permissions (Accessibility, Automation, Screen Recording, …) key off
# the app's codesign Designated Requirement. Signing every rebuild with the SAME
# identity keeps the DR constant, so permissions survive recompiles. An ad-hoc
# signature embeds a per-build cdhash, so every rebuild looks like a new app and
# macOS revokes/re-prompts.
#
# This identity name MUST match tools/buildgate/main.go's localSigningIdentity
# and scripts/create-local-signing-cert.sh's IDENTITY. Creating the identity is a
# one-time manual step: scripts/create-local-signing-cert.sh.
#
# Usage: sign-macos-app.sh <path-to.app>
# Sourced or run; tolerant by design — a signing hiccup never fails the build.

IDENTITY="aw-Local Code Signing"

# An Apple-issued "Apple Development" identity takes precedence when present:
# only Apple certificates carry a team ID, which keys Keychain partition IDs
# and TCC grants to the TEAM instead of the per-build binary hash — Touch ID
# and permissions then survive rebuilds with no re-prompt. Keep in sync with
# tools/buildgate/main.go resolveSigningIdentity.
apple_development_identity() {
  security find-identity -v -p codesigning 2>/dev/null \
    | grep -m1 -o '"Apple Development[^"]*"' | tr -d '"'
}

sign_macos_app() {
  local app="$1"
  if [ -z "$app" ]; then
    echo "usage: sign-macos-app.sh <path-to.app>" >&2
    return 0
  fi
  local apple_id
  apple_id="$(apple_development_identity)"
  if [ -n "$apple_id" ]; then
    codesign --force --deep -s "$apple_id" "$app" >/dev/null 2>&1 || true
    echo "   ↳ signed with Apple identity ($apple_id) — Keychain/TCC grants survive rebuilds"
  elif security find-identity -p codesigning | grep -q "$IDENTITY"; then
    codesign --force --deep -s "$IDENTITY" "$app" >/dev/null 2>&1 || true
    echo "   ↳ signed with stable identity ($IDENTITY) — TCC permissions persist"
  else
    codesign --force --deep -s - "$app" >/dev/null 2>&1 || true
    echo "   ↳ ad-hoc signature — run scripts/create-local-signing-cert.sh for stable TCC permissions"
  fi
}

# Allow running directly: sign-macos-app.sh <path-to.app>
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  sign_macos_app "$1"
fi
