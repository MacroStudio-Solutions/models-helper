#!/usr/bin/env bash

# Compilacao cruzada do models-helper para as tres plataformas declaradas nos
# manifests das extensoes (linux-x64-gnu, linux-arm64-gnu, win32-x64), em um
# unico comando. Segue o formato de distribuicao do wa-control: artefato
# versionado em dist/release, tar.gz com pasta de topo no Linux e zip plano no
# Windows, publicado por release do GitHub.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist/release"
TARGET="${1:-all}"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

if ! printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
	printf 'Erro: VERSION (%s) nao e semver valido.\n' "$VERSION" >&2
	exit 1
fi

GO_BIN="$(command -v go || true)"
if [ -z "$GO_BIN" ] && [ -x "$HOME/.local/go/bin/go" ]; then
	GO_BIN="$HOME/.local/go/bin/go"
fi
if [ -z "$GO_BIN" ]; then
	printf 'Erro: go nao encontrado no PATH (nem em $HOME/.local/go/bin).\n' >&2
	exit 1
fi

LDFLAGS="-s -w"
GOFLAGS="-trimpath"

build_target() {
	local goos="$1"
	local goarch="$2"
	local platform="$3"
	local ext="$4"
	local slug="models-helper-v${VERSION}-${platform}"
	local target_dir="$DIST_DIR/$slug"
	local bin="models-helper${ext}"

	rm -rf "$target_dir" "$DIST_DIR/$slug.tar.gz" "$DIST_DIR/$slug.zip"
	mkdir -p "$target_dir"

	(
		cd "$ROOT_DIR"
		CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" "$GO_BIN" build \
			$GOFLAGS \
			-ldflags "$LDFLAGS" \
			-o "$target_dir/$bin" \
			./
	)

	if [ "$ext" = ".exe" ]; then
		(
			cd "$target_dir"
			python3 -m zipfile -c "$DIST_DIR/$slug.zip" "$bin"
		)
		printf 'Windows x64 (%s) gerado em %s\n' "$goarch" "$DIST_DIR/$slug.zip"
	else
		tar -C "$DIST_DIR" -czf "$DIST_DIR/$slug.tar.gz" "$slug"
		printf '%s gerado em %s\n' "$platform" "$DIST_DIR/$slug.tar.gz"
	fi
}

mkdir -p "$DIST_DIR"

case "$TARGET" in
	linux-x64)
		build_target linux amd64 linux-x64 ""
		;;
	linux-arm64)
		build_target linux arm64 linux-arm64 ""
		;;
	windows-x64)
		build_target windows amd64 windows-x64 ".exe"
		;;
	all)
		build_target linux amd64 linux-x64 ""
		build_target linux arm64 linux-arm64 ""
		build_target windows amd64 windows-x64 ".exe"
		;;
	*)
		printf 'Uso: %s [linux-x64|linux-arm64|windows-x64|all]\n' "${BASH_SOURCE[0]}" >&2
		exit 1
		;;
esac
