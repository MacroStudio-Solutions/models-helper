package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/catalog"
	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/envelope"
	"github.com/MacroStudio-Solutions/models-helper/internal/inventory"
	"github.com/MacroStudio-Solutions/models-helper/internal/jobs"
	"github.com/MacroStudio-Solutions/models-helper/internal/machine"
	"github.com/MacroStudio-Solutions/models-helper/internal/paths"
	"github.com/MacroStudio-Solutions/models-helper/internal/preset"
	"github.com/MacroStudio-Solutions/models-helper/internal/statuscmd"
)

const defaultCatalogLimit = 6
const searchCatalogLimit = 20
const maxCatalogLimit = 50

const profileLocalModels = "local-models"
const profileLocalTranscription = "local-transcription"

func emit(data any, herr *contract.THelperError) {
	e := &contract.THelperEnvelope[any]{
		SchemaVersion: contract.SchemaVersion,
		Ok:            herr == nil,
	}
	if data != nil {
		wrapped := data
		e.Data = &wrapped
	}
	if herr != nil {
		e.Error = herr
	}
	if err := envelope.Print(os.Stdout, e); err != nil {
		fmt.Fprintln(os.Stderr, "models-helper: falha ao emitir o envelope:", err)
		os.Exit(1)
	}
	if herr != nil {
		os.Exit(1)
	}
}

func asHelperError(err error) *contract.THelperError {
	if err == nil {
		return nil
	}
	if he, ok := err.(*contract.THelperError); ok {
		return he
	}
	return contract.Errorf("INTERNAL_ERROR", "%v", err)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parse(fs *flag.FlagSet, args []string) *contract.THelperError {
	if err := fs.Parse(args); err != nil {
		return contract.Errorf("INVALID_USAGE", "uso invalido: %v", err)
	}
	return nil
}

func clampLimit(n int) int {
	if n < 1 {
		return 1
	}
	if n > maxCatalogLimit {
		return maxCatalogLimit
	}
	return n
}

func runMachine(args []string) int {
	fs := newFlagSet("machine")
	if herr := parse(fs, args); herr != nil {
		emit(nil, herr)
		return 1
	}
	emit(machine.Profile(), nil)
	return 0
}

// tCatalogView sao as tres decisoes que o consumidor toma sobre uma lista de
// catalogo: quantos itens, em que ordem e o que esconder. Ficam juntas porque
// os quatro subcomandos de catalogo aceitam exatamente as mesmas.
type tCatalogView struct {
	limit int
	sort  string
	fit   string
}

func catalogViewFlags(fs *flag.FlagSet, defaultLimit int, defaultSort string) *tCatalogView {
	view := &tCatalogView{}
	fs.IntVar(&view.limit, "limit", defaultLimit, "quantidade maxima de modelos")
	fs.StringVar(&view.sort, "sort", defaultSort, "ordem: fit, popularity ou size")
	fs.StringVar(&view.fit, "fit", catalog.FitAny, "filtro de viabilidade: any, fits ou gpu")
	return view
}

func (v *tCatalogView) validate() *contract.THelperError {
	if !catalog.IsSortMode(v.sort) {
		return contract.Errorf("INVALID_USAGE", "ordem desconhecida: %s (use fit, popularity ou size)", v.sort)
	}
	if !catalog.IsFitMode(v.fit) {
		return contract.Errorf("INVALID_USAGE", "filtro de viabilidade desconhecido: %s (use any, fits ou gpu)", v.fit)
	}
	return nil
}

// apply e o unico caminho pelo qual uma lista de catalogo chega ao envelope:
// anexa o estado de download, filtra, ordena e corta, nessa ordem. Filtrar
// antes de ordenar mantem o corte previsivel — cortar primeiro descartaria
// justamente o que a ordenacao traria para o topo.
func (v *tCatalogView) apply(entries []contract.TCatalogEntry, modelsDir string) []contract.TCatalogEntry {
	statuscmd.AttachDownloads(entries, modelsDir)
	entries = catalog.FilterByFit(entries, v.fit)
	catalog.SortEntries(entries, v.sort)
	return catalog.Limit(entries, clampLimit(v.limit))
}

func runCatalog(args []string) int {
	if len(args) == 0 {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: catalog <list|search|versions|curated> [opcoes]"))
		return 1
	}

	sub := args[0]
	rest := args[1:]

	fs := newFlagSet("catalog " + sub)
	defaultLimit := defaultCatalogLimit
	defaultSort := catalog.SortFit
	switch sub {
	case "search":
		defaultLimit = searchCatalogLimit
	case "versions":
		defaultLimit = maxCatalogLimit
		defaultSort = catalog.SortSize
	case "curated":
		defaultLimit = len(catalog.CuratedRepos())
	case "list":
	default:
		emit(nil, contract.Errorf("INVALID_USAGE", "subcomando de catalog desconhecido: %s", sub))
		return 1
	}
	view := catalogViewFlags(fs, defaultLimit, defaultSort)
	if herr := parse(fs, rest); herr != nil {
		emit(nil, herr)
		return 1
	}
	if herr := view.validate(); herr != nil {
		emit(nil, herr)
		return 1
	}

	wantsTerm := sub == "search" || sub == "versions"
	if wantsTerm && fs.NArg() != 1 {
		usage := "uso: catalog search <termo> [opcoes]"
		if sub == "versions" {
			usage = "uso: catalog versions <repo> [opcoes]"
		}
		emit(nil, contract.Errorf("INVALID_USAGE", "%s", usage))
		return 1
	}
	if !wantsTerm && fs.NArg() > 0 {
		emit(nil, contract.Errorf("INVALID_USAGE", "catalog %s nao aceita argumentos posicionais", sub))
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := catalog.NewClient()
	profile := machine.Profile()
	modelsDir := paths.LlamaModelsDir()

	var entries []contract.TCatalogEntry
	switch sub {
	case "list":
		// A busca por popularidade pesca fundo de proposito: filtrar por
		// viabilidade um punhado de resultados devolveria uma lista quase
		// vazia justamente na maquina que mais precisa do filtro.
		models, err := client.List(ctx, clampLimit(view.limit*4))
		if err != nil {
			emit(nil, asHelperError(err))
			return 1
		}
		entries = catalog.DefaultEntries(ctx, client, models, profile, modelsDir)
	case "search":
		models, err := client.Search(ctx, fs.Arg(0), clampLimit(view.limit*2))
		if err != nil {
			emit(nil, asHelperError(err))
			return 1
		}
		entries = catalog.DefaultEntries(ctx, client, models, profile, modelsDir)
	case "versions":
		found, herr := catalog.VersionEntries(ctx, client, fs.Arg(0), profile, modelsDir)
		if herr != nil {
			emit(nil, herr)
			return 1
		}
		entries = found
	case "curated":
		entries = catalog.CuratedEntries(ctx, client, profile, modelsDir)
	}

	emit(view.apply(entries, modelsDir), nil)
	return 0
}

func runInstalled(args []string) int {
	fs := newFlagSet("installed")
	dir := fs.String("dir", "", "diretorio a inventariar")
	ext := fs.String("ext", inventory.ExtGguf, "extensao dos pesos: .gguf ou .bin")
	speech := fs.Bool("speech", false, "avaliar viabilidade pela formula de transcricao")
	if herr := parse(fs, args); herr != nil {
		emit(nil, herr)
		return 1
	}
	if *dir == "" {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: installed --dir <caminho> [--ext .gguf|.bin] [--speech]"))
		return 1
	}
	if *ext != inventory.ExtGguf && *ext != inventory.ExtWhisper {
		emit(nil, contract.Errorf("INVALID_USAGE", "extensao nao suportada: %s (use .gguf ou .bin)", *ext))
		return 1
	}
	profile := machine.Profile()
	models, err := inventory.ListWith(*dir, profile, inventory.TOptions{Ext: *ext, Speech: *speech})
	if err != nil {
		emit(nil, contract.Errorf("INVENTORY_FAILED", "falha ao inventariar %s: %v", *dir, err))
		return 1
	}
	emit(models, nil)
	return 0
}

func runDownload(args []string) int {
	if len(args) == 0 {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: download <start|status|cancel> [opcoes]"))
		return 1
	}
	switch args[0] {
	case "start":
		fs := newFlagSet("download start")
		repo := fs.String("repo", "", "identificador do repositorio (org/nome)")
		file := fs.String("file", "", "arquivo a baixar")
		dest := fs.String("dest", "", "diretorio de destino")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if *repo == "" || *file == "" || *dest == "" {
			emit(nil, contract.Errorf("INVALID_USAGE", "uso: download start --repo <id> --file <arquivo> --dest <caminho>"))
			return 1
		}
		job, herr := jobs.Start(*repo, *file, *dest)
		emit(job, herr)
		if herr != nil {
			return 1
		}
		return 0
	case "status":
		fs := newFlagSet("download status")
		jobId := fs.String("job", "", "identificador do trabalho")
		dest := fs.String("dest", "", "diretorio dos trabalhos")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		list, herr := jobs.Status(*jobId, *dest)
		emit(list, herr)
		if herr != nil {
			return 1
		}
		return 0
	case "cancel":
		fs := newFlagSet("download cancel")
		jobId := fs.String("job", "", "identificador do trabalho")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if *jobId == "" {
			emit(nil, contract.Errorf("INVALID_USAGE", "uso: download cancel --job <id>"))
			return 1
		}
		job, herr := jobs.Cancel(*jobId)
		emit(job, herr)
		if herr != nil {
			return 1
		}
		return 0
	default:
		emit(nil, contract.Errorf("INVALID_USAGE", "subcomando de download desconhecido: %s", args[0]))
		return 1
	}
}

func runRemove(args []string) int {
	fs := newFlagSet("remove")
	path := fs.String("path", "", "caminho do arquivo a remover")
	if herr := parse(fs, args); herr != nil {
		emit(nil, herr)
		return 1
	}
	if *path == "" {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: remove --path <caminho>"))
		return 1
	}
	removed, herr := jobs.Remove(*path)
	data := map[string]bool{"removed": removed}
	emit(data, herr)
	if herr != nil {
		return 1
	}
	return 0
}

func runPreset(args []string) int {
	if len(args) == 0 {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: preset <show|set|clear|ensure> [opcoes]"))
		return 1
	}
	switch args[0] {
	case "show":
		fs := newFlagSet("preset show")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		emit(preset.Read(), nil)
		return 0
	case "ensure":
		fs := newFlagSet("preset ensure")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if _, herr := preset.Ensure(); herr != nil {
			emit(nil, herr)
			return 1
		}
		emit(preset.Read(), nil)
		return 0
	case "set":
		fs := newFlagSet("preset set")
		model := fs.String("model", "", "caminho do modelo padrao")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if *model == "" {
			emit(nil, contract.Errorf("INVALID_USAGE", "uso: preset set --model <caminho>"))
			return 1
		}
		state, herr := preset.Set(*model)
		emit(state, herr)
		if herr != nil {
			return 1
		}
		return 0
	case "clear":
		fs := newFlagSet("preset clear")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		state, herr := preset.Clear()
		emit(state, herr)
		if herr != nil {
			return 1
		}
		return 0
	default:
		emit(nil, contract.Errorf("INVALID_USAGE", "subcomando de preset desconhecido: %s", args[0]))
		return 1
	}
}

func runStatus(args []string) int {
	fs := newFlagSet("status")
	profile := fs.String("profile", "", "perfil de leitura composta")
	if herr := parse(fs, args); herr != nil {
		emit(nil, herr)
		return 1
	}
	switch *profile {
	case "":
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: status --profile <nome>"))
		return 1
	case profileLocalModels:
		emit(statuscmd.Build(), nil)
		return 0
	case profileLocalTranscription:
		emit(statuscmd.BuildTranscription(), nil)
		return 0
	default:
		emit(nil, contract.Errorf("UNSUPPORTED_PROFILE", "perfil nao suportado: %s", *profile))
		return 1
	}
}

func runTransfer(args []string) int {
	fs := newFlagSet("__transfer")
	jobFile := fs.String("job-file", "", "arquivo lateral do trabalho")
	if herr := parse(fs, args); herr != nil {
		fmt.Fprintln(os.Stderr, "models-helper:", herr.Message)
		return 1
	}
	if *jobFile == "" {
		fmt.Fprintln(os.Stderr, "models-helper: __transfer exige --job-file")
		return 1
	}
	if err := jobs.RunTransfer(*jobFile); err != nil {
		fmt.Fprintln(os.Stderr, "models-helper:", err.Error())
		return 1
	}
	return 0
}

func run(args []string) int {
	if len(args) == 0 {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: models-helper <machine|catalog|installed|download|remove|preset|status> [opcoes]"))
		return 1
	}
	switch args[0] {
	case "machine":
		return runMachine(args[1:])
	case "catalog":
		return runCatalog(args[1:])
	case "installed":
		return runInstalled(args[1:])
	case "download":
		return runDownload(args[1:])
	case "remove":
		return runRemove(args[1:])
	case "preset":
		return runPreset(args[1:])
	case "status":
		return runStatus(args[1:])
	case "__transfer":
		return runTransfer(args[1:])
	default:
		emit(nil, contract.Errorf("INVALID_USAGE", "comando desconhecido: %s", args[0]))
		return 1
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}
