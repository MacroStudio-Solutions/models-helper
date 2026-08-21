package statuscmd

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/catalog"
	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/env"
	"github.com/MacroStudio-Solutions/models-helper/internal/inventory"
	"github.com/MacroStudio-Solutions/models-helper/internal/jobs"
	"github.com/MacroStudio-Solutions/models-helper/internal/machine"
	"github.com/MacroStudio-Solutions/models-helper/internal/paths"
	"github.com/MacroStudio-Solutions/models-helper/internal/preset"
	"github.com/MacroStudio-Solutions/models-helper/internal/rtresolve"
	"github.com/MacroStudio-Solutions/models-helper/internal/server"
)

// Tamanho da vitrine na tela. Curta de proposito: uma lista longa de modelos
// que a maquina aguenta ainda e uma lista longa.
const showcaseSize = 6

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// attachDownloads casa cada entrada de catalogo com o trabalho de download que
// exista para ela. Um trabalho concluido cujo arquivo ja esta no disco some:
// o estado dele agora e "instalado", nao "baixado com sucesso".
func AttachDownloads(entries []contract.TCatalogEntry, modelsDir string) {
	collected := jobs.Collect(paths.ModelsRoot())
	for _, c := range collected {
		jobs.Reap(c)
		jobs.RefreshReceived(c)
	}
	byFile := jobs.LatestByFile(collected, modelsDir)
	for i := range entries {
		entry := &entries[i]
		entry.Installed = fileExists(filepath.Join(modelsDir, entry.File))
		j := byFile[entry.File]
		if j == nil || (j.State == contract.JobStateCompleted && entry.Installed) {
			entry.Download = nil
			continue
		}
		job := jobs.Snapshot(j)
		entry.Download = &job
	}
}

func Build() contract.TLocalModelsStatus {
	machineProfile := machine.Profile()

	var wg sync.WaitGroup
	var runtimeOk bool
	var runtimeErr string
	var serverState contract.TServerState
	var catalogEntries []contract.TCatalogEntry
	var catalogErr string

	wg.Add(3)
	go func() {
		defer wg.Done()
		runtimeOk, runtimeErr = rtresolve.ResolveLlamaRuntime(8 * time.Second)
	}()
	go func() {
		defer wg.Done()
		serverState = server.ProbeLlama(env.ServerBaseUrl())
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		client := catalog.NewClient()
		// A vitrine e a lista editorial, nao a lista por popularidade da API:
		// numa maquina comum a maioria do que "mais baixado" devolve nao roda,
		// e um catalogo de recomendacao que recomenda o inviavel nao recomenda
		// nada. A busca aberta continua atendendo quem procura algo especifico.
		catalogEntries = catalog.CuratedEntries(ctx, client, machineProfile, paths.LlamaModelsDir())
		if len(catalogEntries) == 0 {
			models, err := client.List(ctx, 12)
			if err != nil {
				catalogErr = err.(*contract.THelperError).Message
				return
			}
			catalogEntries = catalog.DefaultEntries(ctx, client, models, machineProfile, paths.LlamaModelsDir())
		}
	}()
	wg.Wait()

	recommendedId, recommendedReason := machine.RecommendVariant(machineProfile)

	modelsDir := paths.LlamaModelsDir()
	_ = os.MkdirAll(modelsDir, 0755)

	installed, err := inventory.ListWith(modelsDir, machineProfile, inventory.TOptions{
		Ext:    inventory.ExtGguf,
		Engine: "llama",
	})
	if err != nil {
		installed = []contract.TInstalledModel{}
	}

	defaultPreset := preset.Read()
	loaded := map[string]bool{}
	for _, m := range serverState.Models {
		loaded[m.Id] = m.Loaded
	}
	for i := range installed {
		installed[i].IsDefault = defaultPreset.HasModel && defaultPreset.ModelPath == installed[i].Path
		installed[i].IsLoaded = loaded[installed[i].ApiName]
	}

	if catalogEntries == nil {
		catalogEntries = []contract.TCatalogEntry{}
	}
	AttachDownloads(catalogEntries, modelsDir)
	catalogEntries = catalog.FilterByFit(catalogEntries, catalog.FitFits)
	catalog.SortEntries(catalogEntries, catalog.SortFit)
	catalogEntries = catalog.Limit(catalogEntries, showcaseSize)

	return contract.TLocalModelsStatus{
		Runtime: contract.TRuntimeHealth{
			Ok:                   runtimeOk,
			Error:                runtimeErr,
			RecommendedRuntimeId: recommendedId,
			RecommendationReason: recommendedReason,
		},
		Server:       serverState,
		Machine:      machineProfile,
		Installed:    installed,
		HasInstalled: len(installed) > 0,
		Catalog:      catalogEntries,
		CatalogError: catalogErr,
	}
}

// BuildTranscription e a leitura composta da tela de transcricao. Mesma forma
// da tela de modelos, com duas diferencas de fundo: o catalogo e fixo, e a
// viabilidade usa a formula de fala.
func BuildTranscription() contract.TTranscriptionStatus {
	machineProfile := machine.Profile()

	var wg sync.WaitGroup
	var runtimeOk bool
	var runtimeErr string
	var serverState contract.TTranscriptionServer
	var catalogEntries []contract.TCatalogEntry
	var catalogErr string

	modelsDir := paths.WhisperModelsDir()
	_ = os.MkdirAll(modelsDir, 0755)

	wg.Add(3)
	go func() {
		defer wg.Done()
		runtimeOk, runtimeErr = rtresolve.ResolveRuntime("whisper-cpp", 8*time.Second)
	}()
	go func() {
		defer wg.Done()
		serverState = server.ProbeWhisper(env.TranscriptionServerBaseUrl(), modelsDir)
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		catalogEntries, catalogErr = catalog.SpeechEntries(ctx, catalog.NewClient(), machineProfile, modelsDir)
	}()
	wg.Wait()

	installed, err := inventory.ListWith(modelsDir, machineProfile, inventory.TOptions{
		Ext:    inventory.ExtWhisper,
		Speech: true,
	})
	if err != nil {
		installed = []contract.TInstalledModel{}
	}
	for i := range installed {
		installed[i].Engine = engineOf(installed[i].Name)
		installed[i].IsLoaded = serverState.Online && serverState.ModelName == installed[i].Name
	}

	AttachDownloads(catalogEntries, modelsDir)

	recommended := ""
	hasRecommended := false
	for _, e := range catalogEntries {
		if e.Recommended {
			recommended = e.Name
			hasRecommended = e.Installed
		}
	}

	return contract.TTranscriptionStatus{
		Runtime: contract.TRuntimeHealth{
			Ok:                   runtimeOk,
			Error:                runtimeErr,
			RecommendedRuntimeId: "whisper-cpp",
			RecommendationReason: "artefato de processador; o whisper.cpp declarado por esta extensão não traz variante de GPU",
		},
		Server:          serverState,
		Machine:         machineProfile,
		Installed:       installed,
		HasInstalled:    len(installed) > 0,
		Catalog:         catalogEntries,
		CatalogError:    catalogErr,
		Recommended:     recommended,
		RecommendedFile: catalog.RecommendedSpeechFile,
		HasRecommended:  hasRecommended,
	}
}

// engineOf decide qual programa transcreve um peso instalado. O parakeet tem
// binario proprio e nao carrega no whisper-cli, entao errar aqui e entregar ao
// operador um comando que nao roda.
func engineOf(name string) string {
	if len(name) >= 15 && name[:15] == "ggml-parakeet-t" {
		return catalog.EngineParakeet
	}
	return catalog.EngineWhisper
}
