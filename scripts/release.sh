#!/bin/sh
# Release do aw a partir da máquina local (decisão de scoping: sem CI, o
# buildgate daqui já assina com a identidade Apple Development correta, o que
# mantém Touch ID/Keychain válidos no app publicado).
#
#   scripts/release.sh v0.1.0
#
# Faz, nesta ordem: valida o tag e o estado do repo -> buildgate completo com
# AW_VERSION estampado -> zipa o .app -> checksums.txt (SHA-256, obrigatório
# para o futuro updater) -> cria e sobe o tag -> gh release create com os
# assets. Qualquer falha aborta antes de publicar.
set -eu

TAG="${1:-}"
case "$TAG" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "uso: scripts/release.sh vX.Y.Z" >&2; exit 1 ;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"

[ "$(git branch --show-current)" = "main" ] || { echo "release só a partir de main" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "working tree sujo — commit antes de lançar" >&2; exit 1; }
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "tag $TAG já existe" >&2; exit 1
fi
git fetch origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || { echo "main local difere de origin/main — sincronize antes" >&2; exit 1; }

echo "==> Buildgate ($TAG)"
AW_VERSION="$TAG" go run ./tools/buildgate

APP="build/bin/Agent Workspace.app"
[ -d "$APP" ] || { echo "build não produziu $APP" >&2; exit 1; }

# Confere que o binário realmente carrega a versão estampada antes de publicar.
STAMPED="$("$APP/Contents/MacOS/Agent Workspace" --version 2>/dev/null || true)"
if [ "$STAMPED" != "$TAG" ]; then
  echo "binário reporta '$STAMPED', esperado '$TAG' — abortando" >&2; exit 1
fi

ARCH="$(uname -m)"   # arm64 na maquina de release
ASSET="agent-workspace-$TAG-macos-$ARCH.zip"
DIST="build/release/$TAG"
rm -rf "$DIST" && mkdir -p "$DIST"

echo "==> Empacotando $ASSET"
ditto -c -k --keepParent "$APP" "$DIST/$ASSET"
(cd "$DIST" && shasum -a 256 "$ASSET" > checksums.txt)

echo "==> Tag + release no GitHub"
git tag -a "$TAG" -m "aw $TAG"
git push origin "$TAG"
gh release create "$TAG" \
  --title "aw $TAG" \
  --generate-notes \
  "$DIST/$ASSET" "$DIST/checksums.txt"

echo "✅ release $TAG publicada"
