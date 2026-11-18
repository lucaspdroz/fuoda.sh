#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Uso: ./utils/release-macos.sh <versao>"
  echo "Exemplo: ./utils/release-macos.sh v0.0.3"
  exit 1
fi

VERSION="$1"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DIST_DIR="$ROOT_DIR/dist"
mkdir -p "$DIST_DIR"

LDFLAGS="-s -w -X main.Version=${VERSION}"

ARM_BIN="fuoda.sh-mac-arm64"
INTEL_BIN="fuoda.sh-mac-amd64"

echo "▶ Compilando fuoda.sh (${VERSION}) para macOS..."

# 1. Apple Silicon (M1/M2/M3/M4)
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
  go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/$ARM_BIN" ./cmd/fuoda.sh
codesign -s - -f "$DIST_DIR/$ARM_BIN"
echo "✔ Gerado: dist/$ARM_BIN"

# 2. Intel (x86_64)
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
  go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/$INTEL_BIN" ./cmd/fuoda.sh
codesign -s - -f "$DIST_DIR/$INTEL_BIN"
echo "✔ Gerado: dist/$INTEL_BIN"

# Verifica se o gh CLI está instalado e autenticado
if ! command -v gh >/dev/null 2>&1; then
  echo "✖ GitHub CLI ('gh') não instalado. Instale com: brew install gh && gh auth login"
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "✖ GitHub CLI não autenticado. Rode: gh auth login"
  exit 1
fi

RELEASE_TAG="$VERSION"
if [[ ! "$RELEASE_TAG" =~ ^v ]] && gh release view "v${RELEASE_TAG}" >/dev/null 2>&1; then
  RELEASE_TAG="v${RELEASE_TAG}"
fi

# Se a Release ainda não existir, aguarda o workflow do GitHub Actions terminar
if ! gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
  echo "⏳ Aguardando o GitHub Actions criar a Release ${RELEASE_TAG}..."
  sleep 5
  RUN_ID=$(gh run list --branch "$RELEASE_TAG" --limit 1 --json databaseId --jq '.[0].databaseId' 2>/dev/null || true)
  if [ -n "$RUN_ID" ]; then
    gh run watch "$RUN_ID" --exit-status
  else
    # Fallback: tenta por até 90 segundos caso a API ainda esteja registrando o run
    for i in {1..18}; do
      if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
        break
      fi
      sleep 5
    done
  fi
fi

if ! gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
  echo "✖ Release '${RELEASE_TAG}' não foi criada pelo Actions. Verifique a aba Actions no GitHub."
  exit 1
fi

echo "▶ Enviando binários macOS para a Release ${RELEASE_TAG}..."
gh release upload "$RELEASE_TAG" \
  "$DIST_DIR/$ARM_BIN" \
  "$DIST_DIR/$INTEL_BIN" \
  --clobber

echo "✔ Binários publicados com sucesso na Release ${RELEASE_TAG}!"