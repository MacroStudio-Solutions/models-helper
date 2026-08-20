package statuscmd

import (
	"context"
	"encoding/json"
	"net/http"
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
	"github.com/MacroStudio-Solutions/models-helper/internal/rtresolve"
)

func probeServer() (bool, string) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(env.ServerBaseUrl() + "/v1/models")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, ""
	}
	var payload struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return true, ""
	}
	if len(payload.Data) > 0 {
		return true, payload.Data[0].Id
	}
	return true, ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func Build() contract.TLocalModelsStatus {
	machineProfile := machine.Profile()

	var wg sync.WaitGroup
	var runtimeOk bool
	var runtimeErr string
	var serverOnline bool
	var serverModel string
	var catalogEntries []contract.TCatalogEntry
	var catalogErr string

	wg.Add(3)
	go func() {
		defer wg.Done()
		runtimeOk, runtimeErr = rtresolve.ResolveLlamaRuntime(8 * time.Second)
	}()
	go func() {
		defer wg.Done()
		serverOnline, serverModel = probeServer()
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client := catalog.NewClient()
		models, err := client.List(ctx, 6)
		if err != nil {
			catalogErr = err.(*contract.THelperError).Message
			return
		}
		catalogEntries = catalog.DefaultEntries(ctx, client, models, machineProfile, paths.LlamaModelsDir())
	}()
	wg.Wait()

	recommendedId, recommendedReason := machine.RecommendVariant(machineProfile)

	modelsDir := paths.LlamaModelsDir()
	_ = os.MkdirAll(modelsDir, 0755)

	installed, err := inventory.List(modelsDir, machineProfile)
	if err != nil {
		installed = []contract.TInstalledModel{}
	}
	if catalogEntries == nil {
		catalogEntries = []contract.TCatalogEntry{}
	}
	for i := range catalogEntries {
		catalogEntries[i].Installed = fileExists(filepath.Join(modelsDir, catalogEntries[i].File))
	}

	collected := jobs.Collect(paths.ModelsRoot())
	for _, c := range collected {
		jobs.Reap(c)
		jobs.RefreshReceived(c)
	}
	byFile := jobs.LatestByFile(collected, modelsDir)

	for i := range catalogEntries {
		entry := &catalogEntries[i]
		var dl *contract.TDownloadJob
		if j := byFile[entry.File]; j != nil {
			if j.State == contract.JobStateCompleted && entry.Installed {
				dl = nil
			} else {
				job := j.TDownloadJob
				dl = &job
			}
		}
		entry.Download = dl
	}

	return contract.TLocalModelsStatus{
		Runtime: contract.TRuntimeHealth{
			Ok:                   runtimeOk,
			Error:                runtimeErr,
			RecommendedRuntimeId: recommendedId,
			RecommendationReason: recommendedReason,
		},
		Server: contract.TServerState{
			Online:  serverOnline,
			ModelId: serverModel,
			BaseUrl: env.ServerBaseUrl() + "/v1",
		},
		Machine:      machineProfile,
		Installed:    installed,
		HasInstalled: len(installed) > 0,
		Catalog:      catalogEntries,
		CatalogError: catalogErr,
	}
}
