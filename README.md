# models-helper

Ajudante de modelos locais do Studio: binário próprio em Go, distribuído como
runtime de extensão pinado por sha256 (o mesmo padrão de distribuição já usado
pela extensão `wa-control`). É a única fonte de dados sobre máquina, catálogo,
inventário e download de modelos — nenhum conhecimento de modelo, quantização ou
memória de vídeo entra no núcleo da plataforma.

A saída é sempre um único objeto JSON versionado em stdout:

```json
{ "schemaVersion": 1, "ok": true, "data": { } }
{ "schemaVersion": 1, "ok": false, "error": { "code": "NOME_DO_ERRO", "message": "..." } }
```

Falha devolve código de saída não zero com erro nomeado. A exceção deliberada é
`status --profile local-models`: a falha de resolução do runtime do llama vira
campo de erro dentro do objeto (`runtime.error`) e o código de saída permanece
zero, para o painel renderizar o alerta de saúde em vez de cair no estado de
falha.

## Superfície de comandos

| Comando | Devolve | Observação |
|---|---|---|
| `machine` | `TMachineProfile` | Memória total/disponível, núcleos, GPU, VRAM e caminho Vulkan com motivo quando ausente |
| `catalog list --limit <n>` | `TCatalogEntry[]` | Modelos por popularidade da API pública, com cache curto |
| `catalog search <termo>` | `TCatalogEntry[]` | Busca por palavra-chave na mesma API |
| `catalog versions <repo>` | `TCatalogEntry[]` | Variantes com peso real por versão |
| `installed --dir <caminho>` | `TInstalledModel[]` | Inventário local com veredito já calculado |
| `download start --repo <id> --file <arquivo> --dest <caminho>` | `TDownloadJob` | Retorna imediatamente com o identificador |
| `download status [--job <id>] [--dest <caminho>]` | `TDownloadJob[]` | Bytes, total e estado explícitos |
| `download cancel --job <id>` | `TDownloadJob` | Marca o cancelamento; o transferidor remove o temporário |
| `remove --path <caminho>` | `{ removed: boolean }` | Recusa caminho fora do diretório de modelos |
| `status --profile local-models` | `TLocalModelsStatus` | Leitura composta, fonte única do painel |

O contrato de saída é normativo no SDS
(`engineering/runtime-multiprograma-modelos/sds-runtime-multiprograma-modelos`,
bloco "Modelo de Dados") e espelhado pelas structs Go `THelperEnvelope`,
`TMachineProfile`, `TModelFit`, `TCatalogEntry`, `TInstalledModel`,
`TDownloadJob` e `TLocalModelsStatus` em `internal/contract`. Campos novos
entram como aditivos (`catalogError` é o primeiro); nenhum campo existente é
renomeado.

## Regras de comportamento

- **Veredito pronto no dado**: `fitOk`, `fitTight`, `fitGpu` e `requiredBytes`
  chegam calculados em todo item de catálogo e de inventário; o consumidor não
  compara números. A fórmula replica a do painel original: peso × 1,2 + margem
  de KV-cache de 1,5 GiB contra RAM disponível + VRAM.
- **Download honesto**: escrita em arquivo temporário oculto no próprio
  diretório de destino e promoção do nome final exclusivamente por renomeação —
  nenhum leitor observa arquivo parcial com nome definitivo. O estado é sempre
  exatamente um de `running`, `completed`, `cancelled` ou `failed`, com bytes
  recebidos e totais reais lidos do arquivo lateral, nunca inferidos por data
  de modificação.
- **Cancelamento**: `download cancel` grava um marcador lateral; o transferidor
  observa, remove o temporário e grava `cancelled`. Pid morto com estado
  `running` é promovido a `failed` com mensagem na próxima leitura — nunca por
  janela de tempo sem escrita.
- **Leitura composta em ordem inventário-depois-estado**: dentro da mesma
  invocação, o inventário de `.gguf` é montado antes do estado dos trabalhos;
  um trabalho `completed` cujo arquivo final já apareceu no inventário é tratado
  como simplesmente instalado (`download: null`), sem item duplicado e sem
  barra presa em 100%.
- **Rede só no catálogo**: o ajudante é o único componente que fala HTTP com a
  API pública de modelos, sem chave (listagem por popularidade + árvore por
  modelo). Cache de 5 minutos em arquivo com carimbo de tempo; falha remota com
  cópia local serve a cópia; o inventário local nunca depende da rede.
- **Sem estado entre invocações**, exceto o cache curto do catálogo: tudo é
  derivado do disco e da rede a cada chamada.
- **Recomendação de variante**: máquina sem driver Vulkan adequado cai para a
  variante de processador com motivo declarado; plataforma sem artefato de GPU
  declarado (Windows ARM) nunca recebe a variante de GPU.
- **Confinamento**: `download start` e `remove` recusam caminhos fora de
  `~/.studio/models` (incluindo escapes por link simbólico). Nada escreve no
  cache de terceiros e nenhum ponteiro alheio é apagado.

## Arquivos em disco

| Caminho | Papel |
|---|---|
| `~/.studio/models/<runtime>/*.gguf` | Modelos instalados (do usuário) |
| `~/.studio/models/<runtime>/.<arquivo>.part` | Temporário de transferência em curso |
| `~/.studio/models/<runtime>/<arquivo>.download.json` | Estado lateral do trabalho |
| `~/.studio/models/<runtime>/<arquivo>.download.json.cancel` | Marcador de cancelamento |
| `~/.studio/models/.models-helper/cache/` | Cache curto do catálogo |

## Variáveis de ambiente

| Variável | Padrão | Uso |
|---|---|---|
| `MODELS_HELPER_MODELS_ROOT` | `~/.studio/models` | Raiz do diretório de modelos |
| `MODELS_HELPER_HF_API` | `https://huggingface.co` | Base da API pública de modelos |
| `MODELS_HELPER_SERVER_URL` | `http://127.0.0.1:8081` | Base de sondagem do servidor local |
| `MODELS_HELPER_STUDIO_BIN` | `studio` | Binário da CLI usado na resolução de runtime |

## Desenvolvimento

Requer Go 1.24 ou superior, sem dependências externas:

```sh
go build ./...
go vet ./...
go test ./...
```

A esteira de compilação cruzada (`linux-x64-gnu`, `linux-arm64-gnu`,
`win32-x64`), release no GitHub e soma de verificação por artefato é entregue
em etapa própria, seguindo o formato de distribuição do `wa-control`.
