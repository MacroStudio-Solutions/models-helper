package contract

import (
	"encoding/json"
	"testing"
)

func keys(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	return m
}

// assertKeys confere que todo campo esperado continua presente, sem exigir que
// nada mais exista.
//
// A regra do contrato e aditiva: um consumidor que ja le o envelope nao pode
// perder um campo, mas ganhar um campo novo nunca quebra ninguem. Um teste de
// igualdade exata inverteria a regra — cada campo acrescentado apareceria como
// quebra, e o que ele realmente protege (remocao e renomeacao) ficaria
// indistinguivel de crescimento normal.
func assertKeys(t *testing.T, got map[string]any, want []string) {
	t.Helper()
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("campo %s ausente em %v", k, got)
		}
	}
}

func TestEnvelopeShape(t *testing.T) {
	ok := &THelperEnvelope[map[string]any]{SchemaVersion: 1, Ok: true}
	data := map[string]any{"x": 1}
	ok.Data = &data
	m := keys(t, ok)
	assertKeys(t, m, []string{"schemaVersion", "ok", "data"})
	if _, exists := m["error"]; exists {
		t.Fatalf("envelope de sucesso nao deve carregar error: %v", m)
	}

	fail := &THelperEnvelope[map[string]any]{SchemaVersion: 1, Ok: false, Error: &THelperError{Code: "X", Message: "y"}}
	m = keys(t, fail)
	assertKeys(t, m, []string{"schemaVersion", "ok", "error"})
	if _, exists := m["data"]; exists {
		t.Fatalf("envelope de falha nao deve carregar data: %v", m)
	}

	err := keys(t, fail.Error)
	assertKeys(t, err, []string{"code", "message"})
}

func TestMachineProfileKeepsV1Fields(t *testing.T) {
	assertKeys(t, keys(t, TMachineProfile{}), []string{
		"ramTotalBytes", "ramAvailableBytes", "cpuCores", "hasGpu", "gpuName",
		"vramBytes", "hasVulkan", "vulkanUnavailableReason",
	})
}

func TestMachineProfileCarriesLabels(t *testing.T) {
	assertKeys(t, keys(t, TMachineProfile{}), []string{
		"ramTotalLabel", "ramAvailableLabel", "vramLabel", "cpuLabel", "gpuLabel",
	})
}

func TestModelFitKeepsV1Fields(t *testing.T) {
	assertKeys(t, keys(t, TModelFit{}), []string{"fitOk", "fitTight", "fitGpu", "requiredBytes"})
}

func TestModelFitCarriesRankAndLabel(t *testing.T) {
	assertKeys(t, keys(t, TModelFit{}), []string{"fitRank", "fitLabel", "fitTone", "requiredLabel"})
}

func TestCatalogEntryKeepsV1Fields(t *testing.T) {
	entry := TCatalogEntry{Name: "n", RepoId: "o/r", File: "f.gguf", Quantization: "Q4_K_M", SizeBytes: 1, SizeGb: "0.0"}
	m := keys(t, entry)
	assertKeys(t, m, []string{
		"fitOk", "fitTight", "fitGpu", "requiredBytes", "name", "repoId",
		"file", "quantization", "sizeBytes", "sizeGb", "installed", "download",
	})
	if m["name"] != "n" || m["repoId"] != "o/r" || m["file"] != "f.gguf" || m["sizeGb"] != "0.0" {
		t.Fatalf("valores de v1 mudaram de forma: %v", m)
	}
	if m["download"] != nil {
		t.Fatalf("download sem trabalho deve marshalar null: %v", m["download"])
	}
}

func TestCatalogEntryCarriesEditorialFields(t *testing.T) {
	assertKeys(t, keys(t, TCatalogEntry{}), []string{"sizeLabel", "engine", "engineLabel", "summary", "recommended"})
}

func TestInstalledModelKeepsV1Fields(t *testing.T) {
	assertKeys(t, keys(t, TInstalledModel{}), []string{
		"fitOk", "fitTight", "fitGpu", "requiredBytes", "name", "path", "sizeBytes", "sizeGb",
	})
}

func TestInstalledModelCarriesServingFields(t *testing.T) {
	assertKeys(t, keys(t, TInstalledModel{}), []string{"sizeLabel", "apiName", "engine", "engineLabel", "isDefault", "isLoaded", "canServe"})
}

func TestDownloadJobKeepsV1Fields(t *testing.T) {
	assertKeys(t, keys(t, TDownloadJob{}), []string{
		"jobId", "repoId", "file", "destination", "state", "receivedBytes",
		"totalBytes", "percent", "pid", "startedAt", "updatedAt", "error",
	})
}

func TestDownloadJobCarriesLabels(t *testing.T) {
	assertKeys(t, keys(t, TDownloadJob{}), []string{"receivedLabel", "totalLabel", "progressLabel"})
}

func TestLocalModelsStatusShape(t *testing.T) {
	st := TLocalModelsStatus{Installed: []TInstalledModel{}, Catalog: []TCatalogEntry{}}
	m := keys(t, st)
	assertKeys(t, m, []string{
		"runtime", "server", "machine", "installed", "hasInstalled", "catalog", "catalogError",
	})
	runtimeM, ok := m["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("runtime nao e objeto: %v", m["runtime"])
	}
	assertKeys(t, runtimeM, []string{"ok", "error", "recommendedRuntimeId", "recommendationReason"})
	serverM, ok := m["server"].(map[string]any)
	if !ok {
		t.Fatalf("server nao e objeto: %v", m["server"])
	}
	assertKeys(t, serverM, []string{"online", "modelId", "baseUrl"})
	assertKeys(t, serverM, []string{"mode", "models", "modelCount", "loadedCount", "hasDefault", "defaultName"})
}

func TestTranscriptionStatusShape(t *testing.T) {
	st := TTranscriptionStatus{Installed: []TInstalledModel{}, Catalog: []TCatalogEntry{}}
	m := keys(t, st)
	assertKeys(t, m, []string{
		"runtime", "server", "machine", "installed", "hasInstalled", "catalog",
		"catalogError", "recommended", "recommendedFile", "hasRecommended", "hasServable",
	})
	serverM, ok := m["server"].(map[string]any)
	if !ok {
		t.Fatalf("server nao e objeto: %v", m["server"])
	}
	assertKeys(t, serverM, []string{"online", "baseUrl", "inferenceUrl", "modelName", "modelPath", "hasModelName"})
}

func TestDownloadJobPointerMarshalsNull(t *testing.T) {
	type wrapper struct {
		Download *TDownloadJob `json:"download"`
	}
	data, _ := json.Marshal(wrapper{})
	if string(data) != `{"download":null}` {
		t.Fatalf("esperado null, obtido %s", data)
	}
}
