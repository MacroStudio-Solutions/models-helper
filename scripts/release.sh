#!/usr/bin/env bash

# Publicacao do models-helper por release do GitHub — o mesmo canal usado pelo
# wa-control, sem esteira nova. Valida semver, exige arvore limpa, compila as
# tres plataformas, confere o limite de 512 MB por artefato, cria a tag e
# publica a release com os tres artefatos anexados.
#
# A soma de verificacao nao e publicada aqui: releases do GitHub nao publicam
# checksum, entao cada artefato e baixado e hasheado na preparacao do manifesto
# (scripts/manifest-fragment.sh).

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"
MAX_ARTIFACT_BYTES=$((512 * 1024 * 1024))
TAG="v${VERSION}"

if ! printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
	printf 'Erro: VERSION (%s) nao e semver valido.\n' "$VERSION" >&2
	exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
	printf 'Erro: gh nao encontrado no PATH.\n' >&2
	exit 1
fi

if [ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]; then
	printf 'Erro: arvore de trabalho nao limpa — faca commit antes de publicar.\n' >&2
	git -C "$ROOT_DIR" status --short >&2
	exit 1
fi

if git -C "$ROOT_DIR" rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
	printf 'Erro: tag %s ja existe.\n' "$TAG" >&2
	exit 1
fi

bash "$ROOT_DIR/scripts/build-release.sh" all

ARTIFACTS=(
	"models-helper-v${VERSION}-linux-x64.tar.gz"
	"models-helper-v${VERSION}-linux-arm64.tar.gz"
	"models-helper-v${VERSION}-windows-x64.zip"
)

for artifact in "${ARTIFACTS[@]}"; do
	path="$ROOT_DIR/dist/release/$artifact"
	if [ ! -f "$path" ]; then
		printf 'Erro: artefato esperado ausente: %s\n' "$path" >&2
		exit 1
	fi
	size=$(stat -c '%s' "$path")
	if [ "$size" -ge "$MAX_ARTIFACT_BYTES" ]; then
		printf 'Erro: artefato %s tem %d bytes, acima do limite de %d bytes imposto ao artefato de runtime.\n' "$artifact" "$size" "$MAX_ARTIFACT_BYTES" >&2
		exit 1
	fi
	printf '%s: %d bytes (limite %d)\n' "$artifact" "$size" "$MAX_ARTIFACT_BYTES"
done

NOTES_FILE="$ROOT_DIR/dist/release/notes-$TAG.md"
mkdir -p "$(dirname "$NOTES_FILE")"
cat > "$NOTES_FILE" <<EOF
# models-helper $TAG

Ajudante de modelos locais do Studio: binário próprio em Go, distribuído como
artefato de runtime pinado por sha256 e declarado pelas extensões
\`local-models\` e \`local-transcription\` nesta mesma versão.

## Artefatos

| Plataforma (chave canônica) | Artefato | Formato | Entry point |
|---|---|---|---|
| \`linux-x64-gnu\` | \`models-helper-v${VERSION}-linux-x64.tar.gz\` | tar.gz | \`models-helper-v${VERSION}-linux-x64/models-helper\` |
| \`linux-arm64-gnu\` | \`models-helper-v${VERSION}-linux-arm64.tar.gz\` | tar.gz | \`models-helper-v${VERSION}-linux-arm64/models-helper\` |
| \`win32-x64\` | \`models-helper-v${VERSION}-windows-x64.zip\` | zip | \`models-helper.exe\` |

## Validação declarada

- Linux x64: compilado e validado em máquina real (build, testes e smoke dos
  dez comandos).
- Linux arm64: compilado por compilação cruzada, sem execução em máquina real
  nesta release.
- Windows x64: **declarado-não-validado** — compilado por compilação cruzada,
  sem máquina Windows real para validar execução. A extensão que declara este
  artefato registra o mesmo estado em vez de afirmar suporte validado.

Releases do GitHub não publicam soma de verificação: o sha256 e o tamanho de
cada artefato são calculados baixando o artefato publicado, na preparação do
manifesto da extensão (chaves canônicas \`linux-x64-gnu\`, \`linux-arm64-gnu\`
e \`win32-x64\`).
EOF

BRANCH="$(git -C "$ROOT_DIR" rev-parse --abbrev-ref HEAD)"
git -C "$ROOT_DIR" push origin "$BRANCH"
git -C "$ROOT_DIR" tag -a "$TAG" -m "models-helper $TAG"

gh release create "$TAG" \
	--repo MacroStudio-Solutions/models-helper \
	--title "models-helper $TAG" \
	--notes-file "$NOTES_FILE" \
	"$ROOT_DIR/dist/release/${ARTIFACTS[0]}" \
	"$ROOT_DIR/dist/release/${ARTIFACTS[1]}" \
	"$ROOT_DIR/dist/release/${ARTIFACTS[2]}"

printf 'Release %s publicada. Prepare o fragmento de manifesto com: bash scripts/manifest-fragment.sh %s\n' "$TAG" "$VERSION"
