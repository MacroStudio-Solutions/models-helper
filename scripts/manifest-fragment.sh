#!/usr/bin/env bash

# Preparacao do fragmento de manifesto do runtime models-helper: baixa cada
# artefato da release publicada, calcula sha256 e tamanho do bytes baixado
# (releases do GitHub nao publicam checksum), confere o limite de 512 MB e
# emite o JSON com as chaves de plataforma canonicas prontas para colar no
# manifest das extensoes. As duas extensoes declaram a mesma versao.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="MacroStudio-Solutions/models-helper"
VERSION="${1:-$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")}"
TAG="v${VERSION}"
BASE_URL="https://github.com/$REPO/releases/download/$TAG"
VERIFY_DIR="$ROOT_DIR/dist/verify/$TAG"
OUT_FILE="$ROOT_DIR/dist/manifest-fragment-$TAG.json"
MAX_ARTIFACT_BYTES=$((512 * 1024 * 1024))

if ! printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
	printf 'Erro: versao (%s) nao e semver valido.\n' "$VERSION" >&2
	exit 1
fi

rm -rf "$VERIFY_DIR"
mkdir -p "$VERIFY_DIR"

fetch() {
	local key="$1"
	local artifact="$2"
	printf '%s' "$BASE_URL/$artifact" > "$VERIFY_DIR/$artifact.url"
	curl -fsSL --retry 3 -o "$VERIFY_DIR/$artifact" "$BASE_URL/$artifact"

	local sha size
	sha="$(sha256sum "$VERIFY_DIR/$artifact" | cut -d' ' -f1)"
	size="$(stat -c '%s' "$VERIFY_DIR/$artifact")"
	if [ "$size" -ge "$MAX_ARTIFACT_BYTES" ]; then
		printf 'Erro: artefato %s tem %d bytes, acima do limite de %d bytes imposto ao artefato de runtime.\n' "$artifact" "$size" "$MAX_ARTIFACT_BYTES" >&2
		exit 1
	fi

	local local_path="$ROOT_DIR/dist/release/$artifact"
	if [ -f "$local_path" ]; then
		local local_sha
		local_sha="$(sha256sum "$local_path" | cut -d' ' -f1)"
		if [ "$local_sha" != "$sha" ]; then
			printf 'Erro: sha256 do artefato publicado (%s) diverge do build local (%s) para %s.\n' "$sha" "$local_sha" "$artifact" >&2
			exit 1
		fi
	fi

	printf '%s\t%s\t%s bytes\t%s\n' "$key" "$sha" "$size" "$artifact"
	printf '%s\n' "$sha" > "$VERIFY_DIR/$artifact.sha256"
	printf '%s\n' "$size" > "$VERIFY_DIR/$artifact.size"
}

LINUX_X64_ARTIFACT="models-helper-v${VERSION}-linux-x64.tar.gz"
LINUX_ARM64_ARTIFACT="models-helper-v${VERSION}-linux-arm64.tar.gz"
WINDOWS_X64_ARTIFACT="models-helper-v${VERSION}-windows-x64.zip"

fetch linux-x64-gnu "$LINUX_X64_ARTIFACT"
fetch linux-arm64-gnu "$LINUX_ARM64_ARTIFACT"
fetch win32-x64 "$WINDOWS_X64_ARTIFACT"

LINUX_X64_SHA="$(cat "$VERIFY_DIR/$LINUX_X64_ARTIFACT.sha256")"
LINUX_ARM64_SHA="$(cat "$VERIFY_DIR/$LINUX_ARM64_ARTIFACT.sha256")"
WINDOWS_X64_SHA="$(cat "$VERIFY_DIR/$WINDOWS_X64_ARTIFACT.sha256")"
LINUX_X64_SIZE="$(cat "$VERIFY_DIR/$LINUX_X64_ARTIFACT.size")"
LINUX_ARM64_SIZE="$(cat "$VERIFY_DIR/$LINUX_ARM64_ARTIFACT.size")"
WINDOWS_X64_SIZE="$(cat "$VERIFY_DIR/$WINDOWS_X64_ARTIFACT.size")"

cat > "$OUT_FILE" <<EOF
{
  "id": "models-helper",
  "version": "$VERSION",
  "platforms": {
    "linux-x64-gnu": {
      "ref": "$BASE_URL/$LINUX_X64_ARTIFACT",
      "sha256": "$LINUX_X64_SHA",
      "sizeBytes": $LINUX_X64_SIZE,
      "format": "tar.gz",
      "entrypoint": "models-helper-v${VERSION}-linux-x64/models-helper"
    },
    "linux-arm64-gnu": {
      "ref": "$BASE_URL/$LINUX_ARM64_ARTIFACT",
      "sha256": "$LINUX_ARM64_SHA",
      "sizeBytes": $LINUX_ARM64_SIZE,
      "format": "tar.gz",
      "entrypoint": "models-helper-v${VERSION}-linux-arm64/models-helper"
    },
    "win32-x64": {
      "ref": "$BASE_URL/$WINDOWS_X64_ARTIFACT",
      "sha256": "$WINDOWS_X64_SHA",
      "sizeBytes": $WINDOWS_X64_SIZE,
      "format": "zip",
      "entrypoint": "models-helper.exe",
      "note": "declarado-nao-validado: compilado por cross-compile, sem maquina Windows real para validar"
    }
  }
}
EOF

printf 'Fragmento de manifesto pronto: %s\n' "$OUT_FILE"
printf 'Declarar a versao %s identicamente nas extensoes local-models e local-transcription.\n' "$VERSION"
