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
	"github.com/MacroStudio-Solutions/models-helper/internal/statuscmd"
)

const defaultCatalogLimit = 6
const searchCatalogLimit = 20
const maxCatalogLimit = 50

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

func runCatalog(args []string) int {
	if len(args) == 0 {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: catalog <list|search|versions> [opcoes]"))
		return 1
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("catalog list")
		limit := fs.Int("limit", defaultCatalogLimit, "quantidade maxima de modelos")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if fs.NArg() > 0 {
			emit(nil, contract.Errorf("INVALID_USAGE", "catalog list nao aceita argumentos posicionais"))
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client := catalog.NewClient()
		profile := machine.Profile()
		modelsDir := paths.LlamaModelsDir()
		models, err := client.List(ctx, clampLimit(*limit))
		if err != nil {
			emit(nil, asHelperError(err))
			return 1
		}
		entries := catalog.DefaultEntries(ctx, client, models, profile, modelsDir)
		collected := jobs.Collect(paths.ModelsRoot())
		for _, c := range collected {
			jobs.Reap(c)
			jobs.RefreshReceived(c)
		}
		byFile := jobs.LatestByFile(collected, modelsDir)
		for i := range entries {
			attachDownload(&entries[i], byFile)
		}
		emit(entries, nil)
		return 0
	case "search":
		fs := newFlagSet("catalog search")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if fs.NArg() != 1 {
			emit(nil, contract.Errorf("INVALID_USAGE", "uso: catalog search <termo>"))
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client := catalog.NewClient()
		profile := machine.Profile()
		modelsDir := paths.LlamaModelsDir()
		models, err := client.Search(ctx, fs.Arg(0), searchCatalogLimit)
		if err != nil {
			emit(nil, asHelperError(err))
			return 1
		}
		entries := catalog.DefaultEntries(ctx, client, models, profile, modelsDir)
		collected := jobs.Collect(paths.ModelsRoot())
		for _, c := range collected {
			jobs.Reap(c)
			jobs.RefreshReceived(c)
		}
		byFile := jobs.LatestByFile(collected, modelsDir)
		for i := range entries {
			attachDownload(&entries[i], byFile)
		}
		emit(entries, nil)
		return 0
	case "versions":
		fs := newFlagSet("catalog versions")
		if herr := parse(fs, args[1:]); herr != nil {
			emit(nil, herr)
			return 1
		}
		if fs.NArg() != 1 {
			emit(nil, contract.Errorf("INVALID_USAGE", "uso: catalog versions <repo>"))
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client := catalog.NewClient()
		profile := machine.Profile()
		modelsDir := paths.LlamaModelsDir()
		entries, herr := catalog.VersionEntries(ctx, client, fs.Arg(0), profile, modelsDir)
		if herr != nil {
			emit(nil, herr)
			return 1
		}
		collected := jobs.Collect(paths.ModelsRoot())
		for _, c := range collected {
			jobs.Reap(c)
			jobs.RefreshReceived(c)
		}
		byFile := jobs.LatestByFile(collected, modelsDir)
		for i := range entries {
			attachDownload(&entries[i], byFile)
		}
		emit(entries, nil)
		return 0
	default:
		emit(nil, contract.Errorf("INVALID_USAGE", "subcomando de catalog desconhecido: %s", args[0]))
		return 1
	}
}

func attachDownload(entry *contract.TCatalogEntry, byFile map[string]*jobs.TJobFile) {
	j := byFile[entry.File]
	if j == nil {
		entry.Download = nil
		return
	}
	if j.State == contract.JobStateCompleted && entry.Installed {
		entry.Download = nil
		return
	}
	job := j.TDownloadJob
	entry.Download = &job
}

func runInstalled(args []string) int {
	fs := newFlagSet("installed")
	dir := fs.String("dir", "", "diretorio a inventariar")
	if herr := parse(fs, args); herr != nil {
		emit(nil, herr)
		return 1
	}
	if *dir == "" {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: installed --dir <caminho>"))
		return 1
	}
	profile := machine.Profile()
	models, err := inventory.List(*dir, profile)
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

func runStatus(args []string) int {
	fs := newFlagSet("status")
	profile := fs.String("profile", "", "perfil de leitura composta")
	if herr := parse(fs, args); herr != nil {
		emit(nil, herr)
		return 1
	}
	if *profile == "" {
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: status --profile <nome>"))
		return 1
	}
	if *profile != "local-models" {
		emit(nil, contract.Errorf("UNSUPPORTED_PROFILE", "perfil nao suportado: %s", *profile))
		return 1
	}
	emit(statuscmd.Build(), nil)
	return 0
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
		emit(nil, contract.Errorf("INVALID_USAGE", "uso: models-helper <machine|catalog|installed|download|remove|status> [opcoes]"))
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
