#!/usr/bin/env bash
# Creates a stable, self-signed code-signing identity ("aw-Local Code Signing")
# in the login keychain. Signing every build with the SAME identity keeps the
# app's codesign Designated Requirement constant (identifier + this cert),
# instead of the per-build cdhash an ad-hoc signature produces. That is what
# makes macOS TCC permissions (Accessibility, Automation, Screen Recording)
# survive recompiles instead of being revoked every time.
#
# Run once. After it, `go run ./tools/buildgate` (and the gated `wails build`)
# auto-signs build/bin/Agent Workspace.app with this identity.
#
# Note: the cert is self-signed, so it is "untrusted by policy" (Gatekeeper will
# not bless it) — that is fine for a locally built/launched dev app and does not
# affect TCC stability, which keys off the Designated Requirement.
set -euo pipefail

IDENTITY="aw-Local Code Signing"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if security find-identity -p codesigning | grep -q "$IDENTITY"; then
  echo "Identity \"$IDENTITY\" already present — nothing to do."
  exit 0
fi

echo "==> Generating self-signed code-signing certificate"
cat > "$WORK/cert.conf" <<'EOF'
[ req ]
distinguished_name = dn
prompt = no
x509_extensions = codesign
[ dn ]
CN = aw-Local Code Signing
[ codesign ]
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
basicConstraints = critical, CA:false
EOF

openssl req -x509 -newkey rsa:2048 -keyout "$WORK/aw.key" -out "$WORK/aw.crt" \
  -days 3650 -nodes -config "$WORK/cert.conf" >/dev/null 2>&1

# Legacy PBE/MAC so the macOS `security` tool can import the PKCS#12.
openssl pkcs12 -export -inkey "$WORK/aw.key" -in "$WORK/aw.crt" -out "$WORK/aw.p12" \
  -name "$IDENTITY" -passout pass:awlocal \
  -legacy -certpbe PBE-SHA1-3DES -keypbe PBE-SHA1-3DES -macalg sha1 >/dev/null 2>&1

echo "==> Importing into login keychain"
security import "$WORK/aw.p12" -k "$KEYCHAIN" -P "awlocal" \
  -T /usr/bin/codesign -T /usr/bin/security

# Let codesign use the key without an interactive keychain prompt each build.
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "" "$KEYCHAIN" >/dev/null 2>&1 || true

echo "==> Done. Identity installed:"
security find-identity -p codesigning | grep "$IDENTITY" || true
echo
echo "Next build (go run ./tools/buildgate or wails build via the gate) will sign with it."
echo "macOS will ask for permissions ONE more time (new identity), then keep them across builds."
