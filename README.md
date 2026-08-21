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
| `catalog list [--limit <n>] [--sort <ordem>] [--fit <filtro>]` | `TCatalogEntry[]` | Modelos por popularidade da API pública, com cache curto |
| `catalog search <termo> [--limit] [--sort] [--fit]` | `TCatalogEntry[]` | Busca por palavra-chave na mesma API |
| `catalog versions <repo> [--sort] [--fit]` | `TCatalogEntry[]` | Variantes com peso real por versão |
| `catalog curated [--limit] [--sort] [--fit]` | `TCatalogEntry[]` | Vitrine editorial: modelos escolhidos a dedo, com justificativa por item |
| `installed --dir <caminho> [--ext .gguf\|.bin] [--speech]` | `TInstalledModel[]` | Inventário local com veredito já calculado |
| `download start --repo <id> --file <arquivo> --dest <caminho>` | `TDownloadJob` | Retorna imediatamente com o identificador |
| `download status [--job <id>] [--dest <caminho>]` | `TDownloadJob[]` | Bytes, total e estado explícitos |
| `download cancel --job <id>` | `TDownloadJob` | Marca o cancelamento; o transferidor remove o temporário |
| `remove --path <caminho>` | `{ removed: boolean }` | Recusa caminho fora do diretório de modelos |
| `preset show\|set --model <caminho>\|clear\|ensure` | `TPreset` | Modelo padrão do servidor em modo roteador, exposto como `studio-local` |
| `status --profile local-models` | `TLocalModelsStatus` | Leitura composta, fonte única do painel de modelos |
| `status --profile local-transcription` | `TTranscriptionStatus` | Leitura composta, fonte única do painel de transcrição |

`--sort` aceita `fit` (padrão em `list`, `search` e `curated`), `popularity` e
`size` (padrão em `versions`). `--fit` aceita `any` (padrão), `fits` e `gpu`.
Um modelo já instalado nunca é escondido pelo filtro: ele ocupa disco, e sumir
com ele da lista tiraria do operador o lugar onde ele o remove.

O contrato de saída é normativo no SDS
(`engineering/runtime-multiprograma-modelos/sds-runtime-multiprograma-modelos`,
bloco "Modelo de Dados") e espelhado pelas structs Go `THelperEnvelope`,
`TMachineProfile`, `TModelFit`, `TCatalogEntry`, `TInstalledModel`,
`TDownloadJob`, `TLocalModelsStatus` e `TTranscriptionStatus` em
`internal/contract`. Campos novos entram como aditivos; nenhum campo existente
é renomeado. O teste de contrato afirma o conjunto v1 como piso e permite
crescimento — igualdade exata inverteria a regra, marcando como quebra
justamente o que o contrato autoriza.

## Regras de comportamento

- **Veredito pronto no dado**: `fitOk`, `fitTight`, `fitGpu`, `fitRank`,
  `fitLabel`, `fitTone` e `requiredBytes` chegam calculados em todo item de catálogo e de
  inventário; o consumidor não compara números nem encadeia condições. A
  fórmula de linguagem é peso × 1,2 + margem de KV-cache de 1,5 GiB contra RAM
  disponível + VRAM. A de fala (`--speech`) usa margem de 512 MiB e ignora
  memória de vídeo: um modelo de transcrição não guarda contexto de conversa, e
  o artefato de whisper.cpp declarado é o de processador.
- **Tudo que uma pessoa lê chega formatado**: cada valor bruto tem um rótulo ao
  lado (`ramTotalLabel`, `sizeLabel`, `progressLabel`, `fitLabel`). O painel do
  Construtor apenas interpola tokens — não tem operador de número — então
  formatar no consumidor não é uma opção disponível.
- **Ordem por viabilidade, estável**: a ordenação por veredito preserva a ordem
  de chegada dentro de cada faixa, que é a popularidade da API. Ordenar sem
  estabilidade descartaria o único sinal de qualidade que a lista carrega.
- **Vitrine editorial separada da busca**: `catalog curated` é uma lista escrita
  à mão, com justificativa por modelo; `catalog search` continua sendo
  popularidade pura, porque quem procura um modelo específico não quer opinião.
- **Catálogo de transcrição fixo**: dois repositórios oficiais e onze arquivos
  que não mudam, com peso de referência embutido — a lista continua utilizável
  com a API pública fora do ar. A recomendação padrão é medida, não inferida do
  tamanho.
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
| `~/.studio/models/llama-cpp/presets.ini` | Preset do modo roteador; declara `studio-local` apontando para o padrão |
| `~/.studio/models/whisper-cpp/.server.json` | Registro de qual modelo o servidor de transcrição carregou |
| `~/.studio/models/.models-helper/cache/` | Cache curto do catálogo |

## Variáveis de ambiente

| Variável | Padrão | Uso |
|---|---|---|
| `MODELS_HELPER_MODELS_ROOT` | `~/.studio/models` | Raiz do diretório de modelos |
| `MODELS_HELPER_HF_API` | `https://huggingface.co` | Base da API pública de modelos |
| `MODELS_HELPER_SERVER_URL` | `http://127.0.0.1:8081` | Base de sondagem do servidor de linguagem |
| `MODELS_HELPER_TRANSCRIPTION_SERVER_URL` | `http://127.0.0.1:8082` | Base de sondagem do servidor de transcrição |
| `MODELS_HELPER_STUDIO_BIN` | `studio` | Binário da CLI usado na resolução de runtime |

## Desenvolvimento

Requer Go 1.24 ou superior, sem dependências externas:

```sh
go build ./...
go vet ./...
go test ./...
```

## Distribuição e esteira de release

Ajudante distribuído como artefato de runtime pinado por sha256, seguindo o
formato de distribuição do `wa-control`: release do GitHub como canal, chaves
de plataforma canônicas `linux-x64-gnu`, `linux-arm64-gnu` e `win32-x64`.

| Comando | Papel |
|---|---|
| `bash scripts/build-release.sh all` | Compila `linux-x64`, `linux-arm64` e `windows-x64` em um único comando (cross-compile Go, `CGO_ENABLED=0`) |
| `bash scripts/release.sh` | Valida semver do `VERSION`, exige árvore limpa, compila as três plataformas, confere o limite de 512 MB por artefato, cria a tag e publica a release no GitHub |
| `bash scripts/manifest-fragment.sh` | Baixa cada artefato da release publicada, calcula sha256 e tamanho dos bytes baixados e emite `dist/manifest-fragment-v<versão>.json` com as chaves canônicas, pronto para colar no manifest das extensões |

Releases do GitHub não publicam soma de verificação: o sha256 e o tamanho são
calculados por artefato baixado, na preparação de cada manifesto — nunca do
arquivo local de build.

Layout dos artefatos: tar.gz com pasta de topo versionada no Linux
(`models-helper-v<versão>-<plataforma>/models-helper`, igual ao `wa-control`) e
zip plano no Windows (`models-helper.exe` na raiz, como os zips do llama). O
entry point declarado no manifest precisa refletir esse layout por artefato.

Estado de validação declarado por plataforma:

- `linux-x64-gnu`: compilado e validado em máquina real.
- `linux-arm64-gnu`: cross-compile, sem execução em máquina real.
- `win32-x64`: **declarado-não-validado** — cross-compile sem máquina Windows
  real; a extensão que declara o artefato registra o mesmo estado em vez de
  afirmar suporte validado.

A versão publicada é semver válida (arquivo `VERSION`) e precisa ser declarada
identicamente pelas duas extensões (`local-models` e `local-transcription`)
para manter a resolução de runtime sem ambiguidade.
